package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"github.com/sudameris/backend/internal/models"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// testDB es la conexión compartida para los tests de integración de este
// paquete (los que sí tocan la base). Queda nil si TEST_DATABASE_URL no
// está seteada; cada test que la necesita debe pedirla con
// requireTestDB(t), que hace t.Skip si no hay base disponible. Los tests
// que no tocan la base (como los de middleware_test.go) ni se enteran.
var testDB *sqlx.DB

func TestMain(m *testing.M) {
	// cmd/api/main.go carga .env con godotenv, pero "go test" nunca
	// ejecuta ese main() (cada paquete de test tiene su propio binario),
	// así que hace falta cargarlo acá también para poder setear
	// TEST_DATABASE_URL en el .env en vez de exportarla a mano en cada
	// sesión de shell. godotenv.Load no pisa una variable que ya esté
	// seteada en el entorno, así que un "export" manual sigue ganando.
	// El .env vive en la raíz de backend/, dos niveles arriba de este
	// paquete (internal/handlers).
	_ = godotenv.Load("../../.env")

	url := os.Getenv("TEST_DATABASE_URL")
	if url != "" {
		db, err := sqlx.Connect("pgx", url)
		if err != nil {
			fmt.Printf("TEST_DATABASE_URL está seteada pero no se pudo conectar: %v\n", err)
			os.Exit(1)
		}
		if err := db.Ping(); err != nil {
			fmt.Printf("TEST_DATABASE_URL está seteada pero el ping falló: %v\n", err)
			os.Exit(1)
		}
		testDB = db
	}

	code := m.Run()

	if testDB != nil {
		testDB.Close()
	}
	os.Exit(code)
}

// requireTestDB devuelve la conexión de test, o saltea el test actual si
// TEST_DATABASE_URL no está configurada. Ver backend/README.md, sección
// "Tests de integración de handlers", para cómo armar esa base.
func requireTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL no está seteada — se saltea este test de integración (necesita una CockroachDB real con las migraciones aplicadas)")
	}
	return testDB
}

// bancoIDDePrueba genera un banco_id único para este test, para que sus
// datos nunca choquen con los de otro test (aunque corran en paralelo) y
// no haga falta truncar tablas entre corridas.
func bancoIDDePrueba(t *testing.T) string {
	t.Helper()
	return "TEST-" + idAleatorio(t)
}

func idAleatorio(t *testing.T) string {
	t.Helper()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("generando id aleatorio: %v", err)
	}
	return hex.EncodeToString(b)
}

// uuidDePrueba genera un UUID real, para los campos que la base exige
// como UUID de verdad (idempotency_key, operacion_id) — a diferencia de
// idAleatorio, que alcanza para campos TEXT como banco_id o
// numero_cuenta, pero rompe si se usa donde la columna es UUID.
func uuidDePrueba(t *testing.T) string {
	t.Helper()
	return uuid.New().String()
}

// operacionDePrueba son los campos que hace falta fijar para armar un
// escenario de ListarMovimientos que no sale naturalmente de llamar a los
// handlers de transferencias -- en particular, una operación en un estado
// puntual (p.ej. un crédito entrante RECHAZADA) para probar que el filtro
// contra el "ingreso fantasma" sigue andando. Estado por defecto COMPLETADA.
type operacionDePrueba struct {
	Tipo                string
	BancoOrigen         string
	NumeroCuentaOrigen  string
	BancoDestino        string
	NumeroCuentaDestino string
	Importe             int64
	Estado              string
}

// crearOperacionDirecta inserta la fila por SQL directo contra la base de
// test principal (no sirve para simular algo que ocurrió del lado de un
// banco destino físicamente separado -- para eso ver mundoInterbancario).
func crearOperacionDirecta(t *testing.T, o operacionDePrueba) models.Operacion {
	t.Helper()
	db := requireTestDB(t)

	estado := o.Estado
	if estado == "" {
		estado = models.OperacionCompletada
	}

	var creada models.Operacion
	err := db.Get(&creada, `
		INSERT INTO operaciones (
			tipo, idempotency_key, banco_origen, numero_cuenta_origen,
			banco_destino, numero_cuenta_destino, importe, estado
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING *
	`, o.Tipo, uuidDePrueba(t), o.BancoOrigen, o.NumeroCuentaOrigen,
		o.BancoDestino, o.NumeroCuentaDestino, o.Importe, estado)
	if err != nil {
		t.Fatalf("creando operación de prueba: %v", err)
	}
	return creada
}

