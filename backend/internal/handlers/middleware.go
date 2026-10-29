package handlers

import (
	"bytes"
	"crypto/ed25519"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sudameris/backend/internal/crypto"
)

// VerifyBankSignature verifica el header X-Signature contra la clave
// pública del banco indicado en X-Bank-Id (publicKeys la arma el BCP
// simulado). TODO: sumar validación de X-Timestamp y X-Idempotency-Key.
func VerifyBankSignature(publicKeys map[string]ed25519.PublicKey) gin.HandlerFunc {
	return func(c *gin.Context) {
		bankID := c.GetHeader("X-Bank-Id")
		signature := c.GetHeader("X-Signature")

		if bankID == "" || signature == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "faltan headers X-Bank-Id y/o X-Signature",
			})
			return
		}

		pubKey, ok := publicKeys[bankID]
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "banco_id desconocido: " + bankID,
			})
			return
		}

		// c.Request.Body solo se puede leer una vez: lo leemos acá para
		// verificar la firma y lo reponemos para el siguiente handler.
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "no se pudo leer el body",
			})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		valid, err := crypto.Verify(pubKey, bodyBytes, signature)
		if err != nil || !valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "firma inválida",
			})
			return
		}

		c.Next()
	}
}
