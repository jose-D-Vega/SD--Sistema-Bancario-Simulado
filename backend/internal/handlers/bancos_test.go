package handlers

import (
	"net/http"
	"testing"

	"github.com/sudameris/backend/internal/config"
)

type listaBancosResponse struct {
	Bancos []bancoListado `json:"bancos"`
}

func TestListarBancos_IncluyePropioPrimeroConSuNombreDelRegistro(t *testing.T) {
	registro := map[string]config.BancoRegistrado{
		"SUDAMERIS": {Nombre: "Banco Sudameris"},
		"BCO-B":     {Nombre: "Banco B"},
	}
	handler := ListarBancos("SUDAMERIS", registro)

	w, _ := ejecutarJSON(t, handler, http.MethodGet, "/bancos", nil, nil, nil)
	esperarStatus(t, w, http.StatusOK)

	resp := decodificarJSON[listaBancosResponse](t, w)
	if len(resp.Bancos) != 2 {
		t.Fatalf("esperaba 2 bancos en la respuesta, dio %d", len(resp.Bancos))
	}
	if resp.Bancos[0].BancoID != "SUDAMERIS" || resp.Bancos[0].Nombre != "Banco Sudameris" {
		t.Fatalf("el banco propio debería ir primero y con el nombre del registro, dio %+v", resp.Bancos[0])
	}
}

// Si el registro no tiene una entrada para este mismo banco, igual se
// lista usando el banco_id como nombre -- no debería dar un nombre vacío
// ni romper.
func TestListarBancos_UsaBancoIDComoNombreSiNoHayEntradaPropia(t *testing.T) {
	registro := map[string]config.BancoRegistrado{
		"BCO-B": {Nombre: "Banco B"},
	}
	handler := ListarBancos("SUDAMERIS", registro)

	w, _ := ejecutarJSON(t, handler, http.MethodGet, "/bancos", nil, nil, nil)
	esperarStatus(t, w, http.StatusOK)

	resp := decodificarJSON[listaBancosResponse](t, w)
	if resp.Bancos[0].BancoID != "SUDAMERIS" || resp.Bancos[0].Nombre != "SUDAMERIS" {
		t.Fatalf("esperaba banco_id y nombre = SUDAMERIS, dio %+v", resp.Bancos[0])
	}
}

// El resto del registro va ordenado por banco_id, para que la respuesta
// sea estable entre llamadas aunque el mapa de Go no garantice orden.
func TestListarBancos_OrdenaElRestoPorBancoID(t *testing.T) {
	registro := map[string]config.BancoRegistrado{
		"SUDAMERIS": {Nombre: "Banco Sudameris"},
		"BCO-D":     {Nombre: "Banco D"},
		"BCO-B":     {},
		"BCO-C":     {Nombre: "Banco C"},
	}
	handler := ListarBancos("SUDAMERIS", registro)

	w, _ := ejecutarJSON(t, handler, http.MethodGet, "/bancos", nil, nil, nil)
	esperarStatus(t, w, http.StatusOK)

	resp := decodificarJSON[listaBancosResponse](t, w)
	if len(resp.Bancos) != 4 {
		t.Fatalf("esperaba 4 bancos, dio %d", len(resp.Bancos))
	}
	ordenEsperado := []string{"SUDAMERIS", "BCO-B", "BCO-C", "BCO-D"}
	for i, id := range ordenEsperado {
		if resp.Bancos[i].BancoID != id {
			t.Fatalf("posición %d: esperaba %q, dio %q", i, id, resp.Bancos[i].BancoID)
		}
	}

	// BCO-B no tiene Nombre en el registro: tiene que caer al banco_id.
	if resp.Bancos[1].Nombre != "BCO-B" {
		t.Fatalf("esperaba que BCO-B sin nombre caiga a su banco_id como nombre, dio %q", resp.Bancos[1].Nombre)
	}
}

func TestListarBancos_RegistroVacioListaSoloElPropio(t *testing.T) {
	handler := ListarBancos("SUDAMERIS", map[string]config.BancoRegistrado{})

	w, _ := ejecutarJSON(t, handler, http.MethodGet, "/bancos", nil, nil, nil)
	esperarStatus(t, w, http.StatusOK)

	resp := decodificarJSON[listaBancosResponse](t, w)
	if len(resp.Bancos) != 1 {
		t.Fatalf("esperaba solo el banco propio, dio %d", len(resp.Bancos))
	}
}
