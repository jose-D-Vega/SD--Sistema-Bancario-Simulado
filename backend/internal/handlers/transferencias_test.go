package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sudameris/backend/internal/models"
)

func cambiarEstadoCuentaDirecto(t *testing.T, bancoID, numeroCuenta, estado string) {
	t.Helper()
	db := requireTestDB(t)
	if _, err := db.Exec(`UPDATE cuentas SET estado = $1 WHERE banco_id = $2 AND numero_cuenta = $3`,
		estado, bancoID, numeroCuenta); err != nil {
		t.Fatalf("cambiando estado de cuenta de prueba: %v", err)
	}
}

func decodificarOperacion(t *testing.T, w *httptest.ResponseRecorder) models.Operacion {
	t.Helper()
	var op models.Operacion
	if err := json.Unmarshal(w.Body.Bytes(), &op); err != nil {
		t.Fatalf("la respuesta no es una Operacion válida: %v\nbody: %s", err, w.Body.String())
	}
	return op
}

// Caso 1 (doc 04): transferencia interna exitosa.
func TestTransferenciaInterna_Exitosa(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := TransferenciaInterna(db, bancoID)

	crearCuentaDirecta(t, bancoID, "ORIGEN", 1_000_000, "")
	crearCuentaDirecta(t, bancoID, "DESTINO", 0, "")

	req := transferenciaInternaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		NumeroCuentaDestino: "DESTINO",
		Importe:             100_000,
		IdempotencyKey:      uuidDePrueba(t),
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/internas", req, nil, nil)
	esperarStatus(t, w, http.StatusCreated)

	op := decodificarOperacion(t, w)
	if op.Estado != models.OperacionCompletada {
		t.Fatalf("esperaba estado COMPLETADA, dio %q", op.Estado)
	}

	if saldo := saldoActual(t, bancoID, "ORIGEN"); saldo != 900_000 {
		t.Fatalf("saldo de origen debería quedar en 900000, quedó en %d", saldo)
	}
	if saldo := saldoActual(t, bancoID, "DESTINO"); saldo != 100_000 {
		t.Fatalf("saldo de destino debería quedar en 100000, quedó en %d", saldo)
	}
}

// Caso 2 (doc 04): rechazo por fondos insuficientes, ningún saldo cambia.
func TestTransferenciaInterna_FondosInsuficientes(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := TransferenciaInterna(db, bancoID)

	crearCuentaDirecta(t, bancoID, "ORIGEN", 1_000, "")
	crearCuentaDirecta(t, bancoID, "DESTINO", 0, "")

	req := transferenciaInternaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		NumeroCuentaDestino: "DESTINO",
		Importe:             5_000,
		IdempotencyKey:      uuidDePrueba(t),
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/internas", req, nil, nil)
	esperarStatus(t, w, http.StatusUnprocessableEntity)

	op := decodificarOperacion(t, w)
	if op.Estado != models.OperacionRechazada {
		t.Fatalf("esperaba estado RECHAZADA, dio %q", op.Estado)
	}
	if op.MotivoRechazo == nil {
		t.Fatal("esperaba un motivo_rechazo")
	}

	if saldo := saldoActual(t, bancoID, "ORIGEN"); saldo != 1_000 {
		t.Fatalf("un rechazo no debería tocar el saldo de origen, quedó en %d", saldo)
	}
	if saldo := saldoActual(t, bancoID, "DESTINO"); saldo != 0 {
		t.Fatalf("un rechazo no debería tocar el saldo de destino, quedó en %d", saldo)
	}
}

// Caso 3 (doc 04): rechazo por cuenta destino inexistente.
func TestTransferenciaInterna_CuentaDestinoInexistente(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := TransferenciaInterna(db, bancoID)

	crearCuentaDirecta(t, bancoID, "ORIGEN", 1_000_000, "")
	// "DESTINO" nunca se crea.

	req := transferenciaInternaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		NumeroCuentaDestino: "DESTINO",
		Importe:             100_000,
		IdempotencyKey:      uuidDePrueba(t),
	}

	w, body := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/internas", req, nil, nil)
	esperarStatus(t, w, http.StatusNotFound)

	if msg, _ := body["error"].(string); !strings.Contains(msg, "no encontrada") {
		t.Fatalf("esperaba un mensaje de cuenta no encontrada, dio: %v", body)
	}
	if saldo := saldoActual(t, bancoID, "ORIGEN"); saldo != 1_000_000 {
		t.Fatalf("una cuenta destino inexistente no debería tocar el saldo de origen, quedó en %d", saldo)
	}
}

// Caso 4 (doc 04): rechazo por cuenta origen bloqueada.
func TestTransferenciaInterna_CuentaOrigenBloqueada(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := TransferenciaInterna(db, bancoID)

	crearCuentaDirecta(t, bancoID, "ORIGEN", 1_000_000, "")
	crearCuentaDirecta(t, bancoID, "DESTINO", 0, "")
	cambiarEstadoCuentaDirecto(t, bancoID, "ORIGEN", models.EstadoBloqueada)

	req := transferenciaInternaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		NumeroCuentaDestino: "DESTINO",
		Importe:             100_000,
		IdempotencyKey:      uuidDePrueba(t),
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/internas", req, nil, nil)
	esperarStatus(t, w, http.StatusUnprocessableEntity)

	op := decodificarOperacion(t, w)
	if op.Estado != models.OperacionRechazada {
		t.Fatalf("esperaba estado RECHAZADA, dio %q", op.Estado)
	}
	if saldo := saldoActual(t, bancoID, "ORIGEN"); saldo != 1_000_000 {
		t.Fatalf("un rechazo no debería tocar el saldo de origen, quedó en %d", saldo)
	}
}

