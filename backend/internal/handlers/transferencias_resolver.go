package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/sudameris/backend/internal/config"
	"github.com/sudameris/backend/internal/interbanco"
	"github.com/sudameris/backend/internal/models"
)

type resolverDestinoResponse struct {
	Banco        string `json:"banco"`
	NumeroCuenta string `json:"numero_cuenta"`
	Titular      string `json:"titular"`
	Activa       bool   `json:"activa"`
}

// ResolverDestino implementa GET /transferencias/resolver-destino, paso
// previo (de solo lectura) al wizard de "Enviar dinero": antes de
// confirmar una transferencia, el frontend necesita mostrarle al usuario
// a quién le está por mandar la plata.
//
// Esto es una extensión propia (banca personal), no parte del contrato de
// interoperabilidad — no reserva fondos ni modifica nada, solo consulta.
// Los endpoints de transferencia (POST /transferencias/internas y
// /interbancarias) siguen recibiendo numero_cuenta_destino directamente,
// tal cual antes: el frontend resuelve el alias ACÁ primero, y recién
// después llama a esos endpoints con el numero_cuenta que le devolvimos.
//
//   - banco = este banco: busca por alias O numero_cuenta en la tabla
//     local directamente, sin red.
//   - banco = otro banco: el alias NO viaja entre bancos (doc 02 solo
//     define banco_id + numero_cuenta), así que acá "identificador" tiene
//     que ser ya un numero_cuenta real; se resuelve llamando al GET
//     /cuentas/:numero_cuenta firmado de ese banco.
func ResolverDestino(dbConn *sqlx.DB, bancoID string, cliente *interbanco.Cliente, registro map[string]config.BancoRegistrado) gin.HandlerFunc {
	return func(c *gin.Context) {
		bancoDestino := c.Query("banco")
		identificador := c.Query("identificador")
		if bancoDestino == "" || identificador == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "faltan los parámetros banco e identificador"})
			return
		}

		if bancoDestino == bancoID {
			resolverDestinoLocal(c, dbConn, bancoID, identificador)
			return
		}

		resolverDestinoRemoto(c, bancoDestino, identificador, cliente, registro)
	}
}

func resolverDestinoLocal(c *gin.Context, dbConn *sqlx.DB, bancoID, identificador string) {
	var cuenta models.Cuenta
	err := dbConn.Get(&cuenta, `
		SELECT * FROM cuentas WHERE banco_id = $1 AND (numero_cuenta = $2 OR alias = $2)
	`, bancoID, identificador)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no existe una cuenta con ese alias o número en este banco"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error consultando la cuenta"})
		return
	}

	c.JSON(http.StatusOK, resolverDestinoResponse{
		Banco:        bancoID,
		NumeroCuenta: cuenta.NumeroCuenta,
		Titular:      cuenta.Titular,
		Activa:       cuenta.Estado == models.EstadoActiva,
	})
}

func resolverDestinoRemoto(c *gin.Context, bancoDestino, numeroCuenta string, cliente *interbanco.Cliente, registro map[string]config.BancoRegistrado) {
	destino, ok := registro[bancoDestino]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "banco desconocido: " + bancoDestino})
		return
	}
	if cliente == nil {
		// Mismo caso que /transferencias/interbancarias: sin
		// BANCO_PRIVATE_KEY configurada este banco no puede firmar
		// solicitudes salientes, así que tampoco puede consultar cuentas
		// de otros bancos.
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "este banco todavía no tiene configurada su clave privada para hablar con otros bancos"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeoutLlamadaBanco)
	defer cancel()

	cuenta, err := cliente.ConsultarCuenta(ctx, destino.URL, numeroCuenta)
	if err != nil {
		var ibErr *interbanco.ErrorInterbancario
		if errors.As(err, &ibErr) {
			// Rechazo explícito del banco destino (ej. CUENTA_NO_EXISTE):
			// le devolvemos el mismo status que mandó él.
			c.JSON(ibErr.Status, gin.H{"error": ibErr.Codigo, "mensaje": ibErr.Mensaje})
			return
		}
		// Error de red/timeout: no sabemos nada de la cuenta, no es que
		// "no existe" — son cosas distintas y conviene no confundirlas.
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "no se pudo contactar al banco destino, probá de nuevo"})
		return
	}

	c.JSON(http.StatusOK, resolverDestinoResponse{
		Banco:        cuenta.BancoID,
		NumeroCuenta: cuenta.NumeroCuenta,
		Titular:      cuenta.Titular,
		Activa:       cuenta.Estado == models.EstadoActiva,
	})
}
