#!/usr/bin/env bash
# Simula que otro banco nos llama a POST /interbancario/creditos, firmando
# el request con una clave Ed25519 (doc 02). Pensado para probar el
# middleware y el handler sin depender de que otro grupo tenga su backend
# levantado: firmás con tu propia privada.pem y registrás tu propia
# publica.pem bajo un banco_id de prueba (ver instrucciones de uso abajo).
set -euo pipefail

HOST="${HOST:-http://localhost:8080}"
BANK_ID="${BANK_ID:-BCO_TEST}"
PRIVATE_KEY_PEM="${PRIVATE_KEY_PEM:-privada.pem}"
CUENTA_ORIGEN="${CUENTA_ORIGEN:-999-000001}"
CUENTA_DESTINO="${CUENTA_DESTINO:?Falta CUENTA_DESTINO (una cuenta real de tu banco, ej. 001-123456)}"
IMPORTE="${IMPORTE:-10000}"

METODO="POST"
RUTA="/interbancario/creditos"
TRANSFER_ID=$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid)
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

BODY=$(printf '{"transfer_id":"%s","cuenta_destino":"%s","banco_origen":"%s","cuenta_origen":"%s","importe":%s,"moneda":"PYG","concepto":"prueba manual"}' \
  "$TRANSFER_ID" "$CUENTA_DESTINO" "$BANK_ID" "$CUENTA_ORIGEN" "$IMPORTE")

CANONICAL=$(printf '%s\n%s\n%s\n%s\n%s\n%s' "$METODO" "$RUTA" "$BANK_ID" "$TRANSFER_ID" "$TIMESTAMP" "$BODY")

CANON_FILE=$(mktemp)
printf '%s' "$CANONICAL" > "$CANON_FILE"
SIGNATURE=$(openssl pkeyutl -sign -inkey "$PRIVATE_KEY_PEM" -rawin -in "$CANON_FILE" | base64 -w0)
rm -f "$CANON_FILE"

echo "--- request ---"
echo "banco_id (simulado): $BANK_ID"
echo "transfer_id:         $TRANSFER_ID"
echo "timestamp:            $TIMESTAMP"
echo "body:                 $BODY"
echo

curl -i -X POST "$HOST$RUTA" \
  -H "Content-Type: application/json" \
  -H "X-Bank-Id: $BANK_ID" \
  -H "X-Timestamp: $TIMESTAMP" \
  -H "X-Idempotency-Key: $TRANSFER_ID" \
  -H "X-Signature: $SIGNATURE" \
  -d "$BODY"
echo
