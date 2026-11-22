package handlers

import (
	"bytes"
	"crypto/ed25519"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/sudameris/backend/internal/crypto"
	"github.com/sudameris/backend/internal/models"
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

// RequireSesion implementa la autenticación de usuario final (login del
// frontend, capa de banca personal). Es un mecanismo totalmente distinto de
// VerifyBankSignature: acá no hay firma Ed25519 ni banco_id, hay un token de
// sesión opaco que el servidor generó en /usuarios/login o /usuarios/registro
// y que el cliente manda como `Authorization: Bearer <token>`.
func RequireSesion(dbConn *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extraerTokenSesion(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "falta el header Authorization: Bearer <token>"})
			return
		}

		var sesion models.Sesion
		err := dbConn.Get(&sesion, `SELECT * FROM sesiones WHERE token = $1`, token)
		if errors.Is(err, sql.ErrNoRows) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "sesión inválida"})
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "error verificando la sesión"})
			return
		}

		if time.Now().UTC().After(sesion.ExpiraEn) {
			// Limpieza oportunista: no es crítico si falla, el siguiente
			// intento con el mismo token va a volver a toparse con esto.
			_, _ = dbConn.Exec(`DELETE FROM sesiones WHERE token = $1`, token)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "la sesión expiró, iniciá sesión de nuevo"})
			return
		}

		c.Set("usuarioID", sesion.UsuarioID.String())
		c.Next()
	}
}

func extraerTokenSesion(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
}
