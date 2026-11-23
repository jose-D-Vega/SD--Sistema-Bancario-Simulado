package handlers

import (
	"bytes"
	"crypto/ed25519"
	"crypto/subtle"
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

// RequireAdminKey protege operaciones de back office del propio banco
// (alta y bloqueo/cierre de cuentas, ver cuentas.go) que no encajan ni en
// VerifyBankSignature (no son parte del contrato entre bancos: ningún
// otro banco debería poder crear o bloquear cuentas en el nuestro) ni en
// RequireSesion (son operaciones de antes de que exista un usuario dueño
// de la cuenta -- p. ej. el seed inicial, o una cuenta de comercio sin
// titular). Es deliberadamente un mecanismo simple -- un secreto
// compartido en un header, no una sesión ni una firma -- porque quien
// debe poder llamar a esto es el propio equipo operando su banco, nunca
// un cliente final ni otro banco.
//
// Como el backend tiene que estar expuesto en red para que el contrato
// interbancario funcione, estas rutas son alcanzables por cualquiera que
// sepa la URL del banco si no se protegen -- no es un escenario
// hipotético de producción, es el escenario real de la integración entre
// los 4 grupos.
//
// Si adminKey viene vacío (ADMIN_API_KEY no configurada en el entorno),
// el middleware bloquea TODA solicitud en vez de dejarla pasar: es
// fail-closed a propósito, para que olvidarse de configurar la variable
// no termine reabriendo el mismo hueco que esto viene a cerrar.
func RequireAdminKey(adminKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if adminKey == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "esta ruta requiere ADMIN_API_KEY configurada en el servidor",
			})
			return
		}

		recibida := c.GetHeader("X-Admin-Key")
		// subtle.ConstantTimeCompare solo compara en tiempo constante
		// cuando los dos slices miden igual (si no, devuelve 0 de
		// entrada); comparar la longitud antes es solo para saltear el
		// trabajo cuando obviamente no coincide, no hace falta que la
		// longitud del secreto sea confidencial.
		if len(recibida) != len(adminKey) || subtle.ConstantTimeCompare([]byte(recibida), []byte(adminKey)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "falta o es inválido el header X-Admin-Key",
			})
			return
		}

		c.Next()
	}
}
