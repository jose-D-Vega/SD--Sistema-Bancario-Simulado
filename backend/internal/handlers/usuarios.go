package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"

	"github.com/sudameris/backend/internal/models"
)

// Este archivo implementa la capa de "banca personal": usuarios, login y
// cuentas propias. Es una extensión propia de este banco sobre el mínimo
// obligatorio del reglamento (docs 01/02/03 no dicen nada sobre cómo cada
// banco autentica a sus propios clientes) — pensada para que el frontend
// (diseñado por el equipo como prototipo local) tenga con qué reemplazar su
// modelo en localStorage por datos reales.
//
// Dos cosas para no confundir con lo que ya existía:
//   - PinHash (en Cuenta) es la credencial para operar en el CAJERO (doc 03).
//   - PasswordHash (en Usuario) es la credencial para iniciar sesión en la
//     app. Son conceptos distintos, cada uno con su propio hash bcrypt.

// ---------------------------------------------------------------------
// Registro y login
// ---------------------------------------------------------------------

type registroUsuarioRequest struct {
	Username          string `json:"username" binding:"required"`
	Documento         string `json:"documento" binding:"required"`
	Nombre            string `json:"nombre" binding:"required"`
	Email             string `json:"email"`
	Telefono          string `json:"telefono"`
	Password          string `json:"password" binding:"required,min=8"`
	TipoCuentaInicial string `json:"tipo_cuenta_inicial"`
}

// RegistrarUsuario implementa POST /usuarios/registro: crea el usuario y su
// primera cuenta (en ACTIVA, saldo 0) en una sola transacción, y devuelve
// una sesión ya iniciada para no obligar a loguearse de nuevo.
func RegistrarUsuario(dbConn *sqlx.DB, bancoID string, duracionSesion time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req registroUsuarioRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		tipo := req.TipoCuentaInicial
		if tipo == "" {
			tipo = models.TipoCuentaCorriente
		}
		if tipo != models.TipoCuentaCorriente && tipo != models.TipoCajaAhorro {
			c.JSON(http.StatusBadRequest, gin.H{"error": "tipo_cuenta_inicial inválido: usá CUENTA_CORRIENTE o CAJA_AHORRO"})
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error procesando la contraseña"})
			return
		}

		tx, err := dbConn.Beginx()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error iniciando transacción"})
			return
		}
		defer tx.Rollback()

		var usuario models.Usuario
		err = tx.Get(&usuario, `
			INSERT INTO usuarios (username, documento, nombre, email, telefono, password_hash)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING *
		`, strings.ToLower(strings.TrimSpace(req.Username)), strings.TrimSpace(req.Documento),
			strings.TrimSpace(req.Nombre), nullIfVacio(req.Email), nullIfVacio(req.Telefono), string(hash))
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				c.JSON(http.StatusConflict, gin.H{"error": "el usuario o documento ya está registrado"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error creando el usuario"})
			return
		}

		// El número de cuenta se genera acá (el frontend no lo pide): se
		// reintenta un par de veces por si el aleatorio choca con uno
		// existente, algo muy poco probable con 9 dígitos pero no imposible.
		var cuenta models.Cuenta
		for intento := 0; intento < 5; intento++ {
			numeroCuenta := generarNumeroCuenta()
			err = tx.Get(&cuenta, `
				INSERT INTO cuentas (banco_id, numero_cuenta, titular, usuario_id, tipo)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING *
			`, bancoID, numeroCuenta, usuario.Nombre, usuario.ID, tipo)
			if err == nil {
				break
			}
			var pgErr *pgconn.PgError
			if !(errors.As(err, &pgErr) && pgErr.Code == "23505") {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "error creando la cuenta inicial"})
				return
			}
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo generar un número de cuenta disponible, reintentá"})
			return
		}

		sesion, err := crearSesion(tx, usuario.ID, duracionSesion)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error creando la sesión"})
			return
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error confirmando el registro"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"usuario":   usuario,
			"cuenta":    cuenta,
			"token":     sesion.Token,
			"expira_en": sesion.ExpiraEn,
		})
	}
}

type loginRequest struct {
	Usuario  string `json:"usuario" binding:"required"` // username o documento
	Password string `json:"password" binding:"required"`
}

