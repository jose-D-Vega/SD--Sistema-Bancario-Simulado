#!/usr/bin/env bash
# Simula que un cajero (de este banco o de cualquier otro) pide
# autorización de retiro vía POST /cajero/autorizaciones, firmando el
# request con una clave Ed25519 (doc 02 + doc 03). Mismo mecanismo que
# test_credito.sh, adaptado a los campos del cajero.
set -euo pipefail

HOST="${HOST:-http://localhost:8080}"
BANK_ID="${BANK_ID:-SUDAMERIS}"
PRIVATE_KEY_PEM="${PRIVATE_KEY_PEM:-privada.pem}"
BANCO_CUENTA="${BANCO_CUENTA:-SUDAMERIS}"
NUMERO_CUENTA="${NUMERO_CUENTA:?Falta NUMERO_CUENTA (una cuenta real, con PIN asignado)}"
PIN="${PIN:?Falta PIN}"
IMPORTE="${IMPORTE:-100000}"

METODO="POST"
RUTA="/cajero/autorizaciones"
OPERACION_ID=$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid)
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

BODY=$(printf '{"operacion_id":"%s","banco_cuenta":"%s","numero_cuenta":"%s","pin":"%s","importe":%s,"moneda":"PYG"}' \
  "$OPERACION_ID" "$BANCO_CUENTA" "$NUMERO_CUENTA" "$PIN" "$IMPORTE")

CANONICAL=$(printf '%s\n%s\n%s\n%s\n%s\n%s' "$METODO" "$RUTA" "$BANK_ID" "$OPERACION_ID" "$TIMESTAMP" "$BODY")

CANON_FILE=$(mktemp)
printf '%s' "$CANONICAL" > "$CANON_FILE"
SIGNATURE=$(openssl pkeyutl -sign -inkey "$PRIVATE_KEY_PEM" -rawin -in "$CANON_FILE" | base64 -w0)
rm -f "$CANON_FILE"

echo "--- request ---"
echo "banco_id (cajero): $BANK_ID"
echo "operacion_id:      $OPERACION_ID"
echo "timestamp:          $TIMESTAMP"
echo "body:               $BODY"
echo

curl -i -X POST "$HOST$RUTA" \
  -H "Content-Type: application/json" \
  -H "X-Bank-Id: $BANK_ID" \
  -H "X-Timestamp: $TIMESTAMP" \
  -H "X-Idempotency-Key: $OPERACION_ID" \
  -H "X-Signature: $SIGNATURE" \
  -d "$BODY"
echo
