package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
)

func generarPar(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generando par de claves: %v", err)
	}
	return pub, priv
}

func TestSignVerify_RoundTrip(t *testing.T) {
	_, priv := generarPar(t)
	pub := priv.Public().(ed25519.PublicKey)

	payload := CanonicalString("POST", "/interbancario/creditos", "BCO1",
		"3fa85f64-5717-4562-b3fc-2c963f66afa6", "2026-10-26T14:32:10Z",
		[]byte(`{"transfer_id":"3fa85f64-5717-4562-b3fc-2c963f66afa6"}`))

	sig := Sign(priv, payload)

	ok, err := Verify(pub, payload, sig)
	if err != nil {
		t.Fatalf("Verify devolvió error inesperado: %v", err)
	}
	if !ok {
		t.Fatal("la firma debería verificar contra la clave pública correspondiente")
	}
}

func TestVerify_FallaConOtraClavePublica(t *testing.T) {
	_, priv := generarPar(t)
	otraPub, _ := generarPar(t)

	payload := CanonicalString("GET", "/cuentas/001-000001", "BCO2", "uuid-1", "2026-10-26T14:32:10Z", nil)
	sig := Sign(priv, payload)

	ok, err := Verify(otraPub, payload, sig)
	if err != nil {
		t.Fatalf("Verify no debería devolver error por una firma simplemente inválida: %v", err)
	}
	if ok {
		t.Fatal("la firma NO debería verificar contra una clave pública que no es la del firmante")
	}
}

func TestVerify_FallaSiElPayloadCambio(t *testing.T) {
	// Simula el caso real que previene este esquema: alguien intercepta
	// una solicitud válida y le cambia el importe antes de reenviarla.
	_, priv := generarPar(t)
	pub := priv.Public().(ed25519.PublicKey)

	original := CanonicalString("POST", "/interbancario/creditos", "BCO1", "uuid-1", "2026-10-26T14:32:10Z",
		[]byte(`{"importe":150000}`))
	sig := Sign(priv, original)

	modificado := CanonicalString("POST", "/interbancario/creditos", "BCO1", "uuid-1", "2026-10-26T14:32:10Z",
		[]byte(`{"importe":9999999}`))

	ok, err := Verify(pub, modificado, sig)
	if err != nil {
		t.Fatalf("Verify no debería devolver error por una firma simplemente inválida: %v", err)
	}
	if ok {
		t.Fatal("la firma no debería verificar si el payload cambió después de firmarlo")
	}
}

func TestVerify_FirmaInvalidaBase64(t *testing.T) {
	pub, _ := generarPar(t)
	_, err := Verify(pub, []byte("cualquier cosa"), "esto no es base64 válido!!")
	if err == nil {
		t.Fatal("se esperaba un error al decodificar una firma que no es base64 válido")
	}
}

func TestLoadPrivateKey_AceptaSeedDe32Bytes(t *testing.T) {
	// openssl genpkey + exportar en formato "raw" da la semilla de 32
	// bytes, no la clave expandida de 64 que usa internamente Go.
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	b64 := base64.StdEncoding.EncodeToString(seed)

	priv, err := LoadPrivateKey(b64)
	if err != nil {
		t.Fatalf("LoadPrivateKey con una semilla de 32 bytes no debería fallar: %v", err)
	}
	if len(priv) != ed25519.PrivateKeySize {
		t.Fatalf("la clave cargada debería expandirse a %d bytes, tiene %d", ed25519.PrivateKeySize, len(priv))
	}

	// Y que efectivamente sirva para firmar algo verificable con su
	// clave pública correspondiente.
	pub := priv.Public().(ed25519.PublicKey)
	sig := Sign(priv, []byte("mensaje de prueba"))
	ok, err := Verify(pub, []byte("mensaje de prueba"), sig)
	if err != nil || !ok {
		t.Fatalf("una clave cargada desde una semilla de 32 bytes debería poder firmar y verificar: ok=%v err=%v", ok, err)
	}
}