// LoginUsuario implementa POST /usuarios/login.
func LoginUsuario(dbConn *sqlx.DB, duracionSesion time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		identificador := strings.ToLower(strings.TrimSpace(req.Usuario))

		var usuario models.Usuario
		err := dbConn.Get(&usuario, `
			SELECT * FROM usuarios WHERE username = $1 OR documento = $1
		`, identificador)
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "usuario o contraseña incorrectos"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error consultando el usuario"})
			return
		}

		if bcrypt.CompareHashAndPassword([]byte(usuario.PasswordHash), []byte(req.Password)) != nil {
			// Mismo mensaje que "no existe": no le decimos a quien intenta
			// loguearse cuál de los dos datos está mal.
			c.JSON(http.StatusUnauthorized, gin.H{"error": "usuario o contraseña incorrectos"})
			return
		}

		sesion, err := crearSesion(dbConn, usuario.ID, duracionSesion)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error creando la sesión"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"usuario":   usuario,
			"token":     sesion.Token,
			"expira_en": sesion.ExpiraEn,
		})
	}
}

// LogoutUsuario implementa POST /usuarios/logout (requiere sesión): borra el
// token actual. No invalida otras sesiones del mismo usuario en otros
// dispositivos — para eso ver CambiarMiPassword, que sí las borra todas.
func LogoutUsuario(dbConn *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extraerTokenSesion(c)
		if _, err := dbConn.Exec(`DELETE FROM sesiones WHERE token = $1`, token); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error cerrando la sesión"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ---------------------------------------------------------------------
// Perfil propio
// ---------------------------------------------------------------------

// ObtenerMiPerfil implementa GET /usuarios/me (requiere sesión).
func ObtenerMiPerfil(dbConn *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		usuarioID := c.GetString("usuarioID")

		var usuario models.Usuario
		if err := dbConn.Get(&usuario, `SELECT * FROM usuarios WHERE id = $1`, usuarioID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error consultando el perfil"})
			return
		}
		c.JSON(http.StatusOK, usuario)
	}
}

type actualizarPerfilRequest struct {
	Nombre   string `json:"nombre" binding:"required"`
	Email    string `json:"email"`
	Telefono string `json:"telefono"`
}

// ActualizarMiPerfil implementa PATCH /usuarios/me (requiere sesión). El
// username y el documento no se pueden cambiar acá — son el identificador
// de login, cambiarlos es un caso aparte que no pidió el frontend.
func ActualizarMiPerfil(dbConn *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		usuarioID := c.GetString("usuarioID")

		var req actualizarPerfilRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		nombre := strings.TrimSpace(req.Nombre)
		if nombre == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "nombre no puede estar vacío"})
			return
		}

		var usuario models.Usuario
		err := dbConn.Get(&usuario, `
			UPDATE usuarios SET nombre = $1, email = $2, telefono = $3
			WHERE id = $4
			RETURNING *
		`, nombre, nullIfVacio(req.Email), nullIfVacio(req.Telefono), usuarioID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error actualizando el perfil"})
			return
		}
		c.JSON(http.StatusOK, usuario)
	}
}

type cambiarPasswordUsuarioRequest struct {
	Actual string `json:"actual" binding:"required"`
	Nueva  string `json:"nueva" binding:"required,min=8"`
}

// CambiarMiPassword implementa PATCH /usuarios/me/password (requiere
// sesión). Al cambiar la contraseña se cierran todas las sesiones activas
// del usuario, incluida la que hizo este pedido: el cliente debe loguearse
// de nuevo con la contraseña nueva.
func CambiarMiPassword(dbConn *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		usuarioID := c.GetString("usuarioID")

		var req cambiarPasswordUsuarioRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var usuario models.Usuario
		if err := dbConn.Get(&usuario, `SELECT * FROM usuarios WHERE id = $1`, usuarioID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error consultando el usuario"})
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(usuario.PasswordHash), []byte(req.Actual)) != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "la contraseña actual no coincide"})
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Nueva), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error procesando la nueva contraseña"})
			return
		}

		tx, err := dbConn.Beginx()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error iniciando transacción"})
			return
		}
		defer tx.Rollback()

		if _, err := tx.Exec(`UPDATE usuarios SET password_hash = $1 WHERE id = $2`, string(hash), usuarioID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error actualizando la contraseña"})
			return
		}
		if _, err := tx.Exec(`DELETE FROM sesiones WHERE usuario_id = $1`, usuarioID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error cerrando las sesiones activas"})
			return
		}
		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error confirmando el cambio de contraseña"})
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// ---------------------------------------------------------------------
// Cuentas propias
// ---------------------------------------------------------------------

