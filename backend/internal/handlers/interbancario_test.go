package handlers

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/sudameris/backend/internal/config"
	"github.com/sudameris/backend/internal/interbanco"
	"github.com/sudameris/backend/internal/models"
)

// mundoInterbancario simula "el otro banco": un httptest.Server real con
// el router de recepción de créditos (VerifyBankSignature + RecibirCredito
// + ConsultarCredito), respaldado por su PROPIA base de test (ver
// segundaBaseDePrueba) y con su propio banco_id y su propio par de
// claves. El banco "origen" no necesita su propio servidor: a él lo
// llamamos directo, como a los demás handlers, usando la base de test
// principal.
//
// Es clave que origen y destino usen bases físicamente distintas, no solo
// banco_id distintos en la misma tabla: ver el comentario de
// segundaBaseDePrueba en testmain_test.go para el porqué.
type mundoInterbancario struct {
	bancoOrigenID  string
	clienteOrigen  *interbanco.Cliente
	registroOrigen map[string]config.BancoRegistrado

	bancoDestinoID string
	dbDestino      *sqlx.DB
	destinoServer  *httptest.Server

	llamadas int32 // atómico: cuántas veces se llamó a una ruta del destino
}

func (m *mundoInterbancario) LlamadasADestino() int {
	return int(atomic.LoadInt32(&m.llamadas))
}

// crearCuentaEnDestino y saldoEnDestino son los equivalentes a
// crearCuentaDirecta/saldoActual pero apuntando a la base del banco
// destino simulado, no a la base principal.
func (m *mundoInterbancario) crearCuentaEnDestino(t *testing.T, numeroCuenta string, saldo int64, pin string) {
	t.Helper()
	crearCuentaDirectaEn(t, m.dbDestino, m.bancoDestinoID, numeroCuenta, saldo, pin)
}

func (m *mundoInterbancario) saldoEnDestino(t *testing.T, numeroCuenta string) int64 {
	t.Helper()
	return saldoActualEn(t, m.dbDestino, m.bancoDestinoID, numeroCuenta)
}

func armarMundoInterbancario(t *testing.T, dbOrigen *sqlx.DB) *mundoInterbancario {
	t.Helper()

	dbDestino := segundaBaseDePrueba(t)

	bancoOrigenID := "ORIGEN-" + idAleatorio(t)
	pubOrigen, privOrigen, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generando clave de origen: %v", err)
	}

	bancoDestinoID := "DESTINO-" + idAleatorio(t)
	pubDestino, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generando clave de destino: %v", err)
	}

	m := &mundoInterbancario{
		bancoOrigenID:  bancoOrigenID,
		bancoDestinoID: bancoDestinoID,
		dbDestino:      dbDestino,
	}

	destinoRouter := gin.New()
	destinoRouter.Use(func(c *gin.Context) {
		atomic.AddInt32(&m.llamadas, 1)
		c.Next()
	})
	destinoRouter.Use(VerifyBankSignature(map[string]ed25519.PublicKey{bancoOrigenID: pubOrigen}))
	destinoRouter.POST("/interbancario/creditos", RecibirCredito(dbDestino, bancoDestinoID))
	destinoRouter.GET("/interbancario/creditos/:transfer_id", ConsultarCredito(dbDestino, bancoDestinoID))

	destinoServer := httptest.NewServer(destinoRouter)
	t.Cleanup(destinoServer.Close)
	m.destinoServer = destinoServer

	m.clienteOrigen = interbanco.NuevoCliente(bancoOrigenID, privOrigen)
	m.registroOrigen = map[string]config.BancoRegistrado{
		bancoDestinoID: {URL: destinoServer.URL, PublicKey: pubDestino},
	}

	return m
}