func TestLoadPrivateKey_AceptaClaveExpandidaDe64Bytes(t *testing.T) {
	_, priv := generarPar(t)
	b64 := base64.StdEncoding.EncodeToString(priv) // priv ya son 64 bytes

	cargada, err := LoadPrivateKey(b64)
	if err != nil {
		t.Fatalf("LoadPrivateKey con una clave de 64 bytes no debería fallar: %v", err)
	}
	if !cargada.Equal(priv) {
		t.Fatal("la clave cargada debería ser idéntica a la original")
	}
}

func TestLoadPrivateKey_RechazaTamañoInvalido(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte("muy corta"))
	if _, err := LoadPrivateKey(b64); err == nil {
		t.Fatal("se esperaba un error por un tamaño de clave privada inválido")
	}
}

func TestLoadPrivateKey_RechazaBase64Invalido(t *testing.T) {
	if _, err := LoadPrivateKey("no-es-base64-%%%"); err == nil {
		t.Fatal("se esperaba un error al decodificar base64 inválido")
	}
}

func TestLoadPublicKey_RoundTrip(t *testing.T) {
	pub, _ := generarPar(t)
	b64 := base64.StdEncoding.EncodeToString(pub)

	cargada, err := LoadPublicKey(b64)
	if err != nil {
		t.Fatalf("LoadPublicKey no debería fallar con una clave válida: %v", err)
	}
	if !cargada.Equal(pub) {
		t.Fatal("la clave pública cargada debería ser idéntica a la original")
	}
}

func TestLoadPublicKey_RechazaTamañoInvalido(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte("no son 32 bytes"))
	if _, err := LoadPublicKey(b64); err == nil {
		t.Fatal("se esperaba un error por un tamaño de clave pública inválido")
	}
}

func TestCanonicalString_FormatoExacto(t *testing.T) {
	// Fija el formato exacto que describe el doc 02 sección 1: seis
	// partes separadas por \n, en este orden. Si alguien reordena los
	// campos sin querer, este test tiene que romper.
	got := CanonicalString("POST", "/interbancario/creditos", "BCO1",
		"3fa85f64-5717-4562-b3fc-2c963f66afa6", "2026-10-26T14:32:10Z",
		[]byte(`{"transfer_id":"3fa85f64-5717-4562-b3fc-2c963f66afa6","cuenta_destino":"1002"}`))

	want := "POST\n/interbancario/creditos\nBCO1\n3fa85f64-5717-4562-b3fc-2c963f66afa6\n" +
		"2026-10-26T14:32:10Z\n{\"transfer_id\":\"3fa85f64-5717-4562-b3fc-2c963f66afa6\",\"cuenta_destino\":\"1002\"}"

	if string(got) != want {
		t.Fatalf("cadena canónica no coincide:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestCanonicalString_BodyVacio(t *testing.T) {
	// Caso de un GET sin body (doc 02 §1, §4 paso 4): el \n final antes
	// del body vacío tiene que seguir estando, porque el servidor arma
	// la cadena con 6 partes siempre, body vacío incluido. Los scripts
	// de prueba (test_consulta_cuenta.sh) documentan justamente este
	// detalle como fuente de un bug real con $(...) en bash.
	got := CanonicalString("GET", "/cuentas/001-000001", "BCO_TEST", "uuid-1", "2026-10-26T14:32:10Z", nil)
	want := "GET\n/cuentas/001-000001\nBCO_TEST\nuuid-1\n2026-10-26T14:32:10Z\n"

	if string(got) != want {
		t.Fatalf("cadena canónica con body vacío no coincide:\n  got:  %q\n  want: %q", got, want)
	}
	if !strings.HasSuffix(string(got), "\n") {
		t.Fatal("la cadena canónica con body vacío debe terminar en el separador, no truncarse")
	}
}
