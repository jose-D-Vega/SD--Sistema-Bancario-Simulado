package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Catálogo de códigos de error del doc 02, sección 6, con su HTTP status
// correspondiente.
const (
	ErrCuentaNoExiste       = "CUENTA_NO_EXISTE"
	ErrCuentaInactiva       = "CUENTA_INACTIVA"
	ErrFondosInsuficientes  = "FONDOS_INSUFICIENTES"
	ErrLimiteExcedido       = "LIMITE_EXCEDIDO"
	ErrBancoNoReconocido    = "BANCO_NO_RECONOCIDO"
	ErrSolicitudInvalida    = "SOLICITUD_INVALIDA"
	ErrFirmaInvalida        = "FIRMA_INVALIDA"
	ErrTimestampFueraRango  = "TIMESTAMP_FUERA_DE_RANGO"
	ErrConflictoIdempotencia = "CONFLICTO_IDEMPOTENCIA"
	ErrNoEncontrado         = "NO_ENCONTRADO"
	ErrInterno              = "ERROR_INTERNO"
)

var errorStatus = map[string]int{
	ErrCuentaNoExiste:        http.StatusNotFound,
	ErrCuentaInactiva:        http.StatusConflict,
	ErrFondosInsuficientes:   http.StatusConflict,
	ErrLimiteExcedido:        http.StatusConflict,
	ErrBancoNoReconocido:     http.StatusBadRequest,
	ErrSolicitudInvalida:     http.StatusBadRequest,
	ErrFirmaInvalida:         http.StatusUnauthorized,
	ErrTimestampFueraRango:   http.StatusUnauthorized,
	ErrConflictoIdempotencia: http.StatusConflict,
	ErrNoEncontrado:          http.StatusNotFound,
	ErrInterno:               http.StatusInternalServerError,
}

func responderErrorContrato(c *gin.Context, codigo, mensaje string) {
	status, ok := errorStatus[codigo]
	if !ok {
		status = http.StatusInternalServerError
	}
	c.AbortWithStatusJSON(status, gin.H{"error": codigo, "mensaje": mensaje})
}
