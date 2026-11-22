package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
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
	// Concepto es opcional (doc 02): texto libre que viaja junto con el
	// crédito al banco destino. Ver internal/handlers/concepto.go.
	Concepto string `json:"concepto"`
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
		if conceptoDemasiadoLargo(req.Concepto) {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("concepto no puede superar los %d caracteres", limiteConceptoChars)})
			return
		}

		// Igual que en TransferenciaInterna: esta ruta la invoca el propio
		// usuario de banca personal desde su sesión, así que hay que
		// confirmar que numero_cuenta_origen sea realmente suya antes de
		// reservar y debitar nada.
		usuarioID := c.GetString("usuarioID")
		esDueno, err := esDuenoDeCuenta(dbConn, bancoID, req.NumeroCuentaOrigen, usuarioID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error verificando la cuenta"})
			return
		}
		if !esDueno {
			c.JSON(http.StatusNotFound, gin.H{"error": "la cuenta no existe o no te pertenece"})
			return
		}

		var existente models.Operacion
		err = dbConn.Get(&existente, `
			SELECT * FROM operaciones
			WHERE banco_origen = $1 AND idempotency_key = $2 AND tipo = $3
		`, bancoID, req.IdempotencyKey, models.TipoTransferenciaInterbancaria)
		if err == nil {
			if existente.NumeroCuentaOrigen != req.NumeroCuentaOrigen ||
				existente.BancoDestino == nil || *existente.BancoDestino != req.BancoDestino ||
				existente.NumeroCuentaDestino == nil || *existente.NumeroCuentaDestino != req.NumeroCuentaDestino ||
				existente.Importe != req.Importe {
				responderErrorContrato(c, ErrConflictoIdempotencia, "la clave de idempotencia ya se usó con datos distintos")
				return
			}
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
			Concepto:      req.Concepto,
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

	var salidoHoy int64
	if err == nil {
		salidoHoy, err = totalSalidoHoy(tx, bancoID, req.NumeroCuentaOrigen)
	}

	var motivoRechazo string
	switch {
	case errors.Is(err, sql.ErrNoRows):
		motivoRechazo = "cuenta origen no encontrada"
	case err != nil:
		return models.Operacion{}, err
	case origen.Estado != models.EstadoActiva:
		motivoRechazo = "cuenta origen no está activa"
	case req.Importe > LimiteMaximoTransferencia:
		motivoRechazo = "importe supera el límite máximo por transferencia"
	case salidoHoy+req.Importe > LimiteDiarioCombinado:
		motivoRechazo = "se supera el límite diario combinado"
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
			banco_destino, numero_cuenta_destino, importe, estado, motivo_rechazo, concepto
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING *
	`, models.TipoTransferenciaInterbancaria, req.IdempotencyKey, bancoID, req.NumeroCuentaOrigen,
		req.BancoDestino, req.NumeroCuentaDestino, req.Importe, estado, motivoPtr, conceptoOrNil(req.Concepto))
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

	if callErr == nil && resp.Estado == "RECHAZADO" {
		motivo := resp.MotivoRechazo
		revertirReserva(dbConn, op)
		return actualizarEstado(dbConn, op, models.OperacionRechazada, &motivo)
	}

	if ibErr, ok := callErr.(*interbanco.ErrorInterbancario); ok {
		log.Printf("transferencia %s rechazada por el destino: %v", op.IdempotencyKey, ibErr)
		motivo := ibErr.Mensaje
		revertirReserva(dbConn, op)
		return actualizarEstado(dbConn, op, models.OperacionRechazada, &motivo)
	}

	log.Printf("transferencia %s: error de red/parseo llamando al destino: %v", op.IdempotencyKey, callErr)

	// Error de red/timeout: no sabemos si el destino llegó a procesar la
	// solicitud. Consultamos antes de decidir cualquier cosa.
	ctx, cancel := context.WithTimeout(context.Background(), timeoutLlamadaBanco)
	defer cancel()

	consulta, consultaErr := cliente.ConsultarCredito(ctx, baseURL, op.IdempotencyKey)
	if consultaErr != nil {
		log.Printf("transferencia %s: la consulta de verificación también falló: %v", op.IdempotencyKey, consultaErr)
	}
	if consultaErr == nil {
		if consulta.Estado == "ACREDITADO" {
			return actualizarEstado(dbConn, op, models.OperacionCompletada, nil)
		}
		motivo := consulta.MotivoRechazo
		if motivo == "" {
			motivo = "rechazado por el banco destino (confirmado por consulta)"
		}
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

// ReconciliarOperacion vuelve a consultar el estado de una operación
// PENDIENTE contra el banco destino, y la resuelve si ya hay respuesta.
// La usan tanto el endpoint manual como el job en background.
func ReconciliarOperacion(dbConn *sqlx.DB, cliente *interbanco.Cliente, registro map[string]config.BancoRegistrado, op models.Operacion) models.Operacion {
	if op.Estado != models.OperacionPendiente || op.BancoDestino == nil {
		return op
	}
	destino, ok := registro[*op.BancoDestino]
	if !ok {
		log.Printf("reconciliación de %s: banco destino %s ya no está en el registro", op.IdempotencyKey, *op.BancoDestino)
		return op
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeoutLlamadaBanco)
	defer cancel()

	consulta, err := cliente.ConsultarCredito(ctx, destino.URL, op.IdempotencyKey)
	if err == nil {
		if consulta.Estado == "ACREDITADO" {
			return actualizarEstado(dbConn, op, models.OperacionCompletada, nil)
		}
		motivo := consulta.MotivoRechazo
		if motivo == "" {
			motivo = "rechazado por el banco destino (confirmado por reconciliación)"
		}
		revertirReserva(dbConn, op)
		return actualizarEstado(dbConn, op, models.OperacionRechazada, &motivo)
	}

	ibErr, esNoEncontrado := err.(*interbanco.ErrorInterbancario)
	if !esNoEncontrado || ibErr.Codigo != ErrNoEncontrado {
		// Error ambiguo (red/timeout de la propia consulta): no sabemos
		// nada nuevo, seguimos esperando.
		log.Printf("reconciliación de %s: todavía sin resolverse: %v", op.IdempotencyKey, err)
		return op
	}

	// El destino confirma que nunca recibió nada: es seguro reenviar el
	// crédito original, no hay riesgo de duplicar (y si en el medio sí lo
	// hubiera recibido, su propia idempotencia nos protege igual).
	log.Printf("reconciliación de %s: el destino no tiene registro, reintentando el envío original", op.IdempotencyKey)
	payload := interbanco.CreditoPayload{
		TransferID:    op.IdempotencyKey,
		CuentaDestino: derefOrVacio(op.NumeroCuentaDestino),
		BancoOrigen:   op.BancoOrigen,
		CuentaOrigen:  op.NumeroCuentaOrigen,
		Importe:       op.Importe,
		Moneda:        "PYG",
		Concepto:      derefOrVacio(op.Concepto),
	}
	resp, reenvioErr := cliente.EnviarCredito(ctx, destino.URL, payload)
	if reenvioErr == nil && resp.Estado == "ACREDITADO" {
		return actualizarEstado(dbConn, op, models.OperacionCompletada, nil)
	}
	if ibErr2, ok := reenvioErr.(*interbanco.ErrorInterbancario); ok {
		motivo := ibErr2.Mensaje
		revertirReserva(dbConn, op)
		return actualizarEstado(dbConn, op, models.OperacionRechazada, &motivo)
	}
	log.Printf("reconciliación de %s: el reintento de envío también falló: %v", op.IdempotencyKey, reenvioErr)
	return op
}

func derefOrVacio(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// IniciarJobReconciliacion lanza un goroutine que reintenta periódicamente
// todas las operaciones PENDIENTE de este banco. No bloquea: corre en
// segundo plano mientras el proceso esté vivo.
func IniciarJobReconciliacion(dbConn *sqlx.DB, cliente *interbanco.Cliente, registro map[string]config.BancoRegistrado, bancoID string, intervalo time.Duration) {
	go func() {
		ticker := time.NewTicker(intervalo)
		defer ticker.Stop()
		for range ticker.C {
			var pendientes []models.Operacion
			err := dbConn.Select(&pendientes, `
				SELECT * FROM operaciones WHERE banco_origen = $1 AND tipo = $2 AND estado = $3
			`, bancoID, models.TipoTransferenciaInterbancaria, models.OperacionPendiente)
			if err != nil {
				log.Printf("job de reconciliación: error listando pendientes: %v", err)
				continue
			}
			for _, op := range pendientes {
				if resultado := ReconciliarOperacion(dbConn, cliente, registro, op); resultado.Estado != models.OperacionPendiente {
					log.Printf("job de reconciliación: %s pasó a %s", op.IdempotencyKey, resultado.Estado)
				}
			}
		}
	}()
}

// ForzarReconciliacion es el endpoint manual: POST
// /transferencias/interbancarias/{idempotency_key}/reconciliar, para no
// tener que esperar al próximo ciclo del job.
func ForzarReconciliacion(dbConn *sqlx.DB, bancoID string, cliente *interbanco.Cliente, registro map[string]config.BancoRegistrado) gin.HandlerFunc {
	return func(c *gin.Context) {
		idempotencyKey := c.Param("idempotency_key")

		var op models.Operacion
		err := dbConn.Get(&op, `
			SELECT * FROM operaciones WHERE banco_origen = $1 AND idempotency_key = $2 AND tipo = $3
		`, bancoID, idempotencyKey, models.TipoTransferenciaInterbancaria)
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "no existe esa operación"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error consultando la operación"})
			return
		}

		// Igual que en TransferenciaInterna/TransferenciaInterbancaria
		// (punto 1): esta ruta la usa el propio usuario de banca personal
		// desde su sesión, no otro banco, así que hay que confirmar que la
		// cuenta de origen de ESTA operación sea la suya. El idempotency_key
		// no es un secreto -- viaja en texto plano en el request original y
		// en su respuesta -- así que sin este chequeo cualquiera que lo
		// conociera podía ver el detalle completo de una transferencia
		// ajena (importe, cuentas, concepto) y forzar cuándo se resuelve.
		// Devolvemos el mismo 404 "no existe" que el caso de arriba, para
		// no delatar que la operación sí existe pero es de otro usuario.
		usuarioID := c.GetString("usuarioID")
		esDueno, err := esDuenoDeCuenta(dbConn, bancoID, op.NumeroCuentaOrigen, usuarioID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error verificando la cuenta"})
			return
		}
		if !esDueno {
			c.JSON(http.StatusNotFound, gin.H{"error": "no existe esa operación"})
			return
		}

		if op.Estado != models.OperacionPendiente {
			c.JSON(http.StatusConflict, gin.H{"error": "la operación no está en estado PENDIENTE", "estado_actual": op.Estado})
			return
		}

		resultado := ReconciliarOperacion(dbConn, cliente, registro, op)
		responderOperacionInterbancaria(c, resultado)
	}
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
