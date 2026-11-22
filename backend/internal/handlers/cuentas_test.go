package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/sudameris/backend/internal/models"
)

func decodificarCuenta(t *testing.T, w *httptest.ResponseRecorder) models.Cuenta {
	t.Helper()
	var cuenta models.Cuenta
	if err := json.Unmarshal(w.Body.Bytes(), &cuenta); err != nil {
		t.Fatalf("la respuesta no es una cuenta válida: %v\nbody: %s", err, w.Body.String())
	}
	return cuenta
}

func TestCrearCuenta_Exitosa(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := CrearCuenta(db, bancoID)

	req := crearCuentaRequest{
		NumeroCuenta: "CTA-" + idAleatorio(t),
		Titular:      "Titular de Prueba",
		SaldoInicial: 50_000,
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/cuentas", req, nil, nil)
	esperarStatus(t, w, http.StatusCreated)

	cuenta := decodificarCuenta(t, w)
	if cuenta.Saldo != 50_000 {
		t.Fatalf("esperaba saldo 50000, dio %d", cuenta.Saldo)
	}
	if cuenta.Estado != models.EstadoActiva {
		t.Fatalf("una cuenta nueva debería estar ACTIVA, dio %q", cuenta.Estado)
	}
	if cuenta.Tipo != models.TipoCuentaCorriente {
		t.Fatalf("sin tipo explícito debería caer a CUENTA_CORRIENTE, dio %q", cuenta.Tipo)
	}
}

func TestCrearCuenta_RechazaSaldoNegativo(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := CrearCuenta(db, bancoID)

	req := crearCuentaRequest{
		NumeroCuenta: "CTA-" + idAleatorio(t),
		Titular:      "Titular de Prueba",
		SaldoInicial: -1,
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/cuentas", req, nil, nil)
	esperarStatus(t, w, http.StatusBadRequest)
}

func TestCrearCuenta_RechazaTipoInvalido(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := CrearCuenta(db, bancoID)

	req := crearCuentaRequest{
		NumeroCuenta: "CTA-" + idAleatorio(t),
		Titular:      "Titular de Prueba",
		Tipo:         "TIPO_QUE_NO_EXISTE",
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/cuentas", req, nil, nil)
	esperarStatus(t, w, http.StatusBadRequest)
}

func TestCrearCuenta_RechazaNumeroDuplicado(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := CrearCuenta(db, bancoID)

	numero := "CTA-" + idAleatorio(t)
	crearCuentaDirecta(t, bancoID, numero, 0, "")

	req := crearCuentaRequest{NumeroCuenta: numero, Titular: "Otro Titular"}
	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/cuentas", req, nil, nil)
	esperarStatus(t, w, http.StatusConflict)
}

func TestCambiarEstadoCuenta_Exitoso(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := CambiarEstadoCuenta(db, bancoID)

	numero := "CTA-" + idAleatorio(t)
	crearCuentaDirecta(t, bancoID, numero, 0, "")

	params := gin.Params{{Key: "numero_cuenta", Value: numero}}
	req := cambiarEstadoRequest{Estado: models.EstadoBloqueada}
	w, _ := ejecutarJSON(t, handler, http.MethodPatch, "/cuentas/"+numero+"/estado", req, params, nil)
	esperarStatus(t, w, http.StatusOK)

	cuenta := decodificarCuenta(t, w)
	if cuenta.Estado != models.EstadoBloqueada {
		t.Fatalf("esperaba BLOQUEADA, dio %q", cuenta.Estado)
	}
}

func TestCambiarEstadoCuenta_RechazaEstadoInvalido(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := CambiarEstadoCuenta(db, bancoID)

	numero := "CTA-" + idAleatorio(t)
	crearCuentaDirecta(t, bancoID, numero, 0, "")

	params := gin.Params{{Key: "numero_cuenta", Value: numero}}
	req := cambiarEstadoRequest{Estado: "ESTADO_INEXISTENTE"}
	w, _ := ejecutarJSON(t, handler, http.MethodPatch, "/cuentas/"+numero+"/estado", req, params, nil)
	esperarStatus(t, w, http.StatusBadRequest)
}

func TestCambiarEstadoCuenta_NoEncontrada(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := CambiarEstadoCuenta(db, bancoID)

	params := gin.Params{{Key: "numero_cuenta", Value: "NO-EXISTE-" + idAleatorio(t)}}
	req := cambiarEstadoRequest{Estado: models.EstadoBloqueada}
	w, _ := ejecutarJSON(t, handler, http.MethodPatch, "/cuentas/x/estado", req, params, nil)
	esperarStatus(t, w, http.StatusNotFound)
}

func TestObtenerCuenta_Exitosa(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := ObtenerCuenta(db, bancoID)

	numero := "CTA-" + idAleatorio(t)
	crearCuentaDirecta(t, bancoID, numero, 123_000, "")

	params := gin.Params{{Key: "numero_cuenta", Value: numero}}
	w, _ := ejecutarJSON(t, handler, http.MethodGet, "/cuentas/"+numero, nil, params, nil)
	esperarStatus(t, w, http.StatusOK)

	cuenta := decodificarCuenta(t, w)
	if cuenta.Saldo != 123_000 {
		t.Fatalf("esperaba saldo 123000, dio %d", cuenta.Saldo)
	}
}

// ObtenerCuenta es parte del contrato del BCP (no de banca personal): el
// error tiene que venir en el formato del catálogo del doc 02, no un JSON
// genérico {"error": "..."}.
func TestObtenerCuenta_NoExisteUsaElCatalogoDeErroresDelContrato(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := ObtenerCuenta(db, bancoID)

	params := gin.Params{{Key: "numero_cuenta", Value: "NO-EXISTE-" + idAleatorio(t)}}
	w, _ := ejecutarJSON(t, handler, http.MethodGet, "/cuentas/x", nil, params, nil)
	esperarErrorContrato(t, w, http.StatusNotFound, ErrCuentaNoExiste)
}