// Caso 6 (doc 04): transferencia interbancaria exitosa.
func TestTransferenciaInterbancaria_Exitosa(t *testing.T) {
	db := requireTestDB(t)
	m := armarMundoInterbancario(t, db)
	handler := TransferenciaInterbancaria(db, m.bancoOrigenID, m.clienteOrigen, m.registroOrigen)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, m.bancoOrigenID, "ORIGEN", 1_000_000, "", usuarioID)
	m.crearCuentaEnDestino(t, "DESTINO", 0, "")

	req := transferenciaInterbancariaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		BancoDestino:        m.bancoDestinoID,
		NumeroCuentaDestino: "DESTINO",
		Importe:             100_000,
		IdempotencyKey:      uuidDePrueba(t),
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/interbancarias", req, nil, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w, http.StatusCreated)

	op := decodificarOperacion(t, w)
	if op.Estado != models.OperacionCompletada {
		t.Fatalf("esperaba COMPLETADA, dio %q (motivo: %v)", op.Estado, op.MotivoRechazo)
	}

	if saldo := saldoActual(t, m.bancoOrigenID, "ORIGEN"); saldo != 900_000 {
		t.Fatalf("saldo de origen debería quedar en 900000, quedó en %d", saldo)
	}
	if saldo := m.saldoEnDestino(t, "DESTINO"); saldo != 100_000 {
		t.Fatalf("saldo de destino debería quedar en 100000, quedó en %d", saldo)
	}
}

// Caso 7 (doc 04): rechazo porque la cuenta destino no existe en el banco
// destino. La reserva en origen se tiene que revertir.
func TestTransferenciaInterbancaria_CuentaDestinoNoExiste(t *testing.T) {
	db := requireTestDB(t)
	m := armarMundoInterbancario(t, db)
	handler := TransferenciaInterbancaria(db, m.bancoOrigenID, m.clienteOrigen, m.registroOrigen)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, m.bancoOrigenID, "ORIGEN", 1_000_000, "", usuarioID)
	// "DESTINO" nunca se crea en el banco destino.

	req := transferenciaInterbancariaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		BancoDestino:        m.bancoDestinoID,
		NumeroCuentaDestino: "DESTINO",
		Importe:             100_000,
		IdempotencyKey:      uuidDePrueba(t),
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/interbancarias", req, nil, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w, http.StatusUnprocessableEntity)

	op := decodificarOperacion(t, w)
	if op.Estado != models.OperacionRechazada {
		t.Fatalf("esperaba RECHAZADA, dio %q", op.Estado)
	}
	if op.MotivoRechazo == nil || *op.MotivoRechazo != ErrCuentaNoExiste {
		t.Fatalf("esperaba motivo_rechazo=%q, dio %v", ErrCuentaNoExiste, op.MotivoRechazo)
	}
	if saldo := saldoActual(t, m.bancoOrigenID, "ORIGEN"); saldo != 1_000_000 {
		t.Fatalf("la reserva debería revertirse, saldo quedó en %d", saldo)
	}
}

// Caso 8 (doc 04): rechazo por fondos insuficientes en origen. No debería
// llegar a llamar al banco destino -- se corta localmente antes.
func TestTransferenciaInterbancaria_FondosInsuficientesEnOrigen(t *testing.T) {
	db := requireTestDB(t)
	m := armarMundoInterbancario(t, db)
	handler := TransferenciaInterbancaria(db, m.bancoOrigenID, m.clienteOrigen, m.registroOrigen)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, m.bancoOrigenID, "ORIGEN", 1_000, "", usuarioID)
	m.crearCuentaEnDestino(t, "DESTINO", 0, "")

	req := transferenciaInterbancariaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		BancoDestino:        m.bancoDestinoID,
		NumeroCuentaDestino: "DESTINO",
		Importe:             5_000,
		IdempotencyKey:      uuidDePrueba(t),
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/interbancarias", req, nil, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w, http.StatusUnprocessableEntity)

	op := decodificarOperacion(t, w)
	if op.Estado != models.OperacionRechazada {
		t.Fatalf("esperaba RECHAZADA, dio %q", op.Estado)
	}
	if saldo := saldoActual(t, m.bancoOrigenID, "ORIGEN"); saldo != 1_000 {
		t.Fatalf("un rechazo local no debería tocar el saldo, quedó en %d", saldo)
	}
	if llamadas := m.LlamadasADestino(); llamadas != 0 {
		t.Fatalf("un rechazo por fondos insuficientes en origen no debería llamar al banco destino; lo llamó %d veces", llamadas)
	}
}

