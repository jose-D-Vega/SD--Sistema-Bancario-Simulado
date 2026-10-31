package handlers

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"

	"github.com/sudameris/backend/internal/models"
)

type crearCuentaRequest struct {
	NumeroCuenta string `json:"numero_cuenta" binding:"required"`
	Titular      string `json:"titular" binding:"required"`
	SaldoInicial int64  `json:"saldo_inicial"`
	// Pin es opcional al crear; sin PIN la cuenta no puede operar en el
	// cajero (doc 03). 4 dígitos, como cualquier cajero real.
	Pin string `json:"pin" binding:"omitempty,len=4,numeric"`
}

func CrearCuenta(dbConn *sqlx.DB, bancoID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req crearCuentaRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.SaldoInicial < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "saldo_inicial no puede ser negativo"})
			return
		}

		var pinHash *string
		if req.Pin != "" {
			hash, err := bcrypt.GenerateFromPassword([]byte(req.Pin), bcrypt.DefaultCost)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "error procesando el pin"})
				return
			}
			s := string(hash)
			pinHash = &s
		}

		var cuenta models.Cuenta
		err := dbConn.Get(&cuenta, `
			INSERT INTO cuentas (banco_id, numero_cuenta, titular, saldo, pin_hash)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, banco_id, numero_cuenta, titular, saldo, estado, creado_en
		`, bancoID, req.NumeroCuenta, req.Titular, req.SaldoInicial, pinHash)

		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				c.JSON(http.StatusConflict, gin.H{"error": "ya existe una cuenta con ese número"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error creando la cuenta"})
			return
		}

		c.JSON(http.StatusCreated, cuenta)
	}
}

type cambiarEstadoRequest struct {
	Estado string `json:"estado" binding:"required"`
}

var estadosValidos = map[string]bool{
	models.EstadoActiva:    true,
	models.EstadoBloqueada: true,
	models.EstadoCerrada:   true,
}

// CambiarEstadoCuenta permite bloquear, reactivar o cerrar una cuenta
// propia (doc 01, sección 3). Es una operación interna del banco, no
// forma parte del contrato entre bancos, así que no requiere firma.
func CambiarEstadoCuenta(dbConn *sqlx.DB, bancoID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		numeroCuenta := c.Param("numero_cuenta")

		var req cambiarEstadoRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if !estadosValidos[req.Estado] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "estado inválido: " + req.Estado})
			return
		}

		var cuenta models.Cuenta
		err := dbConn.Get(&cuenta, `
			UPDATE cuentas SET estado = $1
			WHERE banco_id = $2 AND numero_cuenta = $3
			RETURNING id, banco_id, numero_cuenta, titular, saldo, estado, creado_en
		`, req.Estado, bancoID, numeroCuenta)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "cuenta no encontrada"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error actualizando la cuenta"})
			return
		}

		c.JSON(http.StatusOK, cuenta)
	}
}

func ObtenerCuenta(dbConn *sqlx.DB, bancoID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		numeroCuenta := c.Param("numero_cuenta")

		var cuenta models.Cuenta
		err := dbConn.Get(&cuenta, `
			SELECT id, banco_id, numero_cuenta, titular, saldo, estado, creado_en
			FROM cuentas
			WHERE banco_id = $1 AND numero_cuenta = $2
		`, bancoID, numeroCuenta)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "cuenta no encontrada"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error consultando la cuenta"})
			return
		}

		c.JSON(http.StatusOK, cuenta)
	}
}
