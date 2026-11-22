package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/sudameris/backend/internal/models"
)

const duracionSesionDePrueba = 24 * time.Hour

type registroUsuarioResponse struct {
	Usuario  models.Usuario `json:"usuario"`
	Cuenta   models.Cuenta  `json:"cuenta"`
	Token    string         `json:"token"`
	ExpiraEn time.Time      `json:"expira_en"`
}

type loginUsuarioResponse struct {
	Usuario  models.Usuario `json:"usuario"`
	Token    string         `json:"token"`
	ExpiraEn time.Time      `json:"expira_en"`
}

func decodificarJSON[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("la respuesta no es el JSON esperado: %v\nbody: %s", err, w.Body.String())
	}
	return v
}

// ejecutarConToken es como ejecutarJSON pero para LogoutUsuario, que lee el
// token directo del header Authorization (como lo haría un cliente real)
// en vez de un usuarioID ya puesto en el contexto por RequireSesion.
func ejecutarConToken(t *testing.T, handler gin.HandlerFunc, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	handler(c)
	c.Writer.WriteHeaderNow() // ver el comentario equivalente en ejecutarJSON
	return w
}

func sesionExisteEnBase(t *testing.T, token string) bool {
	t.Helper()
	db := requireTestDB(t)
	var existe bool
	if err := db.Get(&existe, `SELECT EXISTS(SELECT 1 FROM sesiones WHERE token = $1)`, token); err != nil {
		t.Fatalf("consultando sesiones de prueba: %v", err)
	}
	return existe
}

// ---------------------------------------------------------------------
// Registro y login
// ---------------------------------------------------------------------