// passwordDePrueba es la contraseña en texto plano que usan los usuarios
// de prueba -- en la base solo vive hasheada, así que los tests que
// necesitan loguearse de verdad (ver usuarios_test.go) la necesitan aparte.
const passwordDePrueba = "clave-de-prueba"

// usuarioDePrueba junta lo que hace falta para operar como el usuario
// recién creado: su id (para asociarle cuentas, como ya hacía
// crearUsuarioDePrueba) y, además, su username y contraseña en texto
// plano para poder ejercitar LoginUsuario de punta a punta.
type usuarioDePrueba struct {
	ID       string
	Username string
	Password string
}

func crearUsuarioConCredenciales(t *testing.T) usuarioDePrueba {
	t.Helper()
	db := requireTestDB(t)

	hash, err := bcrypt.GenerateFromPassword([]byte(passwordDePrueba), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hasheando password de prueba: %v", err)
	}

	username := "usuario-" + idAleatorio(t)
	var id string
	err = db.Get(&id, `
		INSERT INTO usuarios (username, documento, nombre, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, username, idAleatorio(t), "Usuario de prueba", string(hash))
	if err != nil {
		t.Fatalf("creando usuario de prueba: %v", err)
	}
	return usuarioDePrueba{ID: id, Username: username, Password: passwordDePrueba}
}

// crearUsuarioDePrueba inserta un usuario de banca personal por SQL
// directo y devuelve su id (como string, que es como viaja en el
// contexto de gin: ver RequireSesion). Lo usan los tests de transferencias
// para armar una cuenta "ORIGEN" que de verdad le pertenezca a alguien,
// ya que TransferenciaInterna/TransferenciaInterbancaria ahora exigen
// sesión y verifican dueño. Para tests que necesitan loguearse de
// verdad (usuarios_test.go), ver crearUsuarioConCredenciales.
func crearUsuarioDePrueba(t *testing.T) string {
	t.Helper()
	return crearUsuarioConCredenciales(t).ID
}

// crearCuentaDeUsuarioDirecta es como crearCuentaDirecta, pero además
// asocia la cuenta a usuarioID (columna usuario_id). Se usa para la
// cuenta ORIGEN en los tests de transferencias: el handler ahora
// comprueba que quien pide la transferencia (el usuario de la sesión) sea
// el dueño de esa cuenta.
func crearCuentaDeUsuarioDirecta(t *testing.T, bancoID, numeroCuenta string, saldo int64, pin, usuarioID string) models.Cuenta {
	t.Helper()
	cuenta := crearCuentaDirecta(t, bancoID, numeroCuenta, saldo, pin)

	db := requireTestDB(t)
	if _, err := db.Exec(`UPDATE cuentas SET usuario_id = $1 WHERE id = $2`, usuarioID, cuenta.ID); err != nil {
		t.Fatalf("asociando cuenta de prueba a usuario: %v", err)
	}
	parsedID := uuid.MustParse(usuarioID)
	cuenta.UsuarioID = &parsedID
	return cuenta
}

// crearCuentaDirecta inserta una cuenta por SQL directo (sin pasar por el
// handler CrearCuenta), para que los tests de otros handlers no dependan
// de que ESE handler en particular funcione bien. pin puede ser "" para
// una cuenta sin PIN asignado. Usa la base de test principal; para la
// segunda base de los tests interbancarios, ver crearCuentaDirectaEn.
func crearCuentaDirecta(t *testing.T, bancoID, numeroCuenta string, saldo int64, pin string) models.Cuenta {
	t.Helper()
	return crearCuentaDirectaEn(t, requireTestDB(t), bancoID, numeroCuenta, saldo, pin)
}

func crearCuentaDirectaEn(t *testing.T, db *sqlx.DB, bancoID, numeroCuenta string, saldo int64, pin string) models.Cuenta {
	t.Helper()

	var pinHash *string
	if pin != "" {
		// bcrypt.MinCost: a los tests no les importa el costo real, solo
		// que corran rápido.
		hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.MinCost)
		if err != nil {
			t.Fatalf("hasheando pin de prueba: %v", err)
		}
		s := string(hash)
		pinHash = &s
	}

	var cuenta models.Cuenta
	err := db.Get(&cuenta, `
		INSERT INTO cuentas (banco_id, numero_cuenta, titular, saldo, pin_hash)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING *
	`, bancoID, numeroCuenta, "Titular de prueba", saldo, pinHash)
	if err != nil {
		t.Fatalf("creando cuenta de prueba (%s/%s): %v", bancoID, numeroCuenta, err)
	}
	return cuenta
}

// saldoActual consulta el saldo real de una cuenta en la base de test
// principal, para verificar el efecto económico de una operación (o la
// ausencia de efecto, en los casos de rechazo). Para la segunda base de
// los tests interbancarios, ver saldoActualEn.
func saldoActual(t *testing.T, bancoID, numeroCuenta string) int64 {
	t.Helper()
	return saldoActualEn(t, requireTestDB(t), bancoID, numeroCuenta)
}

func saldoActualEn(t *testing.T, db *sqlx.DB, bancoID, numeroCuenta string) int64 {
	t.Helper()

	var saldo int64
	err := db.Get(&saldo, `SELECT saldo FROM cuentas WHERE banco_id = $1 AND numero_cuenta = $2`, bancoID, numeroCuenta)
	if err != nil {
		t.Fatalf("consultando saldo de %s/%s: %v", bancoID, numeroCuenta, err)
	}
	return saldo
}

// ejecutarJSON arma un *gin.Context a mano (sin levantar el router
// completo) y llama al handler directamente. ctxVars simula lo que
// VerifyBankSignature dejaría seteado en el contexto (típicamente
// "bankID"), para testear el handler de negocio sin tener que firmar una
// solicitud real en cada caso.
func ejecutarJSON(t *testing.T, handler gin.HandlerFunc, method, path string, body any, params gin.Params, ctxVars map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("serializando body de prueba: %v", err)
		}
		reader = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = params
	for k, v := range ctxVars {
		c.Set(k, v)
	}

	handler(c)
	// Sin esto, un handler que solo hace c.Status(204) sin escribir body
	// (LogoutUsuario, CambiarMiPassword) queda con w.Code en 200: gin solo
	// escribe el status real a un ResponseWriter la primera vez que algo
	// escribe el body (ver responseWriter.WriteHeaderNow en el propio gin),
	// y ese flush normalmente lo dispara el motor de gin al final de
	// engine.ServeHTTP -- acá no pasamos por ahí, llamamos al handler
	// directo, así que lo disparamos nosotros. Es un no-op si ya se
	// escribió un body (c.JSON ya lo dispara solo).
	c.Writer.WriteHeaderNow()

	var parsed map[string]any
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
			// La respuesta puede ser JSON válido pero no un objeto en el
			// nivel superior (p.ej. ListarMisCuentas devuelve un array
			// directo). Solo es un error de verdad si ni siquiera es JSON
			// válido; si el caller necesita el array, que decodifique con
			// decodificarJSON[T] sobre el *ResponseRecorder directamente.
			var cualquiera any
			if err2 := json.Unmarshal(w.Body.Bytes(), &cualquiera); err2 != nil {
				t.Fatalf("la respuesta no es JSON válido: %v\nbody: %s", err, w.Body.String())
			}
		}
	}
	return w, parsed
}

// esperarStatus es un atajo para el chequeo que se repite en casi todos
// los tests de handlers.
func esperarStatus(t *testing.T, w *httptest.ResponseRecorder, esperado int) {
	t.Helper()
	if w.Code != esperado {
		t.Fatalf("esperaba status %d, dio %d: %s", esperado, w.Code, w.Body.String())
	}
}

// segundaBase es una conexión a una base de test SEPARADA de testDB, que
// usan los tests interbancarios para simular el banco "destino". Hace
// falta que sea una base físicamente distinta -- no solo otro banco_id
// en la misma tabla -- porque el modelo de datos del protocolo (doc 02)
// asume que cada banco tiene su propia base: el fast path de idempotencia
// de RecibirCredito busca por (banco_origen, idempotency_key) sin filtrar
// por banco_destino, así que si "origen" y "destino" comparten tabla,
// destino encuentra la fila que el propio origen acaba de crearse a sí
// mismo (mismo idempotency_key, por diseño del protocolo) y la confunde
// con una respuesta ya resuelta. En producción esto nunca pasa porque las
// bases están separadas.
var (
	segundaBaseUnaVez sync.Once
	segundaBaseConn   *sqlx.DB
	segundaBaseErr    error
)

func segundaBaseDePrueba(t *testing.T) *sqlx.DB {
	t.Helper()
	requireTestDB(t) // asegura el skip estándar si no hay TEST_DATABASE_URL

	segundaBaseUnaVez.Do(func() {
		segundaBaseConn, segundaBaseErr = crearYMigrarSegundaBase()
	})
	if segundaBaseErr != nil {
		t.Fatalf("preparando la segunda base para tests interbancarios: %v", segundaBaseErr)
	}
	return segundaBaseConn
}

func crearYMigrarSegundaBase() (*sqlx.DB, error) {
	rawURL := os.Getenv("TEST_DATABASE_URL")
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parseando TEST_DATABASE_URL: %w", err)
	}
	origName := strings.TrimPrefix(u.Path, "/")
	if origName == "" {
		return nil, fmt.Errorf("TEST_DATABASE_URL no tiene nombre de base en la ruta")
	}
	segundoNombre := origName + "_banco2"

	// Hace falta estar conectado a ALGUNA base para poder crear/borrar
	// otra; la original sirve.
	admin, err := sqlx.Connect("pgx", rawURL)
	if err != nil {
		return nil, fmt.Errorf("conectando para preparar la segunda base: %w", err)
	}
	defer admin.Close()

	// aplicarMigraciones no rastrea qué migraciones ya se aplicaron (no
	// hay una tabla schema_migrations como la de golang-migrate): corre
	// los .up.sql tal cual, una sola vez. Si la base quedara de una
	// corrida anterior del binario de test, una migración no aditiva
	// (como el rename de constraint de la 0003) fallaría la segunda vez.
	// Para evitar eso, arrancamos siempre de una base nueva.
	if _, err := admin.Exec("DROP DATABASE IF EXISTS " + segundoNombre); err != nil {
		return nil, fmt.Errorf("borrando la segunda base de una corrida anterior: %w", err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + segundoNombre); err != nil {
		return nil, fmt.Errorf("creando la base %q: %w", segundoNombre, err)
	}

	u2 := *u
	u2.Path = "/" + segundoNombre
	db2, err := sqlx.Connect("pgx", u2.String())
	if err != nil {
		return nil, fmt.Errorf("conectando a la segunda base: %w", err)
	}
	if err := db2.Ping(); err != nil {
		db2.Close()
		return nil, fmt.Errorf("ping a la segunda base: %w", err)
	}

	// gen_random_uuid() (usado en las migraciones) viene de pgcrypto en
	// Postgres; en CockroachDB esto es un no-op aceptado por
	// compatibilidad (la función ya está disponible nativamente), así
	// que correrlo ahí no cambia nada.
	if _, err := db2.Exec(`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`); err != nil {
		db2.Close()
		return nil, fmt.Errorf("habilitando pgcrypto en la segunda base: %w", err)
	}

	if err := aplicarMigraciones(db2); err != nil {
		db2.Close()
		return nil, err
	}
	return db2, nil
}

// aplicarMigraciones corre los .up.sql del repo tal cual están, contra
// db, para que la segunda base tenga exactamente el mismo esquema que la
// principal sin duplicar la definición de las tablas acá. Es idempotente:
// si ya se aplicaron (de una corrida anterior contra la misma base
// persistente), los "already exists" se ignoran.
func aplicarMigraciones(db *sqlx.DB) error {
	archivos, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil {
		return fmt.Errorf("listando migraciones: %w", err)
	}
	if len(archivos) == 0 {
		return fmt.Errorf("no se encontraron migraciones en ../../migrations (¿go test se está corriendo desde otro directorio?)")
	}
	sort.Strings(archivos)

	for _, f := range archivos {
		contenido, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("leyendo %s: %w", f, err)
		}
		for _, stmt := range strings.Split(quitarComentariosSQL(string(contenido)), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := db.Exec(stmt); err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "already exists") {
					continue
				}
				return fmt.Errorf("aplicando %s:\n%s\nerror: %w", f, stmt, err)
			}
		}
	}
	return nil
}

// quitarComentariosSQL borra todo lo que va de "--" al fin de línea, para
// que aplicarMigraciones pueda partir el archivo por ";" sin que un ";"
// adentro de un comentario (como en 0006_ampliar_cuentas_banca_personal,
// "...numero_cuenta; esto es...") corte la sentencia en un lugar
// equivocado y deje el resto del comentario como si fuera SQL de verdad.
// Asume que ninguna migración tiene un "--" adentro de un string literal
// (hoy no lo hay; si llegara a hacer falta, esto necesitaría ser
// consciente de comillas).
func quitarComentariosSQL(sql string) string {
	lineas := strings.Split(sql, "\n")
	for i, linea := range lineas {
		if idx := strings.Index(linea, "--"); idx != -1 {
			lineas[i] = linea[:idx]
		}
	}
	return strings.Join(lineas, "\n")
}
