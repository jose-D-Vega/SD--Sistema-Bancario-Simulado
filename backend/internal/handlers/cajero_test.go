package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type autorizacionRespuesta struct {
	OperacionID   string `json:"operacion_id"`
	Estado        string `json:"estado"`
	MotivoRechazo string `json:"motivo_rechazo"`
}

func decodificarAutorizacion(t *testing.T, w *httptest.ResponseRecorder) autorizacionRespuesta {
	t.Helper()
	var resp autorizacionRespuesta
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("la respuesta no es una autorización válida: %v\nbody: %s", err, w.Body.String())
	}
	return resp
}

// Caso 15 (doc 04): retiro exitoso desde el cajero de un banco, sobre una
// cuenta de otro banco. bankID (dueño del cajero) y bancoID (emisor de la
// cuenta) son bancos distintos a propósito, para probar el caso real.
func TestAutorizarRetiroCajero_Exitosa(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)        // banco emisor, dueño de la cuenta
	bankID := "CAJERO-" + idAleatorio(t) // banco dueño del cajero
	handler := AutorizarRetiroCajero(db, bancoID)

	crearCuentaDirecta(t, bancoID, "CTA1", 2_000_000, "1234")

	req := autorizacionCajeroRequest{
		OperacionID:  uuidDePrueba(t),
		BancoCuenta:  bancoID,
		NumeroCuenta: "CTA1",
		Pin:          "1234",
		Importe:      500_000,
		Moneda:       "PYG",
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/cajero/autorizaciones", req, nil, map[string]string{"bankID": bankID})
	esperarStatus(t, w, http.StatusOK)

	resp := decodificarAutorizacion(t, w)
	if resp.Estado != "AUTORIZADO" {
		t.Fatalf("esperaba AUTORIZADO, dio %q (motivo: %q)", resp.Estado, resp.MotivoRechazo)
	}
	if saldo := saldoActual(t, bancoID, "CTA1"); saldo != 1_500_000 {
		t.Fatalf("saldo debería quedar en 1500000, quedó en %d", saldo)
	}
}

