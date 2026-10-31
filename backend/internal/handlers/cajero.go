package handlers

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"

	"github.com/sudameris/backend/internal/models"
)

type autorizacionCajeroRequest struct {
	OperacionID  string `json:"operacion_id" binding:"required"`
	BancoCuenta  string `json:"banco_cuenta" binding:"required"`
	NumeroCuenta string `json:"numero_cuenta" binding:"required"`
	Pin          string `json:"pin" binding:"required"`
	Importe      int64  `json:"importe" binding:"required"`
	Moneda       string `json:"moneda"`
}

func estadoExternoCajero(interno string) string {
	if interno == models.OperacionCompletada {
		return "AUTORIZADO"
	}
	return "RECHAZADO"
}

// AutorizarRetiroCajero implementa POST /cajero/autorizaciones (doc 03):
// el cajero (de este banco o de cualquier otro) le pide al banco emisor
// que autorice y debite un retiro de una cuenta propia.
func AutorizarRetiroCajero(dbConn *sqlx.DB, bancoID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		bankID := c.GetString("bankID") // dueño del cajero, seteado por VerifyBankSignature

		var req autorizacionCajeroRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			responderErrorContrato(c, ErrSolicitudInvalida, err.Error())
			return
		}
		if req.BancoCuenta != bancoID {
			responderErrorContrato(c, ErrSolicitudInvalida, "banco_cuenta no corresponde a este banco")
			return
		}
		if req.Importe <= 0 {
			responderErrorContrato(c, ErrSolicitudInvalida, "importe debe ser mayor a 0")
			return
		}

		var existente models.Operacion
		err := dbConn.Get(&existente, `
			SELECT * FROM operaciones WHERE banco_origen = $1 AND idempotency_key = $2 AND tipo = $3
		`, bankID, req.OperacionID, models.TipoRetiroCajero)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"operacion_id": req.OperacionID, "estado": estadoExternoCajero(existente.Estado)})
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
		`, bancoID, req.NumeroCuenta)

		var salidoHoy int64
		if err == nil {
			salidoHoy, err = totalSalidoHoy(tx, bancoID, req.NumeroCuenta)
		}

		var motivoRechazo, codigoRechazo string
		switch {
		case errors.Is(err, sql.ErrNoRows):
			motivoRechazo, codigoRechazo = "la cuenta no existe", ErrCuentaNoExiste
		case err != nil:
			responderErrorContrato(c, ErrInterno, "error consultando la cuenta")
			return
		case cuenta.Estado != models.EstadoActiva:
			motivoRechazo, codigoRechazo = "la cuenta no está activa", ErrCuentaInactiva
		case cuenta.PinHash == nil || bcrypt.CompareHashAndPassword([]byte(*cuenta.PinHash), []byte(req.Pin)) != nil:
			motivoRechazo, codigoRechazo = "PIN incorrecto", ErrPinIncorrecto
		case req.Importe > LimiteMaximoRetiroCajero:
			motivoRechazo, codigoRechazo = "importe supera el límite por retiro", ErrLimiteExcedido
		case salidoHoy+req.Importe > LimiteDiarioCombinado:
			motivoRechazo, codigoRechazo = "se supera el límite diario combinado", ErrLimiteExcedido
		case cuenta.Saldo < req.Importe:
			motivoRechazo, codigoRechazo = "saldo insuficiente", ErrFondosInsuficientes
		}

		if motivoRechazo != "" {
			if _, insErr := tx.Exec(`
				INSERT INTO operaciones (tipo, idempotency_key, banco_origen, numero_cuenta_origen,
					banco_destino, importe, estado, motivo_rechazo)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`, models.TipoRetiroCajero, req.OperacionID, bankID, req.NumeroCuenta,
				bancoID, req.Importe, models.OperacionRechazada, motivoRechazo); insErr != nil {
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

		if _, err := tx.Exec(`UPDATE cuentas SET saldo = saldo - $1 WHERE id = $2`, req.Importe, cuenta.ID); err != nil {
			responderErrorContrato(c, ErrInterno, "error debitando la cuenta")
			return
		}
		if _, err := tx.Exec(`
			INSERT INTO operaciones (tipo, idempotency_key, banco_origen, numero_cuenta_origen,
				banco_destino, importe, estado)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, models.TipoRetiroCajero, req.OperacionID, bankID, req.NumeroCuenta,
			bancoID, req.Importe, models.OperacionCompletada); err != nil {
			responderErrorContrato(c, ErrInterno, "error registrando operación")
			return
		}
		if err := tx.Commit(); err != nil {
			responderErrorContrato(c, ErrInterno, "error confirmando retiro")
			return
		}

		c.JSON(http.StatusOK, gin.H{"operacion_id": req.OperacionID, "estado": "AUTORIZADO"})
	}
}

// ConsultarAutorizacionCajero implementa GET /cajero/autorizaciones/{operacion_id}
// (doc 03, sección 3): para cuando el cajero no recibió respuesta a tiempo.
func ConsultarAutorizacionCajero(dbConn *sqlx.DB, bancoID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		bankID := c.GetString("bankID")
		operacionID := c.Param("operacion_id")

		var op models.Operacion
		err := dbConn.Get(&op, `
			SELECT * FROM operaciones
			WHERE banco_origen = $1 AND idempotency_key = $2 AND banco_destino = $3 AND tipo = $4
		`, bankID, operacionID, bancoID, models.TipoRetiroCajero)

		if errors.Is(err, sql.ErrNoRows) {
			responderErrorContrato(c, ErrNoEncontrado, "no existe una autorización con ese operacion_id")
			return
		}
		if err != nil {
			responderErrorContrato(c, ErrInterno, "error consultando la operación")
			return
		}

		c.JSON(http.StatusOK, gin.H{"operacion_id": operacionID, "estado": estadoExternoCajero(op.Estado)})
	}
}