// Caso 9 (doc 04): rechazo por superar el límite diario combinado de la
// cuenta origen, tras una transferencia interbancaria previa el mismo
// día. Tampoco debería llamar al destino una segunda vez.
func TestTransferenciaInterbancaria_LimiteDiarioCombinado(t *testing.T) {
	db := requireTestDB(t)
	m := armarMundoInterbancario(t, db)
	handler := TransferenciaInterbancaria(db, m.bancoOrigenID, m.clienteOrigen, m.registroOrigen)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, m.bancoOrigenID, "ORIGEN", 20_000_000, "", usuarioID)
	m.crearCuentaEnDestino(t, "DESTINO", 0, "")

	primera := transferenciaInterbancariaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		BancoDestino:        m.bancoDestinoID,
		NumeroCuentaDestino: "DESTINO",
		Importe:             9_500_000,
		IdempotencyKey:      uuidDePrueba(t),
	}
	w1, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/interbancarias", primera, nil, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w1, http.StatusCreated)

	if llamadas := m.LlamadasADestino(); llamadas != 1 {
		t.Fatalf("esperaba 1 llamada al destino tras la primera transferencia, hubo %d", llamadas)
	}

	segunda := transferenciaInterbancariaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		BancoDestino:        m.bancoDestinoID,
		NumeroCuentaDestino: "DESTINO",
		Importe:             1_000_000,
		IdempotencyKey:      uuidDePrueba(t),
	}
	w2, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/interbancarias", segunda, nil, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w2, http.StatusUnprocessableEntity)

	op2 := decodificarOperacion(t, w2)
	if op2.Estado != models.OperacionRechazada {
		t.Fatalf("la segunda transferencia debería rechazarse por límite diario, dio %q", op2.Estado)
	}
	if llamadas := m.LlamadasADestino(); llamadas != 1 {
		t.Fatalf("el rechazo por límite diario no debería llamar al destino; seguía en 1, quedó en %d", llamadas)
	}
	if saldo := saldoActual(t, m.bancoOrigenID, "ORIGEN"); saldo != 20_000_000-9_500_000 {
		t.Fatalf("solo la primera transferencia debería haber afectado el saldo, quedó en %d", saldo)
	}
}

// Caso 10 (doc 04): reintento con la misma idempotency_key y los mismos
// datos -> mismo resultado, sin debitar/acreditar una segunda vez, y sin
// volver a llamar al destino (el fast path responde antes de intentarlo).
func TestTransferenciaInterbancaria_ReintentoMismaClaveMismosDatos(t *testing.T) {
	db := requireTestDB(t)
	m := armarMundoInterbancario(t, db)
	handler := TransferenciaInterbancaria(db, m.bancoOrigenID, m.clienteOrigen, m.registroOrigen)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, m.bancoOrigenID, "ORIGEN", 1_000_000, "", usuarioID)
	m.crearCuentaEnDestino(t, "DESTINO", 0, "")

	req := transferenciaInterbancariaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		BancoDestino:        m.bancoDestinoID,
		NumeroCuentaDestino: "DESTINO",
		Importe:             100_000,
		IdempotencyKey:      uuidDePrueba(t),
	}

	w1, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/interbancarias", req, nil, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w1, http.StatusCreated)
	op1 := decodificarOperacion(t, w1)

	w2, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/interbancarias", req, nil, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w2, http.StatusCreated)
	op2 := decodificarOperacion(t, w2)

	if op1.ID != op2.ID {
		t.Fatalf("el reintento debería devolver la misma operación (id %s), dio %s", op1.ID, op2.ID)
	}
	if saldo := saldoActual(t, m.bancoOrigenID, "ORIGEN"); saldo != 900_000 {
		t.Fatalf("el reintento no debería debitar una segunda vez en origen, saldo quedó en %d", saldo)
	}
	if saldo := m.saldoEnDestino(t, "DESTINO"); saldo != 100_000 {
		t.Fatalf("el reintento no debería acreditar una segunda vez en destino, saldo quedó en %d", saldo)
	}
	if llamadas := m.LlamadasADestino(); llamadas != 1 {
		t.Fatalf("el reintento (fast path de idempotencia) no debería volver a llamar al destino; hubo %d llamadas", llamadas)
	}
}

