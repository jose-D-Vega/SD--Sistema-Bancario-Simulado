package handlers

import (
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"

	"github.com/sudameris/backend/internal/config"
)

type bancoListado struct {
	BancoID string `json:"banco_id"`
	Nombre  string `json:"nombre"`
}

// ListarBancos implementa GET /bancos: alimenta el selector "Banco
// destino" del wizard de envío del frontend, en vez de tener esa lista
// hardcodeada ahí. No requiere sesión ni firma — es información pública,
// nada de esto es sensible. Siempre incluye este mismo banco primero
// (usando el nombre que traiga el registro para su propia entrada, si la
// tiene — el registro oficial del BCP simulado puede listar los 4 bancos,
// este incluido), después el resto ordenado por banco_id para que la
// respuesta sea estable entre llamadas.
func ListarBancos(bancoID string, registro map[string]config.BancoRegistrado) gin.HandlerFunc {
	return func(c *gin.Context) {
		nombrePropio := bancoID
		if propio, ok := registro[bancoID]; ok && propio.Nombre != "" {
			nombrePropio = propio.Nombre
		}
		bancos := []bancoListado{{BancoID: bancoID, Nombre: nombrePropio}}

		ids := make([]string, 0, len(registro))
		for id := range registro {
			if id == bancoID {
				// Ya lo agregamos arriba; evita listarlo dos veces si el
				// registro también incluye una entrada para este banco.
				continue
			}
			ids = append(ids, id)
		}
		sort.Strings(ids)

		for _, id := range ids {
			nombre := registro[id].Nombre
			if nombre == "" {
				nombre = id
			}
			bancos = append(bancos, bancoListado{BancoID: id, Nombre: nombre})
		}

		c.JSON(http.StatusOK, gin.H{"bancos": bancos})
	}
}
