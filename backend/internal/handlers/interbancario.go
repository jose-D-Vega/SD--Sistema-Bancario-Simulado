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

func responderCredito(c *gin.Context, transferID string, op models.Operacion) {
	body := gin.H{"transfer_id": transferID, "estado": estadoExterno(op.Estado)}
	if op.MotivoRechazo != nil {
		body["motivo_rechazo"] = *op.MotivoRechazo
	}
	c.JSON(http.StatusOK, body)
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
		// A diferencia de nuestras propias rutas salientes (transferencias.go,
		// transferencias_interbancarias.go), acá no rechazamos por un
		// concepto largo: el doc 02 no define un límite para este campo, y
		// esta es plata real que OTRO banco ya está mandando -- truncamos
		// en vez de arriesgarnos a rechazar un crédito legítimo por un
		// detalle cosmético que el contrato ni siquiera limita.
		concepto := conceptoOrNil(truncarConcepto(req.Concepto))

		var existente models.Operacion
		err := dbConn.Get(&existente, `
			SELECT * FROM operaciones WHERE banco_origen = $1 AND idempotency_key = $2
		`, bankID, req.TransferID)
		if err == nil {
			if existente.NumeroCuentaDestino == nil || *existente.NumeroCuentaDestino != req.CuentaDestino || existente.Importe != req.Importe {
				responderErrorContrato(c, ErrConflictoIdempotencia, "transfer_id ya usado con datos distintos")
				return
			}
			responderCredito(c, req.TransferID, existente)
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

		var codigoRechazo string
		switch {
		case errors.Is(err, sql.ErrNoRows):
			codigoRechazo = ErrCuentaNoExiste
		case err != nil:
			log.Printf("RecibirCredito: error consultando cuenta destino %s: %v", req.CuentaDestino, err)
			responderErrorContrato(c, ErrInterno, "error consultando cuenta destino")
			return
		case cuenta.Estado != models.EstadoActiva:
			codigoRechazo = ErrCuentaInactiva
		}

		if codigoRechazo != "" {
			if _, insErr := tx.Exec(`
				INSERT INTO operaciones (tipo, idempotency_key, banco_origen, numero_cuenta_origen,
					banco_destino, numero_cuenta_destino, importe, estado, motivo_rechazo, concepto)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			`, models.TipoTransferenciaInterbancaria, req.TransferID, bankID, req.CuentaOrigen,
				bancoID, req.CuentaDestino, req.Importe, models.OperacionRechazada, codigoRechazo, concepto); insErr != nil {
				responderErrorContrato(c, ErrInterno, "error registrando operación")
				return
			}
			if err := tx.Commit(); err != nil {
				responderErrorContrato(c, ErrInterno, "error confirmando operación")
				return
			}
			c.JSON(http.StatusOK, gin.H{"transfer_id": req.TransferID, "estado": "RECHAZADO", "motivo_rechazo": codigoRechazo})
			return
		}

		if _, err := tx.Exec(`UPDATE cuentas SET saldo = saldo + $1 WHERE id = $2`, req.Importe, cuenta.ID); err != nil {
			responderErrorContrato(c, ErrInterno, "error acreditando la cuenta")
			return
		}
		if _, err := tx.Exec(`
			INSERT INTO operaciones (tipo, idempotency_key, banco_origen, numero_cuenta_origen,
				banco_destino, numero_cuenta_destino, importe, estado, concepto)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, models.TipoTransferenciaInterbancaria, req.TransferID, bankID, req.CuentaOrigen,
			bancoID, req.CuentaDestino, req.Importe, models.OperacionCompletada, concepto); err != nil {
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

		responderCredito(c, transferID, op)
	}
}