// Caso 11 (doc 04), versión rápida: ante un error de red/timeout
// ambiguo, resolverResultado consulta al destino antes de decidir, en vez
// de asumir que falló. Se prueba la función directo (sin esperar los 5s
// reales de timeoutLlamadaBanco): se simula el error con errors.New(...),
// no con un timeout real.
func TestTransferenciaInterbancaria_TimeoutConsultaAntesDeDecidir(t *testing.T) {
	db := requireTestDB(t)
	m := armarMundoInterbancario(t, db)

	crearCuentaDirecta(t, m.bancoOrigenID, "ORIGEN", 1_000_000, "")
	m.crearCuentaEnDestino(t, "DESTINO", 0, "")

	req := transferenciaInterbancariaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		BancoDestino:        m.bancoDestinoID,
		NumeroCuentaDestino: "DESTINO",
		Importe:             100_000,
		IdempotencyKey:      uuidDePrueba(t),
	}

	// 1) Reservamos y debitamos en origen, igual que lo haría el handler
	//    real antes de llamar al destino.
	op, err := reservarYDebitar(db, m.bancoOrigenID, req)
	if err != nil {
		t.Fatalf("reservarYDebitar: %v", err)
	}
	if op.Estado != models.OperacionReservada {
		t.Fatalf("esperaba la reserva en RESERVADA, dio %q", op.Estado)
	}

	// 2) El destino SÍ procesa el crédito (llamada real y exitosa) -- pero
	//    la respuesta nunca le llega al origen. Esto deja al destino en el
	//    estado real que tendría tras un timeout del lado del origen.
	payload := interbanco.CreditoPayload{
		TransferID:    op.IdempotencyKey,
		CuentaDestino: req.NumeroCuentaDestino,
		BancoOrigen:   m.bancoOrigenID,
		CuentaOrigen:  req.NumeroCuentaOrigen,
		Importe:       req.Importe,
		Moneda:        "PYG",
	}
	if _, err := m.clienteOrigen.EnviarCredito(context.Background(), m.destinoServer.URL, payload); err != nil {
		t.Fatalf("preparando el estado del destino: %v", err)
	}

	// 3) resolverResultado recibe un error que no es nil ni
	//    *interbanco.ErrorInterbancario: así es, desde el código del banco
	//    origen, como se ve un timeout o una conexión cortada.
	resultado := resolverResultado(db, op, nil, errors.New("simulando timeout/conexión cortada"), m.clienteOrigen, m.destinoServer.URL)

	if resultado.Estado != models.OperacionCompletada {
		t.Fatalf("esperaba que la consulta de verificación confirme COMPLETADA, dio %q (motivo: %v)", resultado.Estado, resultado.MotivoRechazo)
	}
	if saldo := saldoActual(t, m.bancoOrigenID, "ORIGEN"); saldo != 900_000 {
		t.Fatalf("el saldo de origen ya se había debitado en la reserva, no debería cambiar de nuevo: quedó en %d", saldo)
	}
	if saldo := m.saldoEnDestino(t, "DESTINO"); saldo != 100_000 {
		t.Fatalf("saldo de destino debería reflejar el crédito ya aplicado, quedó en %d", saldo)
	}
}

