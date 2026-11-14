package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

// clavePublicaDePrueba genera una clave pública Ed25519 válida en base64
// crudo (32 bytes), en el mismo formato que exige el doc 02 sección 1.
func clavePublicaDePrueba(t *testing.T) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generando clave de prueba: %v", err)
	}
	return base64.StdEncoding.EncodeToString(pub)
}

func escribirRegistro(t *testing.T, contenido string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "registro_bancos.json")
	if err := os.WriteFile(path, []byte(contenido), 0o644); err != nil {
		t.Fatalf("escribiendo registro de prueba: %v", err)
	}
	return path
}

func TestCargarRegistroBancos_PathVacio(t *testing.T) {
	result, err := CargarRegistroBancos("")
	if err != nil {
		t.Fatalf("path vacío no debería devolver error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("path vacío debería devolver un mapa vacío, tiene %d entradas", len(result))
	}
}

func TestCargarRegistroBancos_FormatoOficialCompleto(t *testing.T) {
	clave1 := clavePublicaDePrueba(t)
	clave2 := clavePublicaDePrueba(t)

	contenido := `{
		"version": 1,
		"fecha_actualizacion": "2026-10-28",
		"algoritmo_firma": "Ed25519",
		"bancos": [
			{"banco_id": "BCO1", "nombre": "Banco 1", "url_base": "http://bco1:8080", "clave_publica": "` + clave1 + `"},
			{"banco_id": "BCO2", "nombre": "Banco 2", "url_base": "http://bco2:8080", "clave_publica": "` + clave2 + `"}
		]
	}`

	path := escribirRegistro(t, contenido)
	result, err := CargarRegistroBancos(path)
	if err != nil {
		t.Fatalf("no debería fallar con un registro válido: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("se esperaban 2 bancos, se cargaron %d", len(result))
	}

	bco1, ok := result["BCO1"]
	if !ok {
		t.Fatal("falta BCO1 en el resultado")
	}
	if bco1.URL != "http://bco1:8080" {
		t.Fatalf("URL de BCO1 incorrecta: %q", bco1.URL)
	}
}

func TestCargarRegistroBancos_ObjetoVacio(t *testing.T) {
	// El caso del README: `echo '{}' > registro_bancos.json` cuando
	// todavía no hay registro. No debería fallar.
	path := escribirRegistro(t, `{}`)
	result, err := CargarRegistroBancos(path)
	if err != nil {
		t.Fatalf("un registro {} no debería fallar: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("un registro {} debería devolver un mapa vacío, tiene %d entradas", len(result))
	}
}

func TestCargarRegistroBancos_SalteaEntradasSinCompletar(t *testing.T) {
	// El template de 06_registro_bancos_v2.json que distribuye el BCP
	// trae placeholders tipo "<URL base de la API>" para los bancos que
	// todavía no confirmaron sus datos. Esas entradas se tienen que
	// saltear, no hacer fallar la carga de las demás.
	claveValida := clavePublicaDePrueba(t)

	contenido := `{
		"bancos": [
			{"banco_id": "BCO1", "nombre": "<nombre del grupo / banco 1>", "url_base": "<URL base de la API>", "clave_publica": "<clave pública en base64>"},
			{"banco_id": "BCO2", "nombre": "Banco 2", "url_base": "", "clave_publica": ""},
			{"banco_id": "BCO3", "nombre": "Banco 3", "url_base": "http://bco3:8080", "clave_publica": "` + claveValida + `"}
		]
	}`

	path := escribirRegistro(t, contenido)
	result, err := CargarRegistroBancos(path)
	if err != nil {
		t.Fatalf("entradas sin completar no deberían hacer fallar la carga: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("se esperaba 1 solo banco cargado (BCO3), se cargaron %d: %v", len(result), result)
	}
	if _, ok := result["BCO3"]; !ok {
		t.Fatal("BCO3 (el único completo) debería estar cargado")
	}
	if _, ok := result["BCO1"]; ok {
		t.Fatal("BCO1 tiene placeholders, no debería haberse cargado")
	}
}

func TestCargarRegistroBancos_ClavePublicaInvalidaDaError(t *testing.T) {
	// A diferencia de los placeholders (que se saltean), una clave que
	// parece real pero no es un Ed25519 válido es un error de verdad:
	// algo se completó mal y hay que avisar, no seguir en silencio.
	contenido := `{
		"bancos": [
			{"banco_id": "BCO1", "nombre": "Banco 1", "url_base": "http://bco1:8080", "clave_publica": "no-es-base64-valido-%%%"}
		]
	}`

	path := escribirRegistro(t, contenido)
	_, err := CargarRegistroBancos(path)
	if err == nil {
		t.Fatal("una clave pública inválida (no placeholder) debería devolver error")
	}
}

func TestCargarRegistroBancos_ArchivoInexistente(t *testing.T) {
	_, err := CargarRegistroBancos("/ruta/que/no/existe/registro.json")
	if err == nil {
		t.Fatal("un path inexistente debería devolver error")
	}
}

func TestCargarRegistroBancos_JSONMalformado(t *testing.T) {
	path := escribirRegistro(t, `{ esto no es json`)
	_, err := CargarRegistroBancos(path)
	if err == nil {
		t.Fatal("JSON malformado debería devolver error")
	}
}
