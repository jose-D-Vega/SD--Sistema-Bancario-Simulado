package models

import (
	"time"

	"github.com/google/uuid"
)

type Cuenta struct {
	ID           uuid.UUID `db:"id" json:"id"`
	BancoID      string    `db:"banco_id" json:"banco_id"`
	NumeroCuenta string    `db:"numero_cuenta" json:"numero_cuenta"`
	Titular      string    `db:"titular" json:"titular"`
	Saldo        int64     `db:"saldo" json:"saldo"`
	Estado       string    `db:"estado" json:"estado"`
	PinHash      *string   `db:"pin_hash" json:"-"`
	CreadoEn     time.Time `db:"creado_en" json:"creado_en"`

	// Campos de banca personal (extensión propia, fuera del contrato de
	// interoperabilidad): a qué usuario pertenece, su alias local a este
	// banco, y el tipo de cuenta. Ver migración 0006.
	UsuarioID *uuid.UUID `db:"usuario_id" json:"usuario_id,omitempty"`
	Alias     *string    `db:"alias" json:"alias,omitempty"`
	Tipo      string     `db:"tipo" json:"tipo"`
}

const (
	EstadoActiva    = "ACTIVA"
	EstadoBloqueada = "BLOQUEADA"
	EstadoCerrada   = "CERRADA"
)

const (
	TipoCuentaCorriente = "CUENTA_CORRIENTE"
	TipoCajaAhorro      = "CAJA_AHORRO"
)