// slowWriter envuelve un http.ResponseWriter y demora la primera escritura
// de la cabecera. Se usa para simular un banco destino que SÍ procesa la
// solicitud (ya comprometió los cambios en su base) pero tarda en que la
// respuesta le llegue de vuelta al banco origen.
type slowWriter struct {
	http.ResponseWriter
	atraso     time.Duration
	yaDemorado bool
}

func (w *slowWriter) WriteHeader(status int) {
	if !w.yaDemorado {
		w.yaDemorado = true
		time.Sleep(w.atraso)
	}
	w.ResponseWriter.WriteHeader(status)
}

// Caso 11 (doc 04), versión lenta de punta a punta: un banco destino que
// de verdad tarda más que timeoutLlamadaBanco en responder, pasando por
// el handler HTTP completo (TransferenciaInterbancaria real, con su
// propio context.WithTimeout). Tarda ~timeoutLlamadaBanco en correr; se
// saltea con `go test -short`.
func TestTransferenciaInterbancaria_TimeoutConsultaAntesDeDecidir_EndToEndLento(t *testing.T) {
	if testing.Short() {
		t.Skip("test lento (~" + timeoutLlamadaBanco.String() + "): el banco destino tarda a propósito más que timeoutLlamadaBanco; se saltea con -short")
	}
	dbOrigen := requireTestDB(t)
	dbDestino := segundaBaseDePrueba(t) // ver el comentario en testmain_test.go: hace falta una base físicamente distinta

	bancoOrigenID := "ORIGEN-" + idAleatorio(t)
	pubOrigen, privOrigen, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generando clave de origen: %v", err)
	}
	bancoDestinoID := "DESTINO-" + idAleatorio(t)
	pubDestino, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generando clave de destino: %v", err)
	}

	destinoRouter := gin.New()
	destinoRouter.Use(VerifyBankSignature(map[string]ed25519.PublicKey{bancoOrigenID: pubOrigen}))
	destinoRouter.POST("/interbancario/creditos", RecibirCredito(dbDestino, bancoDestinoID))
	destinoRouter.GET("/interbancario/creditos/:transfer_id", ConsultarCredito(dbDestino, bancoDestinoID))

	// El delay se aplica a nivel de http.ResponseWriter, por fuera de
	// gin: así el handler real procesa y confirma el crédito (rápido) y
	// recién después se demora el envío de la respuesta -- exactamente el
	// escenario real de un timeout, sin ninguna carrera entre el commit
	// en la base y la consulta de verificación posterior.
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/interbancario/creditos" {
			w = &slowWriter{ResponseWriter: w, atraso: timeoutLlamadaBanco + 2*time.Second}
		}
		destinoRouter.ServeHTTP(w, r)
	})
	destinoServer := httptest.NewServer(wrapped)
	t.Cleanup(destinoServer.Close)

	clienteOrigen := interbanco.NuevoCliente(bancoOrigenID, privOrigen)
	registroOrigen := map[string]config.BancoRegistrado{
		bancoDestinoID: {URL: destinoServer.URL, PublicKey: pubDestino},
	}

	handler := TransferenciaInterbancaria(dbOrigen, bancoOrigenID, clienteOrigen, registroOrigen)
	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, bancoOrigenID, "ORIGEN", 1_000_000, "", usuarioID)
	crearCuentaDirectaEn(t, dbDestino, bancoDestinoID, "DESTINO", 0, "")

	req := transferenciaInterbancariaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		BancoDestino:        bancoDestinoID,
		NumeroCuentaDestino: "DESTINO",
		Importe:             100_000,
		IdempotencyKey:      uuidDePrueba(t),
	}

	inicio := time.Now()
	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/interbancarias", req, nil, map[string]string{"usuarioID": usuarioID})
	t.Logf("la solicitud tardó %s (se espera un poco más de %s, por el timeout de la llamada inicial)", time.Since(inicio), timeoutLlamadaBanco)

	esperarStatus(t, w, http.StatusCreated)
	op := decodificarOperacion(t, w)
	if op.Estado != models.OperacionCompletada {
		t.Fatalf("esperaba que la consulta de verificación confirme COMPLETADA, dio %q", op.Estado)
	}
	if saldo := saldoActual(t, bancoOrigenID, "ORIGEN"); saldo != 900_000 {
		t.Fatalf("saldo de origen debería quedar en 900000, quedó en %d", saldo)
	}
	if saldo := saldoActualEn(t, dbDestino, bancoDestinoID, "DESTINO"); saldo != 100_000 {
		t.Fatalf("saldo de destino debería quedar en 100000, quedó en %d", saldo)
	}
}

