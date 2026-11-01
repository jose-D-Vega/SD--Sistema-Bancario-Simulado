package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/sudameris/backend/internal/models"
)

type creditoRequest struct {
	TransferID    string `json:"transfer_id" binding:"required"`
	CuentaDestino string `json:"cuenta_destino" binding:"required"`
	BancoOrigen   string `json:"banco_origen" binding:"required"`
	CuentaOrigen  string `json:"cuenta_origen" binding:"required"`
	Importe       int64  `json:"importe" binding:"required"`
	Moneda        string `json:"moneda"`
	Concepto      string `json:"concepto"`
}

// estadoExterno traduce nuestros estados internos (compartidos con las
// transferencias internas) al vocabulario que usa el contrato para
// créditos interbancarios.
func estadoExterno(interno string) string {
	if interno == models.OperacionCompletada {
		return "ACREDITADO"
	}
	return "RECHAZADO"
}

// RecibirCredito implementa POST /interbancario/creditos (doc 02, paso 2):
// otro banco nos pide acreditar un importe en una cuenta nuestra.
func RecibirCredito(dbConn *sqlx.DB, bancoID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		bankID := c.GetString("bankID") // seteado por VerifyBankSignature

		var req creditoRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			responderErrorContrato(c, ErrSolicitudInvalida, err.Error())
			return
		}
		if req.BancoOrigen != bankID {
			responderErrorContrato(c, ErrSolicitudInvalida, "banco_origen del body no coincide con X-Bank-Id")
			return
		}
		if req.Importe <= 0 {
			responderErrorContrato(c, ErrSolicitudInvalida, "importe debe ser mayor a 0")
			return
		}

		var existente models.Operacion
		err := dbConn.Get(&existente, `
			SELECT * FROM operaciones WHERE banco_origen = $1 AND idempotency_key = $2
		`, bankID, req.TransferID)
		if err == nil {
			if existente.NumeroCuentaDestino == nil || *existente.NumeroCuentaDestino != req.CuentaDestino || existente.Importe != req.Importe {
				responderErrorContrato(c, ErrConflictoIdempotencia, "transfer_id ya usado con datos distintos")
				return
			}
			c.JSON(http.StatusOK, gin.H{"transfer_id": req.TransferID, "estado": estadoExterno(existente.Estado)})
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			responderErrorContrato(c, ErrInterno, "error verificando idempotencia")
			return
		}

		tx, err := dbConn.Beginx()
		if err != nil {
			responderErrorContrato(c, ErrInterno, "error iniciando transacción")
			return
		}
		defer tx.Rollback()

		var cuenta models.Cuenta
		err = tx.Get(&cuenta, `
			SELECT * FROM cuentas WHERE banco_id = $1 AND numero_cuenta = $2 FOR UPDATE
		`, bancoID, req.CuentaDestino)

		var motivoRechazo string
		var codigoRechazo string
		switch {
		case errors.Is(err, sql.ErrNoRows):
			motivoRechazo, codigoRechazo = "la cuenta destino no existe", ErrCuentaNoExiste
		case err != nil:
			log.Printf("RecibirCredito: error consultando cuenta destino %s: %v", req.CuentaDestino, err)
			responderErrorContrato(c, ErrInterno, "error consultando cuenta destino")
			return
		case cuenta.Estado != models.EstadoActiva:
			motivoRechazo, codigoRechazo = "la cuenta destino no está activa", ErrCuentaInactiva
		}

		if motivoRechazo != "" {
			if _, insErr := tx.Exec(`
				INSERT INTO operaciones (tipo, idempotency_key, banco_origen, numero_cuenta_origen,
					banco_destino, numero_cuenta_destino, importe, estado, motivo_rechazo)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`, models.TipoTransferenciaInterbancaria, req.TransferID, bankID, req.CuentaOrigen,
				bancoID, req.CuentaDestino, req.Importe, models.OperacionRechazada, motivoRechazo); insErr != nil {
				responderErrorContrato(c, ErrInterno, "error registrando operación")
				return
			}
			if err := tx.Commit(); err != nil {
				responderErrorContrato(c, ErrInterno, "error confirmando operación")
				return
			}
			responderErrorContrato(c, codigoRechazo, motivoRechazo)
			return
		}

		if _, err := tx.Exec(`UPDATE cuentas SET saldo = saldo + $1 WHERE id = $2`, req.Importe, cuenta.ID); err != nil {
			responderErrorContrato(c, ErrInterno, "error acreditando la cuenta")
			return
		}
		if _, err := tx.Exec(`
			INSERT INTO operaciones (tipo, idempotency_key, banco_origen, numero_cuenta_origen,
				banco_destino, numero_cuenta_destino, importe, estado)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, models.TipoTransferenciaInterbancaria, req.TransferID, bankID, req.CuentaOrigen,
			bancoID, req.CuentaDestino, req.Importe, models.OperacionCompletada); err != nil {
			responderErrorContrato(c, ErrInterno, "error registrando operación")
			return
		}
		if err := tx.Commit(); err != nil {
			responderErrorContrato(c, ErrInterno, "error confirmando acreditación")
			return
		}

		c.JSON(http.StatusOK, gin.H{"transfer_id": req.TransferID, "estado": "ACREDITADO"})
	}
}

// ConsultarCredito implementa GET /interbancario/creditos/{transfer_id}
// (doc 02, paso 4): el banco que inició la transferencia consulta si el
// crédito se aplicó, para el caso de haber sufrido un timeout.
func ConsultarCredito(dbConn *sqlx.DB, bancoID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		bankID := c.GetString("bankID")
		transferID := c.Param("transfer_id")

		var op models.Operacion
		err := dbConn.Get(&op, `
			SELECT * FROM operaciones
			WHERE banco_origen = $1 AND idempotency_key = $2 AND banco_destino = $3
		`, bankID, transferID, bancoID)

		if errors.Is(err, sql.ErrNoRows) {
			responderErrorContrato(c, ErrNoEncontrado, "no existe un crédito con ese transfer_id")
			return
		}
		if err != nil {
			responderErrorContrato(c, ErrInterno, "error consultando la operación")
			return
		}

		c.JSON(http.StatusOK, gin.H{"transfer_id": transferID, "estado": estadoExterno(op.Estado)})
	}
}
