package models

import (
	"time"

	"github.com/google/uuid"
)

// Usuario es el cliente humano que inicia sesión en la app (frontend). Es
// una entidad propia de este banco, separada de "Cuenta": un usuario puede
// tener una o más cuentas (ver Cuenta.UsuarioID). No tiene nada que ver con
// el banco_id de otros bancos ni con la firma Ed25519 del doc 02 — esto es
// autenticación de persona, no de banco.
type Usuario struct {
	ID           uuid.UUID `db:"id" json:"id"`
	Username     string    `db:"username" json:"username"`
	Documento    string    `db:"documento" json:"documento"`
	Nombre       string    `db:"nombre" json:"nombre"`
	Email        *string   `db:"email" json:"email,omitempty"`
	Telefono     *string   `db:"telefono" json:"telefono,omitempty"`
	PasswordHash string    `db:"password_hash" json:"-"`
	CreadoEn     time.Time `db:"creado_en" json:"creado_en"`
}
