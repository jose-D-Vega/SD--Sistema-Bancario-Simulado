# Backend — Banco Sudameris

Backend del banco Sudameris para el proyecto de Sistemas Distribuidos.

## Stack

- **Go** 1.25 (o la versión que diga tu `go.mod`)
- **Gin** — framework HTTP
- **CockroachDB** — base de datos (protocolo compatible con Postgres)
- **sqlx / pgx** — acceso a datos
- **golang-migrate** — migraciones de esquema
- **crypto/ed25519** (stdlib) — firma y verificación entre bancos

## Cómo levantarlo con Docker (recomendado si no vas a tocar el código Go)

Para alguien del equipo que solo necesita que el backend esté corriendo
(frontend, pruebas manuales, etc.) y no quiere instalar Go ni `golang-migrate`:
solo hace falta tener **Docker** instalado.

```bash
cp .env.example .env
# completar BANCO_PRIVATE_KEY (ver sección "Generar tu clave Ed25519" más abajo)

# si todavía no existe, crear un registro vacío (se puede completar después)
[ -f registro_bancos.json ] || echo '{}' > registro_bancos.json

make docker-up
```

Esto levanta CockroachDB, crea la base, corre las migraciones, y compila y
levanta el backend — todo en contenedores, sin instalar nada más. Quedan
escuchando en `localhost:8080` (backend) y `localhost:26257` (base).

```bash
make docker-logs    # ver logs del backend
make docker-rebuild  # después de bajar código nuevo, para reconstruir la imagen
make docker-down    # apagar todo
```

Si editás código y querés ver los cambios reflejados, corré `make docker-rebuild`
(no hace falta bajar todo con `docker-down` para eso).

## Si algún puerto ya está ocupado en tu máquina

Los puertos que usa este proyecto por defecto: `8080` (backend), `26257` y
`8090` (CockroachDB), `8091` (backend-bco-test, solo con `--profile testing`).

1. Confirmá qué está usando el puerto: `sudo lsof -i :8080` (cambiando el
   número). Si es un sobrante de este mismo proyecto (`go run` suelto, un
   `docker compose up` que no bajaste), cerralo (`docker compose down`,
   `pkill -f "go run ./cmd/api"`) en vez de cambiar nada.
2. Si es otro programa sin relación, copiá
   `docker-compose.override.yml.example` a `docker-compose.override.yml`
   (este último está en `.gitignore`, cada uno tiene el suyo) y cambiá ahí
   el número de puerto de tu lado (el de la izquierda en `"HOST:CONTENEDOR"`)
   — nunca el de la derecha, que es el que usan los contenedores para
   hablarse entre sí. `docker compose up` lo toma automáticamente, sin
   flags extra.

## Requisitos para desarrollar el código Go

- Go 1.25+ (o la versión que diga tu `go.mod`)
- Docker (para levantar CockroachDB localmente)
- [golang-migrate](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate) instalado (para correr migraciones)

## Cómo levantar el entorno de desarrollo (sin Docker para el backend)

```bash
# 1. Copiar variables de entorno
cp .env.example .env
# completar BANCO_PRIVATE_KEY con tu clave privada en base64

# 2. Instalar golang-migrate con el driver de cockroachdb (una sola vez)
go install -tags 'cockroachdb' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# 3. Levantar CockroachDB
make db-up

# 4. Crear la base (CockroachDB en modo --insecure no la crea sola)
docker exec -it sudameris-cockroachdb ./cockroach sql --insecure \
  -e "CREATE DATABASE IF NOT EXISTS sudameris;"

# 5. Correr las migraciones (ojo: MIGRATE_DATABASE_URL, no DATABASE_URL)
export MIGRATE_DATABASE_URL="cockroachdb://root@localhost:26257/sudameris?sslmode=disable"
make migrate-up

# 6. Instalar dependencias de Go
make tidy

# 7. Levantar el backend (lee DATABASE_URL desde .env, con esquema postgresql://)
make run
```

El backend queda escuchando en `http://localhost:8080`. Podés probar que
esté vivo con:

```bash
curl http://localhost:8080/health
```

## Estructura

```
backend/
├── cmd/api/            # entrypoint (main.go)
├── internal/
│   ├── config/          # carga de variables de entorno y del registro de bancos
│   ├── crypto/           # firma/verificación Ed25519 (doc 02)
│   ├── db/                # conexión a CockroachDB
│   ├── handlers/          # endpoints HTTP, middlewares y catálogo de errores
│   ├── interbanco/        # cliente HTTP saliente hacia otros bancos (doc 02)
│   └── models/            # structs de dominio (Cuenta, Operacion)
├── migrations/          # esquema de base, versionado con golang-migrate
├── scripts/             # scripts de prueba manual (curl + firma Ed25519)
├── Dockerfile           # imagen del backend
├── docker-compose.yml   # CockroachDB + migraciones + backend (+ un segundo
│                         # banco de prueba con --profile testing)
└── .env.example
```

## Estado actual

### Implementado

- **Cuentas**: creación (`POST /cuentas`, con PIN opcional hasheado con
  bcrypt), cambio de estado (`PATCH /cuentas/:numero_cuenta/estado`) y
  consulta (`GET /cuentas/:numero_cuenta`, firmada — la usan otros
  bancos y cajeros).
- **Transferencias internas** (`POST /transferencias/internas`, doc 01 §5.1):
  transacción local con locking ordenado de cuentas (evita deadlocks),
  validación de límites y saldo, e idempotencia con detección de
  conflicto si se reintenta la misma clave con datos distintos.
- **Transferencias interbancarias salientes** (`POST /transferencias/interbancarias`,
  doc 02 §4): reserva en origen → llamada firmada al destino →
  confirmación o reversa según la respuesta. Ante timeout, consulta el
  estado en vez de asumir un resultado, y si tampoco hay respuesta deja
  la operación en `PENDIENTE` con reconciliación automática en
  background (configurable con `RECONCILIACION_INTERVALO_SEGUNDOS`) y
  un endpoint manual (`POST /transferencias/interbancarias/:idempotency_key/reconciliar`)
  para forzarla.
- **Recepción de créditos interbancarios** (`POST /interbancario/creditos`,
  `GET /interbancario/creditos/:transfer_id`): valida, acredita de forma
  atómica, e idempotencia con chequeo de conflicto si cambian los datos.
- **Cajero** (`POST /cajero/autorizaciones`, `GET /cajero/autorizaciones/:operacion_id`,
  doc 03): valida PIN (bcrypt), estado de cuenta, límites y saldo; debita
  de forma atómica; idempotente.
- **Firma Ed25519 entre bancos** (doc 02 §1): módulo `internal/crypto` +
  middleware `VerifyBankSignature`, que valida headers, ventana de
  `X-Timestamp` y la firma contra la cadena canónica.
- **Catálogo de errores** del doc 02 §6, con su código HTTP correspondiente.

### Pendiente / fuera de este backend

- `frontend/` y `cajero/` (la interfaz del cajero) están vacíos
  todavía — lo hecho hasta ahora es solo este backend.
- No hay tests automatizados (`go test`); las pruebas son manuales, con
  los scripts de `scripts/` firmando requests a mano con `curl` +
  `openssl`.
- Los 20 casos del plan de pruebas (documento 04) todavía no se
  ejecutaron de punta a punta contra los bancos de los otros 3 grupos —
  solo contra el segundo backend de prueba local (`backend-bco-test`).
