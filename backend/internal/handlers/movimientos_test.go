package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/sudameris/backend/internal/models"
)

// listaMovimientosResponse espeja el shape que arma ListarMovimientos. Al
// estar en el mismo paquete, podemos decodificar directo a
// movimientoResponse (no exportado) sin duplicar el tipo acá.
type listaMovimientosResponse struct {
	Movimientos     []movimientoResponse `json:"movimientos"`
	SiguienteCursor *string              `json:"siguiente_cursor"`
}

func decodificarMovimientos(t *testing.T, w *httptest.ResponseRecorder) listaMovimientosResponse {
	t.Helper()
	var resp listaMovimientosResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("la respuesta no es el JSON esperado: %v\nbody: %s", err, w.Body.String())
	}
	return resp
}

func TestListarMovimientos_RechazaSiNoEsDuenoDeLaCuenta(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := ListarMovimientos(db, bancoID)

	dueño := crearUsuarioDePrueba(t)
	otro := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, bancoID, "CTA1", 0, "", dueño)

	params := gin.Params{{Key: "numero_cuenta", Value: "CTA1"}}
	w, _ := ejecutarJSON(t, handler, http.MethodGet, "/cuentas/CTA1/movimientos", nil, params, map[string]string{"usuarioID": otro})
	esperarStatus(t, w, http.StatusNotFound)
}

// Caso feliz: un envío (SALIENTE, desde esta cuenta) y un crédito entrante
// ya acreditado (ENTRANTE) tienen que aparecer los dos, con la dirección
// calculada correctamente.
func TestListarMovimientos_IncluyeSalienteYEntranteCompletados(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := ListarMovimientos(db, bancoID)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, bancoID, "CTA1", 0, "", usuarioID)
	crearCuentaDirecta(t, bancoID, "CTA2", 0, "")

	saliente := crearOperacionDirecta(t, operacionDePrueba{
		Tipo: models.TipoTransferenciaInterna, BancoOrigen: bancoID, NumeroCuentaOrigen: "CTA1",
		BancoDestino: bancoID, NumeroCuentaDestino: "CTA2", Importe: 50_000, Estado: models.OperacionCompletada,
	})
	entrante := crearOperacionDirecta(t, operacionDePrueba{
		Tipo: models.TipoTransferenciaInterbancaria, BancoOrigen: "OTRO-BANCO", NumeroCuentaOrigen: "EXTERNA",
		BancoDestino: bancoID, NumeroCuentaDestino: "CTA1", Importe: 70_000, Estado: models.OperacionCompletada,
	})

	params := gin.Params{{Key: "numero_cuenta", Value: "CTA1"}}
	w, _ := ejecutarJSON(t, handler, http.MethodGet, "/cuentas/CTA1/movimientos", nil, params, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w, http.StatusOK)

	resp := decodificarMovimientos(t, w)
	if len(resp.Movimientos) != 2 {
		t.Fatalf("esperaba 2 movimientos, dio %d", len(resp.Movimientos))
	}

	direcciones := map[string]string{}
	for _, m := range resp.Movimientos {
		direcciones[m.ID.String()] = m.Direccion
	}
	if direcciones[saliente.ID.String()] != "SALIENTE" {
		t.Fatalf("la operación saliente debería tener dirección SALIENTE, dio %q", direcciones[saliente.ID.String()])
	}
	if direcciones[entrante.ID.String()] != "ENTRANTE" {
		t.Fatalf("la operación entrante debería tener dirección ENTRANTE, dio %q", direcciones[entrante.ID.String()])
	}
}

// Test de regresión del bug "ingreso fantasma" (ver
// fix-movimientos-ingreso-fantasma): un crédito entrante que terminó
// RECHAZADA no tiene que aparecer en el historial de la cuenta que lo
// iba a recibir -- esa plata nunca llegó.
func TestListarMovimientos_ExcluyeCreditoEntranteRechazado(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := ListarMovimientos(db, bancoID)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, bancoID, "CTA1", 0, "", usuarioID)

	crearOperacionDirecta(t, operacionDePrueba{
		Tipo: models.TipoTransferenciaInterbancaria, BancoOrigen: "OTRO-BANCO", NumeroCuentaOrigen: "EXTERNA",
		BancoDestino: bancoID, NumeroCuentaDestino: "CTA1", Importe: 70_000, Estado: models.OperacionRechazada,
	})

	params := gin.Params{{Key: "numero_cuenta", Value: "CTA1"}}
	w, _ := ejecutarJSON(t, handler, http.MethodGet, "/cuentas/CTA1/movimientos", nil, params, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w, http.StatusOK)

	resp := decodificarMovimientos(t, w)
	if len(resp.Movimientos) != 0 {
		t.Fatalf("un crédito entrante RECHAZADA no debería listarse (ingreso fantasma), dio %d movimiento(s)", len(resp.Movimientos))
	}
}

