package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/sudameris/backend/internal/config"
	"github.com/sudameris/backend/internal/interbanco"
	"github.com/sudameris/backend/internal/models"
)

type transferenciaInterbancariaRequest struct {
	NumeroCuentaOrigen  string `json:"numero_cuenta_origen" binding:"required"`
	BancoDestino        string `json:"banco_destino" binding:"required"`
	NumeroCuentaDestino string `json:"numero_cuenta_destino" binding:"required"`
	Importe             int64  `json:"importe" binding:"required"`
	IdempotencyKey      string `json:"idempotency_key" binding:"required"`
}

const timeoutLlamadaBanco = 5 * time.Second

func TransferenciaInterbancaria(dbConn *sqlx.DB, bancoID string, cliente *interbanco.Cliente, registro map[string]config.BancoRegistrado) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req transferenciaInterbancariaRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Importe <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "importe debe ser mayor a 0"})
			return
		}
		destino, ok := registro[req.BancoDestino]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "banco_destino desconocido: " + req.BancoDestino})
			return
		}

		var existente models.Operacion
		err := dbConn.Get(&existente, `
			SELECT * FROM operaciones
			WHERE banco_origen = $1 AND idempotency_key = $2 AND tipo = $3
		`, bancoID, req.IdempotencyKey, models.TipoTransferenciaInterbancaria)
		if err == nil {
			responderOperacionInterbancaria(c, existente)
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error verificando idempotencia"})
			return
		}

		op, err := reservarYDebitar(dbConn, bancoID, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if op.Estado == models.OperacionRechazada {
			responderOperacionInterbancaria(c, op)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeoutLlamadaBanco)
		defer cancel()

		payload := interbanco.CreditoPayload{
			TransferID:    req.IdempotencyKey,
			CuentaDestino: req.NumeroCuentaDestino,
			BancoOrigen:   bancoID,
			CuentaOrigen:  req.NumeroCuentaOrigen,
			Importe:       req.Importe,
			Moneda:        "PYG",
		}
		resp, callErr := cliente.EnviarCredito(ctx, destino.URL, payload)
		op = resolverResultado(dbConn, op, resp, callErr, cliente, destino.URL)

		responderOperacionInterbancaria(c, op)
	}
}

// reservarYDebitar valida la cuenta origen y, si está todo bien, descuenta
// el importe ya mismo (reserva) antes de llamar al banco destino. Si la
// validación falla, registra el rechazo sin tocar el saldo.
func reservarYDebitar(dbConn *sqlx.DB, bancoID string, req transferenciaInterbancariaRequest) (models.Operacion, error) {
	tx, err := dbConn.Beginx()
	if err != nil {
		return models.Operacion{}, err
	}
	defer tx.Rollback()

	var origen models.Cuenta
	err = tx.Get(&origen, `
		SELECT * FROM cuentas WHERE banco_id = $1 AND numero_cuenta = $2 FOR UPDATE
	`, bancoID, req.NumeroCuentaOrigen)

	var motivoRechazo string
	switch {
	case errors.Is(err, sql.ErrNoRows):
		motivoRechazo = "cuenta origen no encontrada"
	case err != nil:
		return models.Operacion{}, err
	case origen.Estado != models.EstadoActiva:
		motivoRechazo = "cuenta origen no está activa"
	case origen.Saldo < req.Importe:
		motivoRechazo = "saldo insuficiente"
	}

	estado := models.OperacionReservada
	if motivoRechazo != "" {
		estado = models.OperacionRechazada
	} else {
		if _, err := tx.Exec(`UPDATE cuentas SET saldo = saldo - $1 WHERE id = $2`, req.Importe, origen.ID); err != nil {
			return models.Operacion{}, err
		}
	}

	var op models.Operacion
	var motivoPtr *string
	if motivoRechazo != "" {
		motivoPtr = &motivoRechazo
	}
	err = tx.Get(&op, `
		INSERT INTO operaciones (
			tipo, idempotency_key, banco_origen, numero_cuenta_origen,
			banco_destino, numero_cuenta_destino, importe, estado, motivo_rechazo
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING *
	`, models.TipoTransferenciaInterbancaria, req.IdempotencyKey, bancoID, req.NumeroCuentaOrigen,
		req.BancoDestino, req.NumeroCuentaDestino, req.Importe, estado, motivoPtr)
	if err != nil {
		return models.Operacion{}, err
	}

	return op, tx.Commit()
}

// resolverResultado interpreta la respuesta del banco destino (o su
// ausencia) y deja la operación en un estado definitivo cuando es
// posible. Si ni la llamada directa ni la consulta de verificación
// responden, la operación queda en PENDIENTE para revisión manual: el
// doc 02 es explícito en que ante un timeout no hay que asumir nada.
func resolverResultado(dbConn *sqlx.DB, op models.Operacion, resp *interbanco.RespuestaCredito, callErr error, cliente *interbanco.Cliente, baseURL string) models.Operacion {
	if callErr == nil && resp.Estado == "ACREDITADO" {
		return actualizarEstado(dbConn, op, models.OperacionCompletada, nil)
	}

	if ibErr, ok := callErr.(*interbanco.ErrorInterbancario); ok {
		motivo := ibErr.Mensaje
		revertirReserva(dbConn, op)
		return actualizarEstado(dbConn, op, models.OperacionRechazada, &motivo)
	}

	// Error de red/timeout: no sabemos si el destino llegó a procesar la
	// solicitud. Consultamos antes de decidir cualquier cosa.
	ctx, cancel := context.WithTimeout(context.Background(), timeoutLlamadaBanco)
	defer cancel()

	consulta, consultaErr := cliente.ConsultarCredito(ctx, baseURL, op.IdempotencyKey)
	if consultaErr == nil {
		if consulta.Estado == "ACREDITADO" {
			return actualizarEstado(dbConn, op, models.OperacionCompletada, nil)
		}
		motivo := "rechazado por el banco destino (confirmado por consulta)"
		revertirReserva(dbConn, op)
		return actualizarEstado(dbConn, op, models.OperacionRechazada, &motivo)
	}

	return actualizarEstado(dbConn, op, models.OperacionPendiente, nil)
}

func revertirReserva(dbConn *sqlx.DB, op models.Operacion) {
	dbConn.MustExec(`
		UPDATE cuentas SET saldo = saldo + $1
		WHERE banco_id = $2 AND numero_cuenta = $3
	`, op.Importe, op.BancoOrigen, op.NumeroCuentaOrigen)
}

func actualizarEstado(dbConn *sqlx.DB, op models.Operacion, estado string, motivoRechazo *string) models.Operacion {
	dbConn.MustExec(`
		UPDATE operaciones SET estado = $1, motivo_rechazo = $2, actualizado_en = now() WHERE id = $3
	`, estado, motivoRechazo, op.ID)
	op.Estado = estado
	op.MotivoRechazo = motivoRechazo
	return op
}

func responderOperacionInterbancaria(c *gin.Context, op models.Operacion) {
	switch op.Estado {
	case models.OperacionCompletada:
		c.JSON(http.StatusCreated, op)
	case models.OperacionRechazada:
		c.JSON(http.StatusUnprocessableEntity, op)
	case models.OperacionPendiente, models.OperacionReservada:
		c.JSON(http.StatusAccepted, op)
	default:
		c.JSON(http.StatusOK, op)
	}
}