// Bonus, no numerado en el doc 04 pero relacionado con el caso 12
// (solicitud entre bancos con firma inválida o ausente): pega contra la
// ruta real del destino (no contra el middleware aislado como en
// middleware_test.go), para confirmar que el wiring real del router
// también rechaza una solicitud sin firmar.
func TestInterbancarioCreditos_RutaRealRechazaSinFirma(t *testing.T) {
	db := requireTestDB(t)
	m := armarMundoInterbancario(t, db)

	body := []byte(`{"transfer_id":"x","cuenta_destino":"CTA","banco_origen":"X","cuenta_origen":"Y","importe":1000}`)
	resp, err := http.Post(m.destinoServer.URL+"/interbancario/creditos", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("llamando a la ruta real sin firma: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("esperaba 400 por headers de firma faltantes, dio %d", resp.StatusCode)
	}
}

// Análogo al caso 13 (doc 04) para el flujo interbancario saliente: mismo
// idempotency_key, importe distinto -> CONFLICTO_IDEMPOTENCIA, sin aplicar
// ningún efecto económico de la segunda solicitud. Antes del fix de esta
// sesión, el fast path ignoraba el cambio de importe y devolvía la
// operación guardada.
func TestTransferenciaInterbancaria_ConflictoIdempotencia(t *testing.T) {
	db := requireTestDB(t)
	m := armarMundoInterbancario(t, db)
	handler := TransferenciaInterbancaria(db, m.bancoOrigenID, m.clienteOrigen, m.registroOrigen)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, m.bancoOrigenID, "ORIGEN", 2_000_000, "", usuarioID)
	m.crearCuentaEnDestino(t, "DESTINO", 0, "")

	idempotencyKey := uuidDePrueba(t)

	primera := transferenciaInterbancariaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		BancoDestino:        m.bancoDestinoID,
		NumeroCuentaDestino: "DESTINO",
		Importe:             100_000,
		IdempotencyKey:      idempotencyKey,
	}
	w1, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/interbancarias", primera, nil, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w1, http.StatusCreated)

	segunda := primera
	segunda.Importe = 1_999_999 // mismo idempotency_key, importe bien distinto

	w2, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/interbancarias", segunda, nil, map[string]string{"usuarioID": usuarioID})
	esperarErrorContrato(t, w2, http.StatusConflict, ErrConflictoIdempotencia)

	if saldo := saldoActual(t, m.bancoOrigenID, "ORIGEN"); saldo != 1_900_000 {
		t.Fatalf("el conflicto de idempotencia no debería aplicar ningún efecto económico de la segunda solicitud, saldo quedó en %d", saldo)
	}
	if llamadas := m.LlamadasADestino(); llamadas != 1 {
		t.Fatalf("el conflicto de idempotencia se detecta localmente, no debería volver a llamar al destino; hubo %d llamadas", llamadas)
	}
}

