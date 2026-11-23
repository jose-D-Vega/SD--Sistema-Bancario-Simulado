package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// routerConAdminKey arma un router mínimo: RequireAdminKey seguido de un
// handler que solo confirma que llegó. Así se prueba el middleware
// aislado, sin acoplarse a CrearCuenta ni CambiarEstadoCuenta.
func routerConAdminKey(adminKey string) *gin.Engine {
	r := gin.New()
	r.Use(RequireAdminKey(adminKey))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestRequireAdminKey_HeaderCorrectoPasa(t *testing.T) {
	router := routerConAdminKey("el-secreto")

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-Admin-Key", "el-secreto")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	esperarStatus(t, w, http.StatusOK)
}

func TestRequireAdminKey_SinHeader(t *testing.T) {
	router := routerConAdminKey("el-secreto")

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	// Sin X-Admin-Key.

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	esperarStatus(t, w, http.StatusUnauthorized)
}

func TestRequireAdminKey_HeaderIncorrecto(t *testing.T) {
	router := routerConAdminKey("el-secreto")

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-Admin-Key", "otro-valor")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	esperarStatus(t, w, http.StatusUnauthorized)
}

func TestRequireAdminKey_HeaderIncorrectoDeOtraLongitud(t *testing.T) {
	// Además de "distinto", distinto LARGO -- para que quede ejercitado
	// el chequeo de longitud antes de subtle.ConstantTimeCompare.
	router := routerConAdminKey("el-secreto")

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-Admin-Key", "corto")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	esperarStatus(t, w, http.StatusUnauthorized)
}

// Si ADMIN_API_KEY no está configurada en el servidor, el middleware
// bloquea TODO en vez de dejar pasar -- fail-closed a propósito, incluso
// si alguien manda un X-Admin-Key vacío que "coincidiría" con un secreto
// vacío.
func TestRequireAdminKey_SinConfigurarEnElServidorBloqueaSiempre(t *testing.T) {
	router := routerConAdminKey("")

	casos := []struct {
		nombre      string
		headerValor *string // nil = no mandar el header
	}{
		{"sin header", nil},
		{"header vacío", strPtr("")},
		{"header con algún valor", strPtr("cualquier-cosa")},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			if c.headerValor != nil {
				req.Header.Set("X-Admin-Key", *c.headerValor)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			esperarStatus(t, w, http.StatusServiceUnavailable)
		})
	}
}

func strPtr(s string) *string { return &s }
