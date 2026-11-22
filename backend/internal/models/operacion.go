package models

import (
	"time"

	"github.com/google/uuid"
)

type Operacion struct {
	ID             uuid.UUID `db:"id" json:"id"`
	Tipo           string    `db:"tipo" json:"tipo"`
	IdempotencyKey string    `db:"idempotency_key" json:"idempotency_key"`

	BancoOrigen         string  `db:"banco_origen" json:"banco_origen"`
	NumeroCuentaOrigen  string  `db:"numero_cuenta_origen" json:"numero_cuenta_origen"`
	BancoDestino        *string `db:"banco_destino" json:"banco_destino,omitempty"`
	NumeroCuentaDestino *string `db:"numero_cuenta_destino" json:"numero_cuenta_destino,omitempty"`

	Importe       int64   `db:"importe" json:"importe"`
	Estado        string  `db:"estado" json:"estado"`
	MotivoRechazo *string `db:"motivo_rechazo" json:"motivo_rechazo,omitempty"`

	// Concepto es texto libre y opcional (doc 02): NULL significa que
	// quien inició la operación no mandó ninguno, no que mandó uno vacío.
	// Ver internal/handlers/concepto.go para el límite de longitud y el
	// helper que lo convierte desde el string crudo del request.
	Concepto *string `db:"concepto" json:"concepto,omitempty"`

	CreadoEn      time.Time `db:"creado_en" json:"creado_en"`
	ActualizadoEn time.Time `db:"actualizado_en" json:"actualizado_en"`
}

const (
	TipoTransferenciaInterna       = "TRANSFERENCIA_INTERNA"
	TipoTransferenciaInterbancaria = "TRANSFERENCIA_INTERBANCARIA"
	TipoRetiroCajero               = "RETIRO_CAJERO"

	OperacionPendiente  = "PENDIENTE"
	OperacionRecibida   = "RECIBIDA"
	OperacionReservada  = "RESERVADA"
	OperacionCompletada = "COMPLETADA"
	OperacionRechazada  = "RECHAZADA"
	OperacionFallida    = "FALLIDA"
	OperacionRevertida  = "REVERTIDA"
)
