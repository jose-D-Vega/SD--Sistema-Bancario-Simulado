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

func responderAutorizacion(c *gin.Context, operacionID string, op models.Operacion) {
	body := gin.H{"operacion_id": operacionID, "estado": estadoExternoCajero(op.Estado)}
	if op.MotivoRechazo != nil {
		body["motivo_rechazo"] = *op.MotivoRechazo
	}
	c.JSON(http.StatusOK, body)
}

// coincideRetiroCajero compara una operación ya registrada con los datos
// de esta solicitud: si coinciden, es un reintento legítimo del mismo
// retiro; si no, es un conflicto real de idempotencia. La usan tanto el
// fast path normal como manejarCarreraDeIdempotencia.
func coincideRetiroCajero(existente models.Operacion, req autorizacionCajeroRequest) bool {
	return existente.NumeroCuentaOrigen == req.NumeroCuenta && existente.Importe == req.Importe
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

		existente, encontrada, err := buscarOperacionPorIdempotencia(dbConn, bankID, req.OperacionID)
		if err != nil {
			responderErrorContrato(c, ErrInterno, "error verificando idempotencia")
			return
		}
		if encontrada {
			if !coincideRetiroCajero(existente, req) {
				responderErrorContrato(c, ErrConflictoIdempotencia, "el operacion_id ya se usó con datos distintos")
				return
			}
			responderAutorizacion(c, req.OperacionID, existente)
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

		var codigoRechazo string
		switch {
		case errors.Is(err, sql.ErrNoRows):
			codigoRechazo = ErrCuentaNoExiste
		case err != nil:
			responderErrorContrato(c, ErrInterno, "error consultando la cuenta")
			return
		case cuenta.Estado != models.EstadoActiva:
			codigoRechazo = ErrCuentaInactiva
		case cuenta.PinHash == nil || bcrypt.CompareHashAndPassword([]byte(*cuenta.PinHash), []byte(req.Pin)) != nil:
			codigoRechazo = ErrPinIncorrecto
		case req.Importe > LimiteMaximoRetiroCajero:
			codigoRechazo = ErrLimiteExcedido
		case salidoHoy+req.Importe > LimiteDiarioCombinado:
			codigoRechazo = ErrLimiteExcedido
		case cuenta.Saldo < req.Importe:
			codigoRechazo = ErrFondosInsuficientes
		}

		if codigoRechazo != "" {
			if _, insErr := tx.Exec(`
				INSERT INTO operaciones (tipo, idempotency_key, banco_origen, numero_cuenta_origen,
					banco_destino, importe, estado, motivo_rechazo)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`, models.TipoRetiroCajero, req.OperacionID, bankID, req.NumeroCuenta,
				bancoID, req.Importe, models.OperacionRechazada, codigoRechazo); insErr != nil {
				if manejarCarreraDeIdempotencia(c, dbConn, bankID, req.OperacionID, insErr,
					"el operacion_id ya se usó con datos distintos",
					func(e models.Operacion) bool { return coincideRetiroCajero(e, req) },
					func(e models.Operacion) { responderAutorizacion(c, req.OperacionID, e) },
				) {
					return
				}
				responderErrorContrato(c, ErrInterno, "error registrando operación")
				return
			}
			if err := tx.Commit(); err != nil {
				responderErrorContrato(c, ErrInterno, "error confirmando operación")
				return
			}
			c.JSON(http.StatusOK, gin.H{"operacion_id": req.OperacionID, "estado": "RECHAZADO", "motivo_rechazo": codigoRechazo})
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
			if manejarCarreraDeIdempotencia(c, dbConn, bankID, req.OperacionID, err,
				"el operacion_id ya se usó con datos distintos",
				func(e models.Operacion) bool { return coincideRetiroCajero(e, req) },
				func(e models.Operacion) { responderAutorizacion(c, req.OperacionID, e) },
			) {
				return
			}
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

		responderAutorizacion(c, operacionID, op)
	}
}