func TestRegistrarUsuario_Exitoso(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := RegistrarUsuario(db, bancoID, duracionSesionDePrueba)

	req := registroUsuarioRequest{
		Username:  "nueva-" + idAleatorio(t),
		Documento: idAleatorio(t),
		Nombre:    "Nueva Persona",
		Password:  "unaClaveSegura123",
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/usuarios/registro", req, nil, nil)
	esperarStatus(t, w, http.StatusCreated)

	resp := decodificarJSON[registroUsuarioResponse](t, w)
	if resp.Token == "" {
		t.Fatalf("esperaba un token de sesión ya creado")
	}
	if resp.Cuenta.Saldo != 0 {
		t.Fatalf("la cuenta inicial debería tener saldo 0, dio %d", resp.Cuenta.Saldo)
	}
	if resp.Cuenta.Estado != models.EstadoActiva {
		t.Fatalf("la cuenta inicial debería estar ACTIVA, dio %q", resp.Cuenta.Estado)
	}
	if resp.Cuenta.Tipo != models.TipoCuentaCorriente {
		t.Fatalf("sin tipo explícito debería caer a CUENTA_CORRIENTE, dio %q", resp.Cuenta.Tipo)
	}
	if !sesionExisteEnBase(t, resp.Token) {
		t.Fatalf("el token devuelto debería existir en la tabla sesiones")
	}
}

func TestRegistrarUsuario_RechazaTipoCuentaInicialInvalido(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := RegistrarUsuario(db, bancoID, duracionSesionDePrueba)

	req := registroUsuarioRequest{
		Username: "nueva-" + idAleatorio(t), Documento: idAleatorio(t),
		Nombre: "Nueva Persona", Password: "unaClaveSegura123",
		TipoCuentaInicial: "TIPO_QUE_NO_EXISTE",
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/usuarios/registro", req, nil, nil)
	esperarStatus(t, w, http.StatusBadRequest)
}

func TestRegistrarUsuario_RechazaUsernameDuplicado(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := RegistrarUsuario(db, bancoID, duracionSesionDePrueba)

	existente := crearUsuarioConCredenciales(t)

	req := registroUsuarioRequest{
		Username:  existente.Username, // mismo username, documento distinto
		Documento: idAleatorio(t), Nombre: "Otra Persona", Password: "otraClaveSegura123",
	}

	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/usuarios/registro", req, nil, nil)
	esperarStatus(t, w, http.StatusConflict)
}

func TestLoginUsuario_Exitoso(t *testing.T) {
	db := requireTestDB(t)
	handler := LoginUsuario(db, duracionSesionDePrueba)

	usuario := crearUsuarioConCredenciales(t)

	req := loginRequest{Usuario: usuario.Username, Password: usuario.Password}
	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/usuarios/login", req, nil, nil)
	esperarStatus(t, w, http.StatusOK)

	resp := decodificarJSON[loginUsuarioResponse](t, w)
	if resp.Token == "" {
		t.Fatalf("esperaba un token de sesión")
	}
	if resp.Usuario.ID.String() != usuario.ID {
		t.Fatalf("el usuario devuelto no coincide: esperaba %s, dio %s", usuario.ID, resp.Usuario.ID)
	}
}

func TestLoginUsuario_RechazaPasswordIncorrecta(t *testing.T) {
	db := requireTestDB(t)
	handler := LoginUsuario(db, duracionSesionDePrueba)

	usuario := crearUsuarioConCredenciales(t)

	req := loginRequest{Usuario: usuario.Username, Password: "esto-no-es-la-clave"}
	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/usuarios/login", req, nil, nil)
	esperarStatus(t, w, http.StatusUnauthorized)
}

// Mismo status y mismo mensaje que una contraseña incorrecta: no hay que
// filtrar si el problema fue el usuario o la contraseña.
func TestLoginUsuario_UsuarioInexistenteDaElMismoError(t *testing.T) {
	db := requireTestDB(t)
	handler := LoginUsuario(db, duracionSesionDePrueba)

	req := loginRequest{Usuario: "no-existe-" + idAleatorio(t), Password: "cualquier-cosa"}
	w, parsed := ejecutarJSON(t, handler, http.MethodPost, "/usuarios/login", req, nil, nil)
	esperarStatus(t, w, http.StatusUnauthorized)

	if parsed["error"] != "usuario o contraseña incorrectos" {
		t.Fatalf("esperaba el mensaje genérico, dio %v", parsed["error"])
	}
}

func TestLogoutUsuario_BorraLaSesion(t *testing.T) {
	db := requireTestDB(t)
	usuario := crearUsuarioConCredenciales(t)
	sesion, err := crearSesion(db, uuid.MustParse(usuario.ID), duracionSesionDePrueba)
	if err != nil {
		t.Fatalf("creando sesión de prueba: %v", err)
	}

	handler := LogoutUsuario(db)
	w := ejecutarConToken(t, handler, http.MethodPost, "/usuarios/logout", sesion.Token)
	esperarStatus(t, w, http.StatusNoContent)

	if sesionExisteEnBase(t, sesion.Token) {
		t.Fatalf("la sesión debería haberse borrado tras el logout")
	}
}

// ---------------------------------------------------------------------
// Perfil propio
// ---------------------------------------------------------------------

func TestObtenerMiPerfil_Exitoso(t *testing.T) {
	db := requireTestDB(t)
	usuario := crearUsuarioConCredenciales(t)
	handler := ObtenerMiPerfil(db)

	w, _ := ejecutarJSON(t, handler, http.MethodGet, "/usuarios/me", nil, nil, map[string]string{"usuarioID": usuario.ID})
	esperarStatus(t, w, http.StatusOK)

	resp := decodificarJSON[models.Usuario](t, w)
	if resp.Username != usuario.Username {
		t.Fatalf("esperaba username %q, dio %q", usuario.Username, resp.Username)
	}
}

func TestActualizarMiPerfil_ActualizaCamposYDejaEmailVacioComoNulo(t *testing.T) {
	db := requireTestDB(t)
	usuario := crearUsuarioConCredenciales(t)
	handler := ActualizarMiPerfil(db)

	req := actualizarPerfilRequest{Nombre: "Nombre Actualizado", Email: "", Telefono: "0981123456"}
	w, _ := ejecutarJSON(t, handler, http.MethodPatch, "/usuarios/me", req, nil, map[string]string{"usuarioID": usuario.ID})
	esperarStatus(t, w, http.StatusOK)

	resp := decodificarJSON[models.Usuario](t, w)
	if resp.Nombre != "Nombre Actualizado" {
		t.Fatalf("esperaba el nombre actualizado, dio %q", resp.Nombre)
	}
	if resp.Email != nil {
		t.Fatalf("un email vacío en el request debería quedar NULL, dio %q", *resp.Email)
	}
	if resp.Telefono == nil || *resp.Telefono != "0981123456" {
		t.Fatalf("esperaba el teléfono actualizado, dio %v", resp.Telefono)
	}
}

func TestActualizarMiPerfil_RechazaNombreVacio(t *testing.T) {
	db := requireTestDB(t)
	usuario := crearUsuarioConCredenciales(t)
	handler := ActualizarMiPerfil(db)

	req := actualizarPerfilRequest{Nombre: "   "} // solo espacios
	w, _ := ejecutarJSON(t, handler, http.MethodPatch, "/usuarios/me", req, nil, map[string]string{"usuarioID": usuario.ID})
	esperarStatus(t, w, http.StatusBadRequest)
}

func TestCambiarMiPassword_ExitosoCierraTodasLasSesiones(t *testing.T) {
	db := requireTestDB(t)
	usuario := crearUsuarioConCredenciales(t)
	uid := uuid.MustParse(usuario.ID)

	sesionA, err := crearSesion(db, uid, duracionSesionDePrueba)
	if err != nil {
		t.Fatalf("creando sesión A: %v", err)
	}
	sesionB, err := crearSesion(db, uid, duracionSesionDePrueba)
	if err != nil {
		t.Fatalf("creando sesión B: %v", err)
	}

	handler := CambiarMiPassword(db)
	req := cambiarPasswordUsuarioRequest{Actual: usuario.Password, Nueva: "unaClaveNuevaSegura456"}
	w, _ := ejecutarJSON(t, handler, http.MethodPatch, "/usuarios/me/password", req, nil, map[string]string{"usuarioID": usuario.ID})
	esperarStatus(t, w, http.StatusNoContent)

	if sesionExisteEnBase(t, sesionA.Token) || sesionExisteEnBase(t, sesionB.Token) {
		t.Fatalf("cambiar la contraseña debería cerrar TODAS las sesiones activas")
	}

	// Y el login con la clave nueva tiene que andar.
	login := LoginUsuario(db, duracionSesionDePrueba)
	wLogin, _ := ejecutarJSON(t, login, http.MethodPost, "/usuarios/login",
		loginRequest{Usuario: usuario.Username, Password: "unaClaveNuevaSegura456"}, nil, nil)
	esperarStatus(t, wLogin, http.StatusOK)
}

func TestCambiarMiPassword_RechazaActualIncorrecta(t *testing.T) {
	db := requireTestDB(t)
	usuario := crearUsuarioConCredenciales(t)

	handler := CambiarMiPassword(db)
	req := cambiarPasswordUsuarioRequest{Actual: "clave-incorrecta", Nueva: "unaClaveNuevaSegura456"}
	w, _ := ejecutarJSON(t, handler, http.MethodPatch, "/usuarios/me/password", req, nil, map[string]string{"usuarioID": usuario.ID})
	esperarStatus(t, w, http.StatusUnauthorized)

	// La clave original tiene que seguir andando: el rechazo no debería
	// haber tocado nada.
	login := LoginUsuario(db, duracionSesionDePrueba)
	wLogin, _ := ejecutarJSON(t, login, http.MethodPost, "/usuarios/login",
		loginRequest{Usuario: usuario.Username, Password: usuario.Password}, nil, nil)
	esperarStatus(t, wLogin, http.StatusOK)
}

// ---------------------------------------------------------------------
// Cuentas propias
// ---------------------------------------------------------------------

func TestListarMisCuentas_SoloDelUsuarioYDeEsteBanco(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := ListarMisCuentas(db, bancoID)

	usuarioID := crearUsuarioDePrueba(t)
	otroUsuarioID := crearUsuarioDePrueba(t)
	otroBancoID := "OTRO-BANCO-" + idAleatorio(t)

	crearCuentaDeUsuarioDirecta(t, bancoID, "CTA1", 1_000, "", usuarioID)
	crearCuentaDeUsuarioDirecta(t, bancoID, "CTA2", 2_000, "", usuarioID)
	crearCuentaDeUsuarioDirecta(t, bancoID, "CTA3", 3_000, "", otroUsuarioID) // de otro usuario
	crearCuentaDeUsuarioDirecta(t, otroBancoID, "CTA4", 4_000, "", usuarioID) // mismo usuario, otro banco

	w, _ := ejecutarJSON(t, handler, http.MethodGet, "/usuarios/me/cuentas", nil, nil, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w, http.StatusOK)

	cuentas := decodificarJSON[[]models.Cuenta](t, w)
	if len(cuentas) != 2 {
		t.Fatalf("esperaba 2 cuentas (solo las de este usuario en este banco), dio %d", len(cuentas))
	}
	for _, c := range cuentas {
		if c.NumeroCuenta != "CTA1" && c.NumeroCuenta != "CTA2" {
			t.Fatalf("cuenta inesperada en el listado: %+v", c)
		}
	}
}

func TestCambiarAliasCuenta_Exitoso(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := CambiarAliasCuenta(db, bancoID)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, bancoID, "CTA1", 0, "", usuarioID)

	params := gin.Params{{Key: "numero_cuenta", Value: "CTA1"}}
	req := cambiarAliasRequest{Alias: "mi.alias-01"}
	w, _ := ejecutarJSON(t, handler, http.MethodPatch, "/cuentas/CTA1/alias", req, params, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w, http.StatusOK)

	resp := decodificarJSON[models.Cuenta](t, w)
	if resp.Alias == nil || *resp.Alias != "mi.alias-01" {
		t.Fatalf("esperaba el alias actualizado, dio %v", resp.Alias)
	}
}

func TestCambiarAliasCuenta_RechazaFormatoInvalido(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := CambiarAliasCuenta(db, bancoID)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, bancoID, "CTA1", 0, "", usuarioID)

	params := gin.Params{{Key: "numero_cuenta", Value: "CTA1"}}
	req := cambiarAliasRequest{Alias: "AL"} // mayúsculas y muy corto
	w, _ := ejecutarJSON(t, handler, http.MethodPatch, "/cuentas/CTA1/alias", req, params, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w, http.StatusBadRequest)
}

