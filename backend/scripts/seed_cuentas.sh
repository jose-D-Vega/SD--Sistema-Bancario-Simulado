#!/usr/bin/env bash
# Crea un set fijo de cuentas de prueba contra POST /cuentas (sin firma,
# es el endpoint interno del banco). Pensado para correr después de
# "docker compose up" con el volumen vacío, en SUDAMERIS o en BCO_TEST
# según el HOST que le pases.
#
# Uso:
#   ./scripts/seed_cuentas.sh                              # SUDAMERIS (:8080)
#   HOST=http://localhost:8091 ./scripts/seed_cuentas.sh    # BCO_TEST (:8091)
set -euo pipefail

HOST="${HOST:-http://localhost:8080}"

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
  curl -s -X POST "$HOST/cuentas" -H "Content-Type: application/json" -d "$BODY"
  echo
done

# Una cuenta bloqueada, para probar CUENTA_INACTIVA sin tener que tocarla a mano.
echo "--- 001-000004 (bloqueada) ---"
curl -s -X POST "$HOST/cuentas" -H "Content-Type: application/json" -d \
  '{"numero_cuenta":"001-000004","titular":"Cuenta bloqueada","saldo_inicial":100000,"pin":"0000"}'
echo
curl -s -X PATCH "$HOST/cuentas/001-000004/estado" -H "Content-Type: application/json" -d \
  '{"estado":"BLOQUEADA"}'
echo