// ListarMisCuentas implementa GET /usuarios/me/cuentas (requiere sesión).
// Es la contraparte real del arreglo `accounts` que el prototipo traía
// hardcodeado: todas las cuentas del usuario logueado, sean cuales sean sus
// estados (una cuenta Bloqueada o Cerrada también debe listarse, el
// frontend ya sabe mostrarlas distinto).
func ListarMisCuentas(dbConn *sqlx.DB, bancoID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		usuarioID := c.GetString("usuarioID")

		cuentas := []models.Cuenta{}
		err := dbConn.Select(&cuentas, `
			SELECT * FROM cuentas WHERE banco_id = $1 AND usuario_id = $2 ORDER BY creado_en
		`, bancoID, usuarioID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error consultando las cuentas"})
			return
		}
		c.JSON(http.StatusOK, cuentas)
	}
}

type cambiarAliasRequest struct {
	Alias string `json:"alias" binding:"required"`
}

// aliasValido exige minúsculas, dígitos, '.', '_' o '-' — el mismo patrón
// que ya usa el prototipo del frontend para el alias de perfil.
var aliasValido = regexp.MustCompile(`^[a-z0-9._-]{3,30}$`)

// CambiarAliasCuenta implementa PATCH /cuentas/:numero_cuenta/alias
// (requiere sesión). El WHERE exige que la cuenta sea del usuario logueado:
// no alcanza con conocer el número de cuenta de otro para tocarle el alias.
func CambiarAliasCuenta(dbConn *sqlx.DB, bancoID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		usuarioID := c.GetString("usuarioID")
		numeroCuenta := c.Param("numero_cuenta")

		var req cambiarAliasRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		alias := strings.ToLower(strings.TrimSpace(req.Alias))
		if !aliasValido.MatchString(alias) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "alias inválido: usá 3 a 30 caracteres entre minúsculas, números, '.', '_' o '-'"})
			return
		}

		var cuenta models.Cuenta
		err := dbConn.Get(&cuenta, `
			UPDATE cuentas SET alias = $1
			WHERE banco_id = $2 AND numero_cuenta = $3 AND usuario_id = $4
			RETURNING *
		`, alias, bancoID, numeroCuenta, usuarioID)
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "la cuenta no existe o no te pertenece"})
			return
		}
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				c.JSON(http.StatusConflict, gin.H{"error": "ese alias ya está en uso"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error actualizando el alias"})
			return
		}
		c.JSON(http.StatusOK, cuenta)
	}
}

// ---------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------

// crearSesion genera un token de alta entropía y lo guarda. q acepta tanto
// *sqlx.DB como *sqlx.Tx (ver la interfaz getter en limites.go) para poder
// crear la sesión dentro de la misma transacción que el registro, o suelta
// en el login.
func crearSesion(q getter, usuarioID uuid.UUID, duracion time.Duration) (models.Sesion, error) {
	token, err := generarTokenSesion()
	if err != nil {
		return models.Sesion{}, err
	}

	var sesion models.Sesion
	err = q.Get(&sesion, `
		INSERT INTO sesiones (token, usuario_id, expira_en)
		VALUES ($1, $2, $3)
		RETURNING *
	`, token, usuarioID, time.Now().UTC().Add(duracion))
	return sesion, err
}

func generarTokenSesion() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generando token de sesión: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// generarNumeroCuenta arma un número de cuenta de 9 dígitos al azar. No usa
// math/rand (predecible) sino crypto/rand, igual que el resto del código
// que genera identificadores sensibles (ver generarTokenSesion acá mismo y
// Sign/Verify en internal/crypto).
func generarNumeroCuenta() string {
	var b [8]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read no falla en la práctica salvo un error grave del SO
	n := binary.BigEndian.Uint64(b[:]) % 1_000_000_000
	return fmt.Sprintf("%09d", n)
}

// nullIfVacio convierte un string vacío (o solo espacios) en nil, para
// columnas nullable como email/telefono.
func nullIfVacio(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
