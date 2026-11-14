package handlers

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sudameris/backend/internal/crypto"
)

// routerConMiddleware arma un router mínimo: VerifyBankSignature seguido
// de un handler que solo confirma que llegó y qué bankID quedó seteado
// en el contexto. Así se prueba el middleware aislado, sin acoplarse a
// ningún handler de negocio real.
func routerConMiddleware(publicKeys map[string]ed25519.PublicKey) *gin.Engine {
	r := gin.New()
	r.Use(VerifyBankSignature(publicKeys))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "bankID": c.GetString("bankID")})
	})
	return r
}

type firmante struct {
	bancoID string
	priv    ed25519.PrivateKey
	pub     ed25519.PublicKey
}

func nuevoFirmante(t *testing.T, bancoID string) firmante {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generando clave de prueba: %v", err)
	}
	return firmante{bancoID: bancoID, priv: priv, pub: pub}
}

// firmar arma y firma una solicitud reusando crypto.CanonicalString, la
// misma función que usa el cliente real (internal/interbanco.Cliente) —
// así el test no depende de una segunda implementación de la cadena
// canónica que podría divergir en silencio de la real.
func (f firmante) firmar(method, path, idempotencyKey, timestamp string, body []byte) string {
	canonical := crypto.CanonicalString(method, path, f.bancoID, idempotencyKey, timestamp, body)
	return crypto.Sign(f.priv, canonical)
}

func nuevoRequestFirmado(t *testing.T, f firmante, method, path string, body []byte, timestamp string) *http.Request {
	t.Helper()
	idempotencyKey := "idem-" + idAleatorio(t)
	signature := f.firmar(method, path, idempotencyKey, timestamp, body)

	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("X-Bank-Id", f.bancoID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Idempotency-Key", idempotencyKey)
	req.Header.Set("X-Signature", signature)
	return req
}

func TestVerifyBankSignature_SolicitudValidaPasa(t *testing.T) {
	f := nuevoFirmante(t, "BCO_TEST")
	router := routerConMiddleware(map[string]ed25519.PublicKey{f.bancoID: f.pub})

	timestamp := time.Now().UTC().Format(time.RFC3339)
	req := nuevoRequestFirmado(t, f, http.MethodPost, "/test", []byte(`{"x":1}`), timestamp)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	esperarStatus(t, w, http.StatusOK)

	var body struct {
		BankID string `json:"bankID"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("respuesta no es JSON válido: %v", err)
	}
	if body.BankID != f.bancoID {
		t.Fatalf("esperaba que el middleware setee bankID=%q en el contexto, quedó %q", f.bancoID, body.BankID)
	}
}

func TestVerifyBankSignature_FaltaUnHeader(t *testing.T) {
	f := nuevoFirmante(t, "BCO_TEST")
	router := routerConMiddleware(map[string]ed25519.PublicKey{f.bancoID: f.pub})

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-Bank-Id", f.bancoID)
	// Sin X-Timestamp, X-Idempotency-Key ni X-Signature.

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	esperarErrorContrato(t, w, http.StatusBadRequest, ErrSolicitudInvalida)
}

func TestVerifyBankSignature_BancoNoReconocido(t *testing.T) {
	f := nuevoFirmante(t, "BCO_DESCONOCIDO")
	// El registro solo conoce a BCO_TEST, no a BCO_DESCONOCIDO.
	router := routerConMiddleware(map[string]ed25519.PublicKey{"BCO_TEST": f.pub})

	timestamp := time.Now().UTC().Format(time.RFC3339)
	req := nuevoRequestFirmado(t, f, http.MethodPost, "/test", []byte(`{}`), timestamp)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	esperarErrorContrato(t, w, http.StatusBadRequest, ErrBancoNoReconocido)
}

func TestVerifyBankSignature_FirmaInvalidaSiElBodyCambioDespues(t *testing.T) {
	// Simula el caso real que previene este esquema: alguien intercepta
	// una solicitud válida y le cambia el importe antes de que llegue.
	f := nuevoFirmante(t, "BCO_TEST")
	router := routerConMiddleware(map[string]ed25519.PublicKey{f.bancoID: f.pub})

	timestamp := time.Now().UTC().Format(time.RFC3339)
	req := nuevoRequestFirmado(t, f, http.MethodPost, "/test", []byte(`{"importe":100}`), timestamp)
	req.Body = io.NopCloser(bytes.NewReader([]byte(`{"importe":999999}`)))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	esperarErrorContrato(t, w, http.StatusUnauthorized, ErrFirmaInvalida)
}

func TestVerifyBankSignature_FirmaDeOtroBanco(t *testing.T) {
	// BCO_TEST dice ser quien firma (X-Bank-Id), pero la firma en
	// realidad la hizo la clave privada de otro banco.
	legitimo := nuevoFirmante(t, "BCO_TEST")
	impostor := nuevoFirmante(t, "BCO_TEST")
	router := routerConMiddleware(map[string]ed25519.PublicKey{"BCO_TEST": legitimo.pub})

	timestamp := time.Now().UTC().Format(time.RFC3339)
	req := nuevoRequestFirmado(t, impostor, http.MethodPost, "/test", []byte(`{}`), timestamp)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	esperarErrorContrato(t, w, http.StatusUnauthorized, ErrFirmaInvalida)
}

func TestVerifyBankSignature_TimestampFueraDeRango(t *testing.T) {
	f := nuevoFirmante(t, "BCO_TEST")
	router := routerConMiddleware(map[string]ed25519.PublicKey{f.bancoID: f.pub})

	viejo := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)
	req := nuevoRequestFirmado(t, f, http.MethodPost, "/test", []byte(`{}`), viejo)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	esperarErrorContrato(t, w, http.StatusUnauthorized, ErrTimestampFueraRango)
}

func TestVerifyBankSignature_TimestampMalFormado(t *testing.T) {
	f := nuevoFirmante(t, "BCO_TEST")
	router := routerConMiddleware(map[string]ed25519.PublicKey{f.bancoID: f.pub})

	req := nuevoRequestFirmado(t, f, http.MethodPost, "/test", []byte(`{}`), "esto-no-es-una-fecha")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	esperarErrorContrato(t, w, http.StatusUnauthorized, ErrTimestampFueraRango)
}

func esperarErrorContrato(t *testing.T, w *httptest.ResponseRecorder, statusEsperado int, codigoEsperado string) {
	t.Helper()
	esperarStatus(t, w, statusEsperado)

	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("respuesta no es JSON válido: %v\nbody: %s", err, w.Body.String())
	}
	if body.Error != codigoEsperado {
		t.Fatalf("esperaba error %q, dio %q (body: %s)", codigoEsperado, body.Error, w.Body.String())
	}
}