func TestListarMovimientos_FiltraPorTipo(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := ListarMovimientos(db, bancoID)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, bancoID, "CTA1", 0, "", usuarioID)
	crearCuentaDirecta(t, bancoID, "CTA2", 0, "")

	crearOperacionDirecta(t, operacionDePrueba{
		Tipo: models.TipoTransferenciaInterna, BancoOrigen: bancoID, NumeroCuentaOrigen: "CTA1",
		BancoDestino: bancoID, NumeroCuentaDestino: "CTA2", Importe: 10_000, Estado: models.OperacionCompletada,
	})
	crearOperacionDirecta(t, operacionDePrueba{
		Tipo: models.TipoRetiroCajero, BancoOrigen: "CAJERO-X", NumeroCuentaOrigen: "CTA1",
		BancoDestino: bancoID, NumeroCuentaDestino: "CTA1", Importe: 20_000, Estado: models.OperacionCompletada,
	})

	params := gin.Params{{Key: "numero_cuenta", Value: "CTA1"}}
	w, _ := ejecutarJSON(t, handler, http.MethodGet,
		"/cuentas/CTA1/movimientos?tipo=RETIRO_CAJERO", nil, params, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w, http.StatusOK)

	resp := decodificarMovimientos(t, w)
	if len(resp.Movimientos) != 1 || resp.Movimientos[0].Tipo != models.TipoRetiroCajero {
		t.Fatalf("esperaba 1 movimiento RETIRO_CAJERO, dio %+v", resp.Movimientos)
	}
}

func TestListarMovimientos_RespetaLimiteYDaCursorSiguiente(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := ListarMovimientos(db, bancoID)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, bancoID, "CTA1", 0, "", usuarioID)
	crearCuentaDirecta(t, bancoID, "CTA2", 0, "")

	for i := 0; i < 3; i++ {
		crearOperacionDirecta(t, operacionDePrueba{
			Tipo: models.TipoTransferenciaInterna, BancoOrigen: bancoID, NumeroCuentaOrigen: "CTA1",
			BancoDestino: bancoID, NumeroCuentaDestino: "CTA2", Importe: 1_000, Estado: models.OperacionCompletada,
		})
	}

	params := gin.Params{{Key: "numero_cuenta", Value: "CTA1"}}
	w, _ := ejecutarJSON(t, handler, http.MethodGet,
		"/cuentas/CTA1/movimientos?limite=2", nil, params, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w, http.StatusOK)

	resp := decodificarMovimientos(t, w)
	if len(resp.Movimientos) != 2 {
		t.Fatalf("esperaba 2 movimientos (limite=2), dio %d", len(resp.Movimientos))
	}
	if resp.SiguienteCursor == nil {
		t.Fatalf("esperaba un siguiente_cursor porque hay una 3ra operación, dio nil")
	}

	w2, _ := ejecutarJSON(t, handler, http.MethodGet,
		"/cuentas/CTA1/movimientos?limite=2&cursor="+*resp.SiguienteCursor, nil, params, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w2, http.StatusOK)

	resp2 := decodificarMovimientos(t, w2)
	if len(resp2.Movimientos) != 1 {
		t.Fatalf("esperaba 1 movimiento en la segunda página, dio %d", len(resp2.Movimientos))
	}
	if resp2.SiguienteCursor != nil {
		t.Fatalf("no debería haber una tercera página")
	}
}

func TestListarMovimientos_RechazaLimiteInvalido(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := ListarMovimientos(db, bancoID)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, bancoID, "CTA1", 0, "", usuarioID)

	params := gin.Params{{Key: "numero_cuenta", Value: "CTA1"}}
	w, _ := ejecutarJSON(t, handler, http.MethodGet,
		"/cuentas/CTA1/movimientos?limite=no-es-un-numero", nil, params, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w, http.StatusBadRequest)
}
