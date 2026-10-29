package handlers

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/sudameris/backend/internal/models"
)

type transferenciaInternaRequest struct {
	NumeroCuentaOrigen  string `json:"numero_cuenta_origen" binding:"required"`
	NumeroCuentaDestino string `json:"numero_cuenta_destino" binding:"required"`
	Importe             int64  `json:"importe" binding:"required"`
	IdempotencyKey      string `json:"idempotency_key" binding:"required"`
}

func TransferenciaInterna(dbConn *sqlx.DB, bancoID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req transferenciaInternaRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Importe <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "importe debe ser mayor a 0"})
			return
		}
		if req.NumeroCuentaOrigen == req.NumeroCuentaDestino {
			c.JSON(http.StatusBadRequest, gin.H{"error": "la cuenta origen y destino no pueden ser la misma"})
			return
		}

		// Fast path: si ya procesamos esta idempotency_key antes, devolvemos
		// el resultado guardado en vez de reprocesar.
		var existente models.Operacion
		err := dbConn.Get(&existente, `
			SELECT * FROM operaciones
			WHERE banco_origen = $1 AND idempotency_key = $2 AND tipo = $3
		`, bancoID, req.IdempotencyKey, models.TipoTransferenciaInterna)
		if err == nil {
			responderOperacion(c, existente)
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error verificando idempotencia"})
			return
		}

		tx, err := dbConn.Beginx()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error iniciando transacción"})
			return
		}
		defer tx.Rollback()

		// Lockeamos ambas cuentas en un orden consistente (por numero_cuenta)
		// para evitar deadlocks si dos transferencias cruzadas corren a la vez.
		primera, segunda := req.NumeroCuentaOrigen, req.NumeroCuentaDestino
		if primera > segunda {
			primera, segunda = segunda, primera
		}

		cuentas := map[string]models.Cuenta{}
		for _, numero := range []string{primera, segunda} {
			var cuenta models.Cuenta
			err := tx.Get(&cuenta, `
				SELECT * FROM cuentas WHERE banco_id = $1 AND numero_cuenta = $2 FOR UPDATE
			`, bancoID, numero)
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "cuenta no encontrada: " + numero})
				return
			}
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "error consultando cuentas"})
				return
			}
			cuentas[numero] = cuenta
		}

		origen := cuentas[req.NumeroCuentaOrigen]
		destino := cuentas[req.NumeroCuentaDestino]

		motivoRechazo := ""
		switch {
		case origen.Estado != models.EstadoActiva:
			motivoRechazo = "cuenta origen no está activa"
		case destino.Estado != models.EstadoActiva:
			motivoRechazo = "cuenta destino no está activa"
		case origen.Saldo < req.Importe:
			motivoRechazo = "saldo insuficiente"
		}

		if motivoRechazo != "" {
			op, err := registrarOperacion(tx, bancoID, req, models.OperacionRechazada, &motivoRechazo)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "error registrando operación"})
				return
			}
			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "error confirmando operación"})
				return
			}
			responderOperacion(c, op)
			return
		}

		if _, err := tx.Exec(`UPDATE cuentas SET saldo = saldo - $1 WHERE id = $2`, req.Importe, origen.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error actualizando cuenta origen"})
			return
		}
		if _, err := tx.Exec(`UPDATE cuentas SET saldo = saldo + $1 WHERE id = $2`, req.Importe, destino.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error actualizando cuenta destino"})
			return
		}

		op, err := registrarOperacion(tx, bancoID, req, models.OperacionCompletada, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error registrando operación"})
			return
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error confirmando transferencia"})
			return
		}

		responderOperacion(c, op)
	}
}

func registrarOperacion(tx *sqlx.Tx, bancoID string, req transferenciaInternaRequest, estado string, motivoRechazo *string) (models.Operacion, error) {
	var op models.Operacion
	// $3 se reusa para banco_destino: en una transferencia interna el
	// banco origen y destino son el mismo.
	err := tx.Get(&op, `
		INSERT INTO operaciones (
			tipo, idempotency_key, banco_origen, numero_cuenta_origen,
			banco_destino, numero_cuenta_destino, importe, estado, motivo_rechazo
		)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8)
		RETURNING *
	`, models.TipoTransferenciaInterna, req.IdempotencyKey, bancoID, req.NumeroCuentaOrigen,
		req.NumeroCuentaDestino, req.Importe, estado, motivoRechazo)
	return op, err
}

func responderOperacion(c *gin.Context, op models.Operacion) {
	switch op.Estado {
	case models.OperacionCompletada:
		c.JSON(http.StatusCreated, op)
	case models.OperacionRechazada:
		c.JSON(http.StatusUnprocessableEntity, op)
	default:
		c.JSON(http.StatusOK, op)
	}
}