// Caso 16 (doc 04): retiro rechazado por PIN incorrecto.
func TestAutorizarRetiroCajero_PinIncorrecto(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	bankID := "CAJERO-" + idAleatorio(t)
	handler := AutorizarRetiroCajero(db, bancoID)

	crearCuentaDirecta(t, bancoID, "CTA1", 2_000_000, "1234")

	req := autorizacionCajeroRequest{
		OperacionID:  uuidDePrueba(t),
		BancoCuenta:  bancoID,
		NumeroCuenta: "CTA1",
		Pin:          "0000", // incorrecto
		Importe:      500_000,
		Moneda:       "PYG",
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/cajero/autorizaciones", req, nil, map[string]string{"bankID": bankID})
	esperarStatus(t, w, http.StatusOK) // el rechazo va envuelto en 200 OK, doc 02 §6

	resp := decodificarAutorizacion(t, w)
	if resp.Estado != "RECHAZADO" {
		t.Fatalf("esperaba RECHAZADO, dio %q", resp.Estado)
	}
	if resp.MotivoRechazo != ErrPinIncorrecto {
		t.Fatalf("esperaba motivo_rechazo=%q, dio %q", ErrPinIncorrecto, resp.MotivoRechazo)
	}
	if saldo := saldoActual(t, bancoID, "CTA1"); saldo != 2_000_000 {
		t.Fatalf("un PIN incorrecto no debería tocar el saldo, quedó en %d", saldo)
	}
}

// Caso 17 (doc 04): retiro rechazado por superar el límite de retiro
// (G. 1.500.000).
func TestAutorizarRetiroCajero_ExcedeLimiteDeRetiro(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	bankID := "CAJERO-" + idAleatorio(t)
	handler := AutorizarRetiroCajero(db, bancoID)

	crearCuentaDirecta(t, bancoID, "CTA1", 10_000_000, "1234")

	req := autorizacionCajeroRequest{
		OperacionID:  uuidDePrueba(t),
		BancoCuenta:  bancoID,
		NumeroCuenta: "CTA1",
		Pin:          "1234",
		Importe:      LimiteMaximoRetiroCajero + 1,
		Moneda:       "PYG",
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/cajero/autorizaciones", req, nil, map[string]string{"bankID": bankID})
	esperarStatus(t, w, http.StatusOK)

	resp := decodificarAutorizacion(t, w)
	if resp.Estado != "RECHAZADO" {
		t.Fatalf("esperaba RECHAZADO, dio %q", resp.Estado)
	}
	if resp.MotivoRechazo != ErrLimiteExcedido {
		t.Fatalf("esperaba motivo_rechazo=%q, dio %q", ErrLimiteExcedido, resp.MotivoRechazo)
	}
	if saldo := saldoActual(t, bancoID, "CTA1"); saldo != 10_000_000 {
		t.Fatalf("un rechazo no debería tocar el saldo, quedó en %d", saldo)
	}
}

// Caso 18 (doc 04): retiro rechazado por límite diario combinado ya
// alcanzado, tras una transferencia interna previa el mismo día. Ejercita
// el mismo totalSalidoHoy que comparten transferencias.go y cajero.go.
func TestAutorizarRetiroCajero_ExcedeLimiteDiarioCombinado(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	bankID := "CAJERO-" + idAleatorio(t)
	cajeroHandler := AutorizarRetiroCajero(db, bancoID)
	transferenciaHandler := TransferenciaInterna(db, bancoID)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, bancoID, "CTA1", 20_000_000, "1234", usuarioID)
	crearCuentaDirecta(t, bancoID, "CTA2", 0, "")

	// Transferencia interna previa: consume 9.500.000 del límite diario
	// combinado de CTA1.
	transferencia := transferenciaInternaRequest{
		NumeroCuentaOrigen:  "CTA1",
		NumeroCuentaDestino: "CTA2",
		Importe:             9_500_000,
		IdempotencyKey:      uuidDePrueba(t),
	}
	wt, _ := ejecutarJSON(t, transferenciaHandler, http.MethodPost, "/transferencias/internas", transferencia, nil, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, wt, http.StatusCreated)

	// El retiro, aislado, está dentro del límite por operación (1.000.000
	// < 1.500.000), pero combinado con la transferencia anterior
	// (9.500.000 + 1.000.000 = 10.500.000) supera el límite diario
	// combinado de 10.000.000.
	req := autorizacionCajeroRequest{
		OperacionID:  uuidDePrueba(t),
		BancoCuenta:  bancoID,
		NumeroCuenta: "CTA1",
		Pin:          "1234",
		Importe:      1_000_000,
		Moneda:       "PYG",
	}
	w, _ := ejecutarJSON(t, cajeroHandler, http.MethodPost, "/cajero/autorizaciones", req, nil, map[string]string{"bankID": bankID})
	esperarStatus(t, w, http.StatusOK)

	resp := decodificarAutorizacion(t, w)
	if resp.Estado != "RECHAZADO" {
		t.Fatalf("esperaba RECHAZADO por límite diario, dio %q", resp.Estado)
	}
	if resp.MotivoRechazo != ErrLimiteExcedido {
		t.Fatalf("esperaba motivo_rechazo=%q, dio %q", ErrLimiteExcedido, resp.MotivoRechazo)
	}

	// Solo la transferencia debería haber afectado el saldo.
	if saldo := saldoActual(t, bancoID, "CTA1"); saldo != 20_000_000-9_500_000 {
		t.Fatalf("el retiro rechazado no debería tocar el saldo, quedó en %d", saldo)
	}
}

// Caso 19 (doc 04): reintento de un retiro con el mismo operacion_id tras
// timeout simulado: no se debita dos veces.
func TestAutorizarRetiroCajero_ReintentoMismoOperacionID(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	bankID := "CAJERO-" + idAleatorio(t)
	handler := AutorizarRetiroCajero(db, bancoID)

	crearCuentaDirecta(t, bancoID, "CTA1", 2_000_000, "1234")

	req := autorizacionCajeroRequest{
		OperacionID:  uuidDePrueba(t),
		BancoCuenta:  bancoID,
		NumeroCuenta: "CTA1",
		Pin:          "1234",
		Importe:      500_000,
		Moneda:       "PYG",
	}

	// Primer intento: el cajero nunca recibió la respuesta (timeout
	// simulado) y reintenta con el mismo operacion_id.
	w1, _ := ejecutarJSON(t, handler, http.MethodPost, "/cajero/autorizaciones", req, nil, map[string]string{"bankID": bankID})
	esperarStatus(t, w1, http.StatusOK)
	resp1 := decodificarAutorizacion(t, w1)
	if resp1.Estado != "AUTORIZADO" {
		t.Fatalf("esperaba AUTORIZADO en el primer intento, dio %q", resp1.Estado)
	}

	w2, _ := ejecutarJSON(t, handler, http.MethodPost, "/cajero/autorizaciones", req, nil, map[string]string{"bankID": bankID})
	esperarStatus(t, w2, http.StatusOK)
	resp2 := decodificarAutorizacion(t, w2)
	if resp2.Estado != "AUTORIZADO" {
		t.Fatalf("esperaba AUTORIZADO en el reintento, dio %q", resp2.Estado)
	}

	if saldo := saldoActual(t, bancoID, "CTA1"); saldo != 1_500_000 {
		t.Fatalf("el reintento no debería debitar una segunda vez, saldo quedó en %d", saldo)
	}
}

// ConsultarAutorizacionCajero (doc 03 §3): el cajero puede consultar el
// estado de una autorización que ya se resolvió, usando el mismo
// operacion_id.
func TestConsultarAutorizacionCajero_DespuesDeAutorizar(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	bankID := "CAJERO-" + idAleatorio(t)
	autorizar := AutorizarRetiroCajero(db, bancoID)
	consultar := ConsultarAutorizacionCajero(db, bancoID)

	crearCuentaDirecta(t, bancoID, "CTA1", 2_000_000, "1234")
	operacionID := uuidDePrueba(t)

	req := autorizacionCajeroRequest{
		OperacionID:  operacionID,
		BancoCuenta:  bancoID,
		NumeroCuenta: "CTA1",
		Pin:          "1234",
		Importe:      500_000,
		Moneda:       "PYG",
	}
	w1, _ := ejecutarJSON(t, autorizar, http.MethodPost, "/cajero/autorizaciones", req, nil, map[string]string{"bankID": bankID})
	esperarStatus(t, w1, http.StatusOK)

	params := gin.Params{{Key: "operacion_id", Value: operacionID}}
	w2, _ := ejecutarJSON(t, consultar, http.MethodGet, "/cajero/autorizaciones/"+operacionID, nil, params, map[string]string{"bankID": bankID})
	esperarStatus(t, w2, http.StatusOK)

	resp := decodificarAutorizacion(t, w2)
	if resp.Estado != "AUTORIZADO" {
		t.Fatalf("esperaba consultar AUTORIZADO, dio %q", resp.Estado)
	}
}

func TestConsultarAutorizacionCajero_NoEncontrada(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	consultar := ConsultarAutorizacionCajero(db, bancoID)

	params := gin.Params{{Key: "operacion_id", Value: uuidDePrueba(t)}}
	w, _ := ejecutarJSON(t, consultar, http.MethodGet, "/cajero/autorizaciones/no-existe", nil, params, map[string]string{"bankID": "CAJERO-X"})

	esperarErrorContrato(t, w, http.StatusNotFound, ErrNoEncontrado)
}

// Análogo al caso 13 (doc 04) para el cajero: mismo operacion_id, importe
// distinto -> CONFLICTO_IDEMPOTENCIA, sin aplicar ningún efecto económico
// de la segunda solicitud. Antes del fix de esta sesión, el fast path
// ignoraba el cambio de importe y devolvía el resultado guardado.
func TestAutorizarRetiroCajero_ConflictoIdempotencia(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	bankID := "CAJERO-" + idAleatorio(t)
	handler := AutorizarRetiroCajero(db, bancoID)

	crearCuentaDirecta(t, bancoID, "CTA1", 2_000_000, "1234")

	operacionID := uuidDePrueba(t)

	primera := autorizacionCajeroRequest{
		OperacionID:  operacionID,
		BancoCuenta:  bancoID,
		NumeroCuenta: "CTA1",
		Pin:          "1234",
		Importe:      500_000,
		Moneda:       "PYG",
	}
	w1, _ := ejecutarJSON(t, handler, http.MethodPost, "/cajero/autorizaciones", primera, nil, map[string]string{"bankID": bankID})
	esperarStatus(t, w1, http.StatusOK)

	segunda := primera
	segunda.Importe = 1_999_999 // mismo operacion_id, importe bien distinto

	w2, _ := ejecutarJSON(t, handler, http.MethodPost, "/cajero/autorizaciones", segunda, nil, map[string]string{"bankID": bankID})
	esperarErrorContrato(t, w2, http.StatusConflict, ErrConflictoIdempotencia)

	if saldo := saldoActual(t, bancoID, "CTA1"); saldo != 1_500_000 {
		t.Fatalf("el conflicto de idempotencia no debería aplicar ningún efecto económico, saldo quedó en %d", saldo)
	}
}