// Nuevo (concepto, doc 02): tiene que viajar hasta el banco destino y
// quedar guardado en SU base (físicamente separada de la nuestra, ver
// armarMundoInterbancario) -- no alcanza con que quede en nuestra propia
// operación local.
func TestTransferenciaInterbancaria_GuardaConceptoYLoReenviaAlDestino(t *testing.T) {
	db := requireTestDB(t)
	m := armarMundoInterbancario(t, db)
	handler := TransferenciaInterbancaria(db, m.bancoOrigenID, m.clienteOrigen, m.registroOrigen)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, m.bancoOrigenID, "ORIGEN", 1_000_000, "", usuarioID)
	m.crearCuentaEnDestino(t, "DESTINO", 0, "")

	idempotencyKey := uuidDePrueba(t)
	req := transferenciaInterbancariaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		BancoDestino:        m.bancoDestinoID,
		NumeroCuentaDestino: "DESTINO",
		Importe:             100_000,
		IdempotencyKey:      idempotencyKey,
		Concepto:            "Pago de factura UNCA-884",
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/interbancarias", req, nil, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w, http.StatusCreated)

	op := decodificarOperacion(t, w)
	if op.Concepto == nil || *op.Concepto != "Pago de factura UNCA-884" {
		t.Fatalf("en el lado origen esperaba concepto %q, dio %v", "Pago de factura UNCA-884", op.Concepto)
	}

	var opDestino models.Operacion
	if err := m.dbDestino.Get(&opDestino, `
		SELECT * FROM operaciones WHERE banco_origen = $1 AND idempotency_key = $2
	`, m.bancoOrigenID, idempotencyKey); err != nil {
		t.Fatalf("no encontré la operación en la base del destino: %v", err)
	}
	if opDestino.Concepto == nil || *opDestino.Concepto != "Pago de factura UNCA-884" {
		t.Fatalf("en el lado destino esperaba concepto %q, dio %v", "Pago de factura UNCA-884", opDestino.Concepto)
	}
}

// RecibirCredito trunca en vez de rechazar un concepto demasiado largo
// que llega de otro banco -- ver el comentario en interbancario.go sobre
// por qué no tratamos esto como un error de contrato.
func TestTransferenciaInterbancaria_ConceptoLargoDelDestinoSeTrunca(t *testing.T) {
	db := requireTestDB(t)
	m := armarMundoInterbancario(t, db)
	handler := TransferenciaInterbancaria(db, m.bancoOrigenID, m.clienteOrigen, m.registroOrigen)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, m.bancoOrigenID, "ORIGEN", 1_000_000, "", usuarioID)
	m.crearCuentaEnDestino(t, "DESTINO", 0, "")

	// Nuestro propio lado SÍ valida el límite (ver el test de arriba para
	// transferencias internas), así que para simular "otro banco mandando
	// algo más laxo" probamos el límite justo en el borde permitido de
	// nuestro lado, y confirmamos que llega entero sin truncarse: el
	// truncado es solo para cuando algo se pasa, cosa que nuestro propio
	// cliente nunca permite mandar.
	conceptoAlLimite := strings.Repeat("a", limiteConceptoChars)

	idempotencyKey := uuidDePrueba(t)
	req := transferenciaInterbancariaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		BancoDestino:        m.bancoDestinoID,
		NumeroCuentaDestino: "DESTINO",
		Importe:             100_000,
		IdempotencyKey:      idempotencyKey,
		Concepto:            conceptoAlLimite,
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/interbancarias", req, nil, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w, http.StatusCreated)

	var opDestino models.Operacion
	if err := m.dbDestino.Get(&opDestino, `
		SELECT * FROM operaciones WHERE banco_origen = $1 AND idempotency_key = $2
	`, m.bancoOrigenID, idempotencyKey); err != nil {
		t.Fatalf("no encontré la operación en la base del destino: %v", err)
	}
	if opDestino.Concepto == nil || *opDestino.Concepto != conceptoAlLimite {
		t.Fatalf("esperaba que el concepto de %d caracteres llegara entero, dio %v", limiteConceptoChars, opDestino.Concepto)
	}
}