// Mismo patrón de seguridad que las transferencias (punto 1): no alcanza
// con conocer el número de cuenta de otro para tocarle el alias.
func TestCambiarAliasCuenta_RechazaSiNoEsDueno(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := CambiarAliasCuenta(db, bancoID)

	dueño := crearUsuarioDePrueba(t)
	otro := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, bancoID, "CTA1", 0, "", dueño)

	params := gin.Params{{Key: "numero_cuenta", Value: "CTA1"}}
	req := cambiarAliasRequest{Alias: "intento-ajeno"}
	w, _ := ejecutarJSON(t, handler, http.MethodPatch, "/cuentas/CTA1/alias", req, params, map[string]string{"usuarioID": otro})
	esperarStatus(t, w, http.StatusNotFound)
}

func TestCambiarAliasCuenta_RechazaAliasDuplicado(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := CambiarAliasCuenta(db, bancoID)

	usuarioA := crearUsuarioDePrueba(t)
	usuarioB := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, bancoID, "CTA1", 0, "", usuarioA)
	crearCuentaDeUsuarioDirecta(t, bancoID, "CTA2", 0, "", usuarioB)

	paramsA := gin.Params{{Key: "numero_cuenta", Value: "CTA1"}}
	wA, _ := ejecutarJSON(t, handler, http.MethodPatch, "/cuentas/CTA1/alias",
		cambiarAliasRequest{Alias: "alias-compartido"}, paramsA, map[string]string{"usuarioID": usuarioA})
	esperarStatus(t, wA, http.StatusOK)

	paramsB := gin.Params{{Key: "numero_cuenta", Value: "CTA2"}}
	wB, _ := ejecutarJSON(t, handler, http.MethodPatch, "/cuentas/CTA2/alias",
		cambiarAliasRequest{Alias: "alias-compartido"}, paramsB, map[string]string{"usuarioID": usuarioB})
	esperarStatus(t, wB, http.StatusConflict)
}
