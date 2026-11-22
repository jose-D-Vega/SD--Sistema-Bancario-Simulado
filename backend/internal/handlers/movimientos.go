package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/sudameris/backend/internal/models"
)

// movimientoResponse le suma a la operación tal cual está guardada un
// campo "direccion", calculado respecto a la cuenta consultada — así el
// frontend no tiene que reinventar la lógica de "banco_origen vs
// banco_destino" para decidir si pintar el movimiento como ingreso o
// egreso (los filtros "Ingresos/Egresos" del prototipo).
type movimientoResponse struct {
	models.Operacion
	Direccion string `json:"direccion"` // "ENTRANTE" o "SALIENTE"
}

const (
	limiteMovimientosDefault = 20
	limiteMovimientosMaximo  = 100
)

// cursorMovimientos es la paginación keyset (creado_en, id) codificada en
// base64: más robusta que un simple offset, porque no se desordena si
// entre una página y la siguiente se registra una operación nueva.
type cursorMovimientos struct {
	CreadoEn time.Time `json:"creado_en"`
	ID       string    `json:"id"`
}

func codificarCursorMovimientos(cur cursorMovimientos) (string, error) {
	b, err := json.Marshal(cur)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func decodificarCursorMovimientos(s string) (*cursorMovimientos, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	var cur cursorMovimientos
	if err := json.Unmarshal(b, &cur); err != nil {
		return nil, err
	}
	return &cur, nil
}

// ListarMovimientos implementa GET /cuentas/:numero_cuenta/movimientos
// (requiere sesión y ser dueño de la cuenta): el historial que usa la
// pantalla "Movimientos" del frontend. Reutiliza operaciones, la misma
// tabla de auditoría que ya exige el doc 01 — no hace falta tabla nueva.
//
// Importante sobre qué cuenta cada tipo (ver internal/handlers/limites.go
// y cajero.go, que ya documentan esto para los límites diarios): en una
// TRANSFERENCIA_* banco_origen/banco_destino son el banco de cada punta
// tal cual suena. En un RETIRO_CAJERO en cambio banco_origen es el banco
// DUEÑO DEL CAJERO (puede ser cualquiera) y banco_destino es el banco
// EMISOR de la cuenta debitada — por eso el filtro de cajero compara
// contra banco_destino, nunca contra banco_origen: si comparáramos contra
// banco_origen, un cajero nuestro procesando el retiro de la cuenta de
// OTRO banco con el mismo número que una cuenta nuestra (coincidencia de
// numero_cuenta entre bancos, posible porque no es globalmente único)
// podría colarse en el historial de un cliente que no tiene nada que ver.
//
// Parámetros de query opcionales: tipo, desde/hasta (RFC3339), limite
// (default 20, máximo 100) y cursor (de la respuesta anterior).
func ListarMovimientos(dbConn *sqlx.DB, bancoID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		usuarioID := c.GetString("usuarioID")
		numeroCuenta := c.Param("numero_cuenta")

		esDueno, err := esDuenoDeCuenta(dbConn, bancoID, numeroCuenta, usuarioID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error verificando la cuenta"})
			return
		}
		if !esDueno {
			c.JSON(http.StatusNotFound, gin.H{"error": "la cuenta no existe o no te pertenece"})
			return
		}

		limite := limiteMovimientosDefault
		if raw := c.Query("limite"); raw != "" {
			n, convErr := strconv.Atoi(raw)
			if convErr != nil || n <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "limite inválido: tiene que ser un entero positivo"})
				return
			}
			if n > limiteMovimientosMaximo {
				n = limiteMovimientosMaximo
			}
			limite = n
		}

		var tipoFiltro string
		if raw := c.Query("tipo"); raw != "" {
			switch raw {
			case models.TipoTransferenciaInterna, models.TipoTransferenciaInterbancaria, models.TipoRetiroCajero:
				tipoFiltro = raw
			default:
				c.JSON(http.StatusBadRequest, gin.H{"error": "tipo inválido: usá TRANSFERENCIA_INTERNA, TRANSFERENCIA_INTERBANCARIA o RETIRO_CAJERO"})
				return
			}
		}

		const layoutFecha = time.RFC3339
		var desde, hasta *time.Time
		if raw := c.Query("desde"); raw != "" {
			t, parseErr := time.Parse(layoutFecha, raw)
			if parseErr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "desde debe ser una fecha RFC3339, ej. 2026-11-01T00:00:00Z"})
				return
			}
			desde = &t
		}
		if raw := c.Query("hasta"); raw != "" {
			t, parseErr := time.Parse(layoutFecha, raw)
			if parseErr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "hasta debe ser una fecha RFC3339, ej. 2026-11-30T23:59:59Z"})
				return
			}
			hasta = &t
		}

		var cursor *cursorMovimientos
		if raw := c.Query("cursor"); raw != "" {
			cur, parseErr := decodificarCursorMovimientos(raw)
			if parseErr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "cursor inválido"})
				return
			}
			cursor = cur
		}

		// Arma el WHERE dinámicamente: los filtros son opcionales, pero
		// cada valor sigue yendo parametrizado (nunca interpolado como
		// texto), así que no hay riesgo de inyección.
		// La cláusula del medio (créditos entrantes) exige estado =
		// COMPLETADA a propósito: las otras dos describen intentos sobre
		// ESTA cuenta (un envío o un retiro que esta cuenta inició), y
		// tiene sentido mostrarlos aunque hayan fallado — es información
		// sobre la propia cuenta, como una compra rechazada en una
		// tarjeta. La del medio en cambio es plata que OTRA cuenta
		// intentó mandar hacia acá: si ese intento fue RECHAZADA o
		// REVERTIDA, no entró nada, y mostrarlo iguel sería un ingreso
		// fantasma en el resumen de alguien que no tuvo nada que ver con
		// por qué falló.
		query := `
			SELECT * FROM operaciones
			WHERE (
				(banco_origen = $1 AND numero_cuenta_origen = $2 AND tipo <> $3)
				OR (banco_destino = $1 AND numero_cuenta_destino = $2 AND estado = $4)
				OR (banco_destino = $1 AND numero_cuenta_origen = $2 AND tipo = $3)
			)`
		args := []interface{}{bancoID, numeroCuenta, models.TipoRetiroCajero, models.OperacionCompletada}

		if tipoFiltro != "" {
			query += fmt.Sprintf(" AND tipo = $%d", len(args)+1)
			args = append(args, tipoFiltro)
		}
		if desde != nil {
			query += fmt.Sprintf(" AND creado_en >= $%d", len(args)+1)
			args = append(args, *desde)
		}
		if hasta != nil {
			query += fmt.Sprintf(" AND creado_en <= $%d", len(args)+1)
			args = append(args, *hasta)
		}
		if cursor != nil {
			query += fmt.Sprintf(" AND (creado_en, id) < ($%d, $%d::uuid)", len(args)+1, len(args)+2)
			args = append(args, cursor.CreadoEn, cursor.ID)
		}
		query += " ORDER BY creado_en DESC, id DESC"
		query += fmt.Sprintf(" LIMIT $%d", len(args)+1)
		args = append(args, limite+1) // uno de más, para saber si hay página siguiente sin un segundo round-trip

		var operaciones []models.Operacion
		if err := dbConn.Select(&operaciones, query, args...); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error consultando los movimientos"})
			return
		}

		haySiguiente := len(operaciones) > limite
		if haySiguiente {
			operaciones = operaciones[:limite]
		}

		movimientos := make([]movimientoResponse, 0, len(operaciones))
		for _, op := range operaciones {
			direccion := "SALIENTE"
			entrante := op.Tipo != models.TipoRetiroCajero &&
				op.Estado == models.OperacionCompletada &&
				op.BancoDestino != nil && *op.BancoDestino == bancoID &&
				op.NumeroCuentaDestino != nil && *op.NumeroCuentaDestino == numeroCuenta
			if entrante {
				direccion = "ENTRANTE"
			}
			movimientos = append(movimientos, movimientoResponse{Operacion: op, Direccion: direccion})
		}

		var siguienteCursor *string
		if haySiguiente {
			ultimo := operaciones[len(operaciones)-1]
			token, err := codificarCursorMovimientos(cursorMovimientos{CreadoEn: ultimo.CreadoEn, ID: ultimo.ID.String()})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "error generando el cursor de paginación"})
				return
			}
			siguienteCursor = &token
		}

		c.JSON(http.StatusOK, gin.H{
			"movimientos":      movimientos,
			"siguiente_cursor": siguienteCursor,
		})
	}
}