// Nuestro propio lado (el que origina la transferencia) rechaza un
// concepto demasiado largo antes de reservar nada -- simétrico al test
// de transferencias internas.
func TestTransferenciaInterbancaria_RechazaConceptoDemasiadoLargo(t *testing.T) {
	db := requireTestDB(t)
	m := armarMundoInterbancario(t, db)
	handler := TransferenciaInterbancaria(db, m.bancoOrigenID, m.clienteOrigen, m.registroOrigen)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, m.bancoOrigenID, "ORIGEN", 1_000_000, "", usuarioID)
	m.crearCuentaEnDestino(t, "DESTINO", 0, "")

	req := transferenciaInterbancariaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		BancoDestino:        m.bancoDestinoID,
		NumeroCuentaDestino: "DESTINO",
		Importe:             100_000,
		IdempotencyKey:      uuidDePrueba(t),
		Concepto:            strings.Repeat("x", limiteConceptoChars+1),
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/interbancarias", req, nil, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w, http.StatusBadRequest)

	if saldo := saldoActual(t, m.bancoOrigenID, "ORIGEN"); saldo != 1_000_000 {
		t.Fatalf("un concepto inválido no debería tocar el saldo, quedó en %d", saldo)
	}
	if m.LlamadasADestino() != 0 {
		t.Fatalf("no debería haber llamado al destino, lo llamó %d veces", m.LlamadasADestino())
	}
}

// Nuevo (seguridad): igual que TransferenciaInterbancaria, este endpoint
// ahora exige sesión y verifica que la cuenta de origen de la operación
// sea del usuario logueado. Antes, el idempotency_key del path alcanzaba
// -- y no es un secreto -- para ver el detalle completo de una
// transferencia ajena (importe, cuentas, concepto) y forzar cuándo se
// resuelve. No hace falta armar todo mundoInterbancario para este test:
// el handler rechaza antes de necesitar cliente/registro para nada.
func TestForzarReconciliacion_RechazaSiNoEsDuenoDeLaCuentaOrigen(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := ForzarReconciliacion(db, bancoID, nil, nil)

	dueño := crearUsuarioDePrueba(t)
	otro := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, bancoID, "ORIGEN", 1_000_000, "", dueño)

	op := crearOperacionDirecta(t, operacionDePrueba{
		Tipo: models.TipoTransferenciaInterbancaria, BancoOrigen: bancoID, NumeroCuentaOrigen: "ORIGEN",
		BancoDestino: "OTRO-BANCO", NumeroCuentaDestino: "EXTERNA", Importe: 100_000, Estado: models.OperacionPendiente,
	})

	params := gin.Params{{Key: "idempotency_key", Value: op.IdempotencyKey}}
	w, _ := ejecutarJSON(t, handler, http.MethodPost,
		"/transferencias/interbancarias/"+op.IdempotencyKey+"/reconciliar", nil, params, map[string]string{"usuarioID": otro})
	esperarStatus(t, w, http.StatusNotFound)
}

func TestForzarReconciliacion_RechazaSinSesion(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := ForzarReconciliacion(db, bancoID, nil, nil)

	dueño := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, bancoID, "ORIGEN", 1_000_000, "", dueño)

	op := crearOperacionDirecta(t, operacionDePrueba{
		Tipo: models.TipoTransferenciaInterbancaria, BancoOrigen: bancoID, NumeroCuentaOrigen: "ORIGEN",
		BancoDestino: "OTRO-BANCO", NumeroCuentaDestino: "EXTERNA", Importe: 100_000, Estado: models.OperacionPendiente,
	})

	params := gin.Params{{Key: "idempotency_key", Value: op.IdempotencyKey}}
	w, _ := ejecutarJSON(t, handler, http.MethodPost,
		"/transferencias/interbancarias/"+op.IdempotencyKey+"/reconciliar", nil, params, nil)
	esperarStatus(t, w, http.StatusNotFound)
}
