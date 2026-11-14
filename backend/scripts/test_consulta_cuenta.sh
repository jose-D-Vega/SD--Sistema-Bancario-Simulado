#!/usr/bin/env bash
# Prueba GET /cuentas/{numero_cuenta} firmado, igual que test_credito.sh y
# test_cajero.sh pero sin body (ahora requiere firma: antes no pasaba por
# VerifyBankSignature).
set -euo pipefail

HOST="${HOST:-http://localhost:8080}"
BANK_ID="${BANK_ID:-BCO_TEST}"
PRIVATE_KEY_PEM="${PRIVATE_KEY_PEM:-privada.pem}"
NUMERO_CUENTA="${NUMERO_CUENTA:?Falta NUMERO_CUENTA (ej. 001-123456)}"

METODO="GET"
RUTA="/cuentas/$NUMERO_CUENTA"
BODY=""
IDEMPOTENCY_KEY=$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid)
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Importante: NO armar la cadena canónica con CANONICAL=$(printf ...) y
# después volcarla a un archivo. La sustitución de comandos $(...) de bash
# elimina los saltos de línea finales del resultado, y como BODY va vacío
# en un GET, el \n final (el que separa X-Timestamp del body vacío) se
# pierde ahí. El servidor sí lo conserva (hace strings.Join con 6 partes,
# incluyendo el body vacío), así que la firma no verifica. Por eso se
# escribe directo al archivo con printf, sin pasar por una variable.
CANON_FILE=$(mktemp)
printf '%s\n%s\n%s\n%s\n%s\n%s' "$METODO" "$RUTA" "$BANK_ID" "$IDEMPOTENCY_KEY" "$TIMESTAMP" "$BODY" > "$CANON_FILE"
SIGNATURE=$(openssl pkeyutl -sign -inkey "$PRIVATE_KEY_PEM" -rawin -in "$CANON_FILE" | base64 -w0)
rm -f "$CANON_FILE"

echo "--- request ---"
echo "banco_id (simulado): $BANK_ID"
echo "ruta:                 $RUTA"
echo "timestamp:            $TIMESTAMP"
echo

curl -i -X GET "$HOST$RUTA" \
  -H "X-Bank-Id: $BANK_ID" \
  -H "X-Timestamp: $TIMESTAMP" \
  -H "X-Idempotency-Key: $IDEMPOTENCY_KEY" \
  -H "X-Signature: $SIGNATURE"
echo
