package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/sudameris/backend/internal/models"
)

// dispararConcurrente ejecuta n copias de hacer() lanzadas lo más juntas
// posible (todas esperan en una barrera antes de arrancar) y devuelve las
// n respuestas en orden. A diferencia de armar el escenario a mano con
// crearOperacionDirecta (como en limites_test.go), acá lo que se quiere
// ejercitar es la carrera real contra la base -- dos (o más) INSERT
// intentando la misma fila de verdad, no un estado ya resuelto de
// antemano.
//
// hacer() corre en una goroutine que el test principal no controla, así
// que NO debe llamar a ningún método de *testing.T (Fatalf, Helper, etc.)
// -- eso violaría las reglas de la propia librería "testing" y podría
// colgar o corromper el resultado del test. Cualquier chequeo con t va
// después, en la goroutine principal, sobre las respuestas ya recolectadas.
func dispararConcurrente(n int, hacer func() *httptest.ResponseRecorder) []*httptest.ResponseRecorder {
	var wg sync.WaitGroup
	listo := make(chan struct{})
	resultados := make([]*httptest.ResponseRecorder, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-listo
			resultados[i] = hacer()
		}(i)
	}
	close(listo)
	wg.Wait()
	return resultados
}

// Regression test del punto "carrera de idempotencia concurrente"
// (hueco encontrado en revisión de código, no es uno de los 20 casos del
// doc 04): n solicitudes IDÉNTICAS (mismo idempotency_key, mismos datos)
// disparadas en simultáneo tienen que terminar todas de acuerdo con el
// mismo resultado -- nunca un 500 para la que pierde la carrera del
// INSERT contra el UNIQUE (banco_origen, idempotency_key).
func TestTransferenciaInterna_CarreraDeIdempotenciaConcurrente(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := TransferenciaInterna(db, bancoID)

	usuarioID := crearUsuarioDePrueba(t)
	origen := "ORIGEN-" + idAleatorio(t)
	destino := "DESTINO-" + idAleatorio(t)
	crearCuentaDeUsuarioDirecta(t, bancoID, origen, 1_000_000, "", usuarioID)
	crearCuentaDirecta(t, bancoID, destino, 0, "")

	req := transferenciaInternaRequest{
		NumeroCuentaOrigen:  origen,
		NumeroCuentaDestino: destino,
		Importe:             100_000,
		IdempotencyKey:      uuidDePrueba(t),
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("serializando body: %v", err)
	}

	const n = 10
	recorders := dispararConcurrente(n, func() *httptest.ResponseRecorder {
		httpReq := httptest.NewRequest(http.MethodPost, "/transferencias/internas", bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httpReq
		c.Set("usuarioID", usuarioID)
		handler(c)
		return w
	})

	var completadas, rechazadas int
	for i, w := range recorders {
		if w.Code == http.StatusInternalServerError {
			t.Fatalf("respuesta %d: 500 inesperado (perdió la carrera de idempotencia sin recuperarse): %s", i, w.Body.String())
		}
		var op models.Operacion
		if err := json.Unmarshal(w.Body.Bytes(), &op); err != nil {
			t.Fatalf("respuesta %d no es una operación válida: %v (status %d, body %s)", i, err, w.Code, w.Body.String())
		}
		switch op.Estado {
		case models.OperacionCompletada:
			completadas++
		case models.OperacionRechazada:
			rechazadas++
		default:
			t.Fatalf("respuesta %d: estado inesperado %q", i, op.Estado)
		}
	}

	// Las n solicitudes son IDÉNTICAS, así que tienen que terminar todas
	// de acuerdo: o las n ven la misma operación ya completada, o las n
	// ven el mismo rechazo -- nunca una mezcla, porque eso significaría
	// que alguna no encontró la fila que realmente ganó la carrera.
	if completadas != n && rechazadas != n {
		t.Fatalf("las %d respuestas deberían coincidir entre sí (vio %d completadas, %d rechazadas) -- alguna no encontró la operación ganadora", n, completadas, rechazadas)
	}

	saldoEsperado := int64(1_000_000)
	if completadas == n {
		saldoEsperado -= req.Importe
	}
	if saldo := saldoActual(t, bancoID, origen); saldo != saldoEsperado {
		t.Fatalf("el saldo debería reflejar un solo movimiento (esperado %d), quedó en %d", saldoEsperado, saldo)
	}

	var cantidad int
	if err := db.Get(&cantidad, `
		SELECT count(*) FROM operaciones WHERE banco_origen = $1 AND idempotency_key = $2
	`, bancoID, req.IdempotencyKey); err != nil {
		t.Fatalf("contando operaciones: %v", err)
	}
	if cantidad != 1 {
		t.Fatalf("debería haber una sola fila en operaciones para esta idempotency_key, hay %d", cantidad)
	}
}

// Mismo escenario que el test anterior, pero en AutorizarRetiroCajero: es
// el caso más realista de los cuatro, porque un cajero reintentando el
// mismo operacion_id tras un timeout -- mientras la solicitud original
// todavía se está procesando -- es justo la situación para la que existe
// la idempotencia (doc 03).
func TestAutorizarRetiroCajero_CarreraDeIdempotenciaConcurrente(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)        // banco emisor, dueño de la cuenta
	bankID := "CAJERO-" + idAleatorio(t) // banco dueño del cajero
	handler := AutorizarRetiroCajero(db, bancoID)

	numeroCuenta := "CTA-" + idAleatorio(t)
	crearCuentaDirecta(t, bancoID, numeroCuenta, 1_000_000, "1234")

	req := autorizacionCajeroRequest{
		OperacionID:  uuidDePrueba(t),
		BancoCuenta:  bancoID,
		NumeroCuenta: numeroCuenta,
		Pin:          "1234",
		Importe:      500_000,
		Moneda:       "PYG",
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("serializando body: %v", err)
	}

	const n = 10
	recorders := dispararConcurrente(n, func() *httptest.ResponseRecorder {
		httpReq := httptest.NewRequest(http.MethodPost, "/cajero/autorizaciones", bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httpReq
		c.Set("bankID", bankID)
		handler(c)
		return w
	})

	var autorizadas, rechazadas int
	for i, w := range recorders {
		if w.Code == http.StatusInternalServerError {
			t.Fatalf("respuesta %d: 500 inesperado (perdió la carrera de idempotencia sin recuperarse): %s", i, w.Body.String())
		}
		resp := decodificarAutorizacion(t, w)
		switch resp.Estado {
		case "AUTORIZADO":
			autorizadas++
		case "RECHAZADO":
			rechazadas++
		default:
			t.Fatalf("respuesta %d: estado inesperado %q", i, resp.Estado)
		}
	}

	if autorizadas != n && rechazadas != n {
		t.Fatalf("las %d respuestas deberían coincidir entre sí (vio %d autorizadas, %d rechazadas) -- alguna no encontró la operación ganadora", n, autorizadas, rechazadas)
	}

	saldoEsperado := int64(1_000_000)
	if autorizadas == n {
		saldoEsperado -= req.Importe
	}
	if saldo := saldoActual(t, bancoID, numeroCuenta); saldo != saldoEsperado {
		t.Fatalf("el saldo debería reflejar un solo movimiento (esperado %d), quedó en %d", saldoEsperado, saldo)
	}

	var cantidad int
	if err := db.Get(&cantidad, `
		SELECT count(*) FROM operaciones WHERE banco_origen = $1 AND idempotency_key = $2
	`, bankID, req.OperacionID); err != nil {
		t.Fatalf("contando operaciones: %v", err)
	}
	if cantidad != 1 {
		t.Fatalf("debería haber una sola fila en operaciones para este operacion_id, hay %d", cantidad)
	}
}