// Caso 5 (doc 04): rechazo por superar el límite por operación (G. 10.000.000).
func TestTransferenciaInterna_ExcedeLimitePorOperacion(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := TransferenciaInterna(db, bancoID)

	crearCuentaDirecta(t, bancoID, "ORIGEN", 50_000_000, "")
	crearCuentaDirecta(t, bancoID, "DESTINO", 0, "")

	req := transferenciaInternaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		NumeroCuentaDestino: "DESTINO",
		Importe:             LimiteMaximoTransferencia + 1,
		IdempotencyKey:      uuidDePrueba(t),
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/internas", req, nil, nil)
	esperarStatus(t, w, http.StatusUnprocessableEntity)

	op := decodificarOperacion(t, w)
	if op.Estado != models.OperacionRechazada {
		t.Fatalf("esperaba estado RECHAZADA, dio %q", op.Estado)
	}
	if saldo := saldoActual(t, bancoID, "ORIGEN"); saldo != 50_000_000 {
		t.Fatalf("un rechazo no debería tocar el saldo de origen, quedó en %d", saldo)
	}
}

// Bonus (no es un caso explícito de la sección A, pero ejercita la misma
// lógica compartida con los casos 9 y 18): dos transferencias que, cada
// una por separado, están dentro del límite por operación, pero juntas
// superan el límite diario combinado.
//
// Nota: usa la fecha/hora real de la máquina que corre el test, igual
// que totalSalidoHoy. Es teóricamente flaky solo si el test corre justo
// en el instante en que cruza medianoche en America/Asuncion.
func TestTransferenciaInterna_LimiteDiarioCombinado(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := TransferenciaInterna(db, bancoID)

	crearCuentaDirecta(t, bancoID, "ORIGEN", 50_000_000, "")
	crearCuentaDirecta(t, bancoID, "DESTINO", 0, "")

	primera := transferenciaInternaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		NumeroCuentaDestino: "DESTINO",
		Importe:             6_000_000,
		IdempotencyKey:      uuidDePrueba(t),
	}
	w1, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/internas", primera, nil, nil)
	esperarStatus(t, w1, http.StatusCreated)

	segunda := transferenciaInternaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		NumeroCuentaDestino: "DESTINO",
		Importe:             6_000_000,
		IdempotencyKey:      uuidDePrueba(t),
	}
	w2, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/internas", segunda, nil, nil)
	esperarStatus(t, w2, http.StatusUnprocessableEntity)

	op := decodificarOperacion(t, w2)
	if op.Estado != models.OperacionRechazada {
		t.Fatalf("la segunda transferencia debería rechazarse por límite diario, dio estado %q", op.Estado)
	}
	if saldo := saldoActual(t, bancoID, "ORIGEN"); saldo != 50_000_000-6_000_000 {
		t.Fatalf("solo la primera transferencia debería haber afectado el saldo, quedó en %d", saldo)
	}
}

// Parte del caso 10 (doc 04) aplicada a transferencias internas: reintentar
// con la misma idempotency_key y los mismos datos debe devolver el mismo
// resultado, sin aplicar el efecto económico una segunda vez.
func TestTransferenciaInterna_ReintentoMismaClaveMismosDatos(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := TransferenciaInterna(db, bancoID)

	crearCuentaDirecta(t, bancoID, "ORIGEN", 1_000_000, "")
	crearCuentaDirecta(t, bancoID, "DESTINO", 0, "")

	req := transferenciaInternaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		NumeroCuentaDestino: "DESTINO",
		Importe:             100_000,
		IdempotencyKey:      uuidDePrueba(t),
	}

	w1, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/internas", req, nil, nil)
	esperarStatus(t, w1, http.StatusCreated)
	op1 := decodificarOperacion(t, w1)

	w2, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/internas", req, nil, nil)
	esperarStatus(t, w2, http.StatusCreated)
	op2 := decodificarOperacion(t, w2)

	if op1.ID != op2.ID {
		t.Fatalf("el reintento debería devolver la misma operación (id %s), dio %s", op1.ID, op2.ID)
	}
	if saldo := saldoActual(t, bancoID, "ORIGEN"); saldo != 900_000 {
		t.Fatalf("el reintento no debería debitar una segunda vez, saldo quedó en %d", saldo)
	}
}

// Caso 13 (doc 04): misma idempotency_key, datos distintos (acá, el
// importe) -> CONFLICTO_IDEMPOTENCIA, sin aplicar ningún efecto económico
// de la segunda solicitud. Este es exactamente el caso que estaba roto
// antes del fix de esta sesión: el fast path devolvía el resultado
// guardado sin chequear si los datos de la solicitud coincidían.
func TestTransferenciaInterna_ConflictoIdempotencia(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := TransferenciaInterna(db, bancoID)

	crearCuentaDirecta(t, bancoID, "ORIGEN", 1_000_000, "")
	crearCuentaDirecta(t, bancoID, "DESTINO", 0, "")

	idempotencyKey := uuidDePrueba(t)

	primera := transferenciaInternaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		NumeroCuentaDestino: "DESTINO",
		Importe:             100_000,
		IdempotencyKey:      idempotencyKey,
	}
	w1, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/internas", primera, nil, nil)
	esperarStatus(t, w1, http.StatusCreated)

	segunda := primera
	segunda.Importe = 999_999 // mismo key, importe distinto

	w2, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/internas", segunda, nil, nil)
	esperarErrorContrato(t, w2, http.StatusConflict, ErrConflictoIdempotencia)

	if saldo := saldoActual(t, bancoID, "ORIGEN"); saldo != 900_000 {
		t.Fatalf("el conflicto de idempotencia no debería aplicar ningún efecto económico, saldo quedó en %d", saldo)
	}
}
