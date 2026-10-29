package handlers

import (
	"bytes"
	"crypto/ed25519"
	"io"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sudameris/backend/internal/crypto"
)

const ventanaTimestamp = 5 * time.Minute

// VerifyBankSignature implementa la verificación de firma del doc 02,
// sección 1: valida los headers, la ventana de X-Timestamp, y la firma
// Ed25519 contra la cadena canónica reconstruida a partir del request.
func VerifyBankSignature(publicKeys map[string]ed25519.PublicKey) gin.HandlerFunc {
	return func(c *gin.Context) {
		bankID := c.GetHeader("X-Bank-Id")
		timestamp := c.GetHeader("X-Timestamp")
		idempotencyKey := c.GetHeader("X-Idempotency-Key")
		signature := c.GetHeader("X-Signature")

		if bankID == "" || timestamp == "" || idempotencyKey == "" || signature == "" {
			responderErrorContrato(c, ErrSolicitudInvalida, "faltan headers obligatorios (X-Bank-Id, X-Timestamp, X-Idempotency-Key, X-Signature)")
			return
		}

		pubKey, ok := publicKeys[bankID]
		if !ok {
			responderErrorContrato(c, ErrBancoNoReconocido, "banco_id desconocido: "+bankID)
			return
		}

		ts, err := time.Parse(time.RFC3339, timestamp)
		if err != nil || time.Since(ts).Abs() > ventanaTimestamp {
			responderErrorContrato(c, ErrTimestampFueraRango, "X-Timestamp inválido o fuera de rango")
			return
		}

		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			responderErrorContrato(c, ErrSolicitudInvalida, "no se pudo leer el body")
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		canonical := crypto.CanonicalString(c.Request.Method, c.Request.URL.Path, bankID, idempotencyKey, timestamp, bodyBytes)
		valid, err := crypto.Verify(pubKey, canonical, signature)
		if err != nil || !valid {
			responderErrorContrato(c, ErrFirmaInvalida, "la firma no verifica contra la clave pública del banco declarado")
			return
		}

		c.Set("bankID", bankID)
		c.Set("idempotencyKey", idempotencyKey)
		c.Next()
	}
}
