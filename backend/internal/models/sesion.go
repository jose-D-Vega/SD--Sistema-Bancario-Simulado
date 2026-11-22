package models

import (
	"time"

	"github.com/google/uuid"
)

// Sesion es un token de login de usuario (no confundir con la firma Ed25519
// entre bancos del doc 02). El token en sí es el identificador: un valor
// aleatorio de alta entropía generado por el servidor, no algo que haga
// falta volver a hashear para buscarlo.
type Sesion struct {
	Token     string    `db:"token" json:"token"`
	UsuarioID uuid.UUID `db:"usuario_id" json:"usuario_id"`
	CreadoEn  time.Time `db:"creado_en" json:"creado_en"`
	ExpiraEn  time.Time `db:"expira_en" json:"expira_en"`
}
