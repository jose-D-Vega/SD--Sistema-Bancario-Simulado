#!/usr/bin/env bash
# Crea un set fijo de cuentas de prueba contra POST /cuentas (endpoint de
# back office del banco, protegido con ADMIN_API_KEY en vez de firma --
# ver handlers.RequireAdminKey). Pensado para correr después de
# "docker compose up" con el volumen vacío, en SUDAMERIS o en BCO_TEST
# según el HOST que le pases.
#
# Uso:
#   ADMIN_API_KEY=... ./scripts/seed_cuentas.sh                                  # SUDAMERIS (:8080)
#   ADMIN_API_KEY=... HOST=http://localhost:8091 ./scripts/seed_cuentas.sh       # BCO_TEST (:8091, usar BCO_TEST_ADMIN_API_KEY del .env)
set -euo pipefail

HOST="${HOST:-http://localhost:8080}"

if [[ -z "${ADMIN_API_KEY:-}" ]]; then
  echo "Falta ADMIN_API_KEY (tiene que matchear la que tenga configurada el backend en $HOST)." >&2
  exit 1
fi

# numero_cuenta|titular|saldo_inicial|pin
CUENTAS=(
  "001-000001|Cuenta de prueba 1|500000|1234"
  "001-000002|Cuenta de prueba 2|1000000|5678"
  "001-000003|Cuenta sin fondos|0|1111"
  "999-000001|Cuenta origen (otro banco)|2000000|4321"
)

for linea in "${CUENTAS[@]}"; do
  IFS='|' read -r numero_cuenta titular saldo pin <<< "$linea"
  BODY=$(printf '{"numero_cuenta":"%s","titular":"%s","saldo_inicial":%s,"pin":"%s"}' \
    "$numero_cuenta" "$titular" "$saldo" "$pin")
  echo "--- $numero_cuenta ---"
  curl -s -X POST "$HOST/cuentas" -H "Content-Type: application/json" -H "X-Admin-Key: $ADMIN_API_KEY" -d "$BODY"
  echo
done

# Una cuenta bloqueada, para probar CUENTA_INACTIVA sin tener que tocarla a mano.
echo "--- 001-000004 (bloqueada) ---"
curl -s -X POST "$HOST/cuentas" -H "Content-Type: application/json" -H "X-Admin-Key: $ADMIN_API_KEY" -d \
  '{"numero_cuenta":"001-000004","titular":"Cuenta bloqueada","saldo_inicial":100000,"pin":"0000"}'
echo
curl -s -X PATCH "$HOST/cuentas/001-000004/estado" -H "Content-Type: application/json" -H "X-Admin-Key: $ADMIN_API_KEY" -d \
  '{"estado":"BLOQUEADA"}'
echo
