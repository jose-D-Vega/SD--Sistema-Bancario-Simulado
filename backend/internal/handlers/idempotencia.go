package handlers

import (
	"database/sql"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/sudameris/backend/internal/models"
)

// constraintIdempotencia es el nombre que CockroachDB le dio al UNIQUE
// (banco_origen, idempotency_key) de la migración 0002 -- confirmado con
// SHOW CONSTRAINTS FROM operaciones contra la base real del proyecto, no
// es un nombre adivinado. Si algún día se recrea esta tabla con un
// nombre explícito distinto para el constraint, hay que actualizar esto.
const constraintIdempotencia = "operaciones_banco_origen_idempotency_key_key"

// buscarOperacionPorIdempotencia busca una operación ya registrada para
// este banco_origen + idempotency_key. No filtra por tipo: el UNIQUE de
// la migración 0002 es sobre (banco_origen, idempotency_key) nada más,
// así que como mucho existe una fila, sea cual sea su tipo.
//
// encontrada=false junto con err=nil significa "no existe todavía"
// (sql.ErrNoRows es el resultado esperado en el camino feliz, no un
// error); cualquier otro err es un problema real de la consulta.
func buscarOperacionPorIdempotencia(dbConn *sqlx.DB, bancoID, idempotencyKey string) (op models.Operacion, encontrada bool, err error) {
	err = dbConn.Get(&op, `
		SELECT * FROM operaciones WHERE banco_origen = $1 AND idempotency_key = $2
	`, bancoID, idempotencyKey)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Operacion{}, false, nil
	}
	if err != nil {
		return models.Operacion{}, false, err
	}
	return op, true, nil
}

// esConflictoDeIdempotenciaConcurrente detecta específicamente la
// violación del UNIQUE de idempotencia -- no cualquier unique_violation
// de la tabla -- comparando también el nombre del constraint, para no
// tratar por error una violación de unicidad distinta (si el día de
// mañana se agrega otro índice único a "operaciones") como si fuera esta
// carrera.
func esConflictoDeIdempotenciaConcurrente(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == constraintIdempotencia
}

// manejarCarreraDeIdempotencia se llama cuando un INSERT en "operaciones"
// devolvió error, antes de tratarlo como un error interno genérico: si
// fue por perder la carrera contra el UNIQUE de idempotencia -- otra
// solicitud con el mismo banco_origen + idempotency_key ya commiteó
// mientras esta procesaba, algo esperable bajo concurrencia real (p. ej.
// un cajero u otro banco reintentando tras un timeout mientras la
// primera solicitud todavía se estaba procesando, que es justamente para
// lo que existe la idempotencia) -- busca esa fila ya commiteada y la
// trata exactamente igual que el fast path normal (mismo chequeo de
// "¿son los mismos datos?", misma respuesta). Sin esto, el que pierde la
// carrera recibía un 500 genérico en vez del resultado autoritativo,
// aunque el dinero en sí nunca corriera riesgo: el UPDATE de saldo de la
// transacción perdedora se revierte solo, por el defer tx.Rollback() de
// quien llama, al no poder commitear después de este error.
//
// Devuelve true si ya respondió (el caller no debe hacer nada más);
// false si el error no fue por esto, o si por algún motivo no encontró la
// fila (no debería pasar nunca si hubo un 23505 en este constraint) -- en
// ambos casos el caller mantiene su manejo de error genérico de siempre.
func manejarCarreraDeIdempotencia(
	c *gin.Context,
	dbConn *sqlx.DB,
	bancoID string,
	idempotencyKey string,
	insertErr error,
	mensajeConflicto string,
	coincide func(models.Operacion) bool,
	responder func(models.Operacion),
) bool {
	if !esConflictoDeIdempotenciaConcurrente(insertErr) {
		return false
	}
	existente, encontrada, err := buscarOperacionPorIdempotencia(dbConn, bancoID, idempotencyKey)
	if err != nil || !encontrada {
		return false
	}
	if !coincide(existente) {
		responderErrorContrato(c, ErrConflictoIdempotencia, mensajeConflicto)
		return true
	}
	responder(existente)
	return true
}
