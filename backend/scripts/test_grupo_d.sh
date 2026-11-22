#!/bin/bash
set -e
BASE="http://localhost:8080"

echo "=== 1. Crear dos usuarios frescos para no pisar datos que ya tengas ==="
SUFIJO=$(date +%s)
REG_A=$(curl -s -X POST $BASE/usuarios/registro -H "Content-Type: application/json" -d "{
  \"username\":\"test_movA_$SUFIJO\",\"documento\":\"9${SUFIJO}1\",\"nombre\":\"Test Movimientos A\",\"password\":\"clave12345\"
}")
REG_B=$(curl -s -X POST $BASE/usuarios/registro -H "Content-Type: application/json" -d "{
  \"username\":\"test_movB_$SUFIJO\",\"documento\":\"9${SUFIJO}2\",\"nombre\":\"Test Movimientos B\",\"password\":\"clave12345\"
}")

TOKEN_A=$(echo $REG_A | python3 -c "import json,sys;print(json.load(sys.stdin)['token'])")
CUENTA_A=$(echo $REG_A | python3 -c "import json,sys;print(json.load(sys.stdin)['cuenta']['numero_cuenta'])")
TOKEN_B=$(echo $REG_B | python3 -c "import json,sys;print(json.load(sys.stdin)['token'])")
CUENTA_B=$(echo $REG_B | python3 -c "import json,sys;print(json.load(sys.stdin)['cuenta']['numero_cuenta'])")
echo "A=$CUENTA_A  B=$CUENTA_B"

echo "=== 2. Historial vacío antes de cualquier movimiento ==="
curl -s "$BASE/cuentas/$CUENTA_A/movimientos" -H "Authorization: Bearer $TOKEN_A"
echo
echo "  -> esperado: {\"movimientos\":[],\"siguiente_cursor\":null}"

echo "=== 3. Generar 3 transferencias internas A -> B (para tener algo que paginar) ==="
# A arranca en saldo 0: hace falta fondearla a mano para esta prueba.
echo "  (si la transferencia de abajo da FONDOS_INSUFICIENTES, fondeá la cuenta $CUENTA_A antes de seguir)"
for i in 1 2 3; do
  curl -s -X POST $BASE/transferencias/internas -H "Content-Type: application/json" -d "{
    \"numero_cuenta_origen\":\"$CUENTA_A\",\"numero_cuenta_destino\":\"$CUENTA_B\",
    \"importe\":1000,\"idempotency_key\":\"$(python3 -c 'import uuid;print(uuid.uuid4())')\"
  }" | python3 -c "import json,sys;d=json.load(sys.stdin);print(' ->', d.get('estado', d))"
done

echo "=== 4. Historial de A: deben verse las 3, como SALIENTE ==="
curl -s "$BASE/cuentas/$CUENTA_A/movimientos" -H "Authorization: Bearer $TOKEN_A" | python3 -m json.tool

echo "=== 5. Historial de B: la misma plata, pero como ENTRANTE ==="
curl -s "$BASE/cuentas/$CUENTA_B/movimientos" -H "Authorization: Bearer $TOKEN_B" | python3 -c "
import json,sys
d = json.load(sys.stdin)
for m in d['movimientos']:
    print(m['tipo'], m['direccion'], m['importe'])
"
echo "  -> esperado: las 3 como TRANSFERENCIA_INTERNA ENTRANTE 1000"

echo "=== 6. Paginación: limite=2 no debe repetir ni saltear nada ==="
PAGINA1=$(curl -s "$BASE/cuentas/$CUENTA_A/movimientos?limite=2" -H "Authorization: Bearer $TOKEN_A")
echo $PAGINA1 | python3 -c "import json,sys;d=json.load(sys.stdin);print('página 1:', [m['id'][:8] for m in d['movimientos']], '| hay siguiente:', d['siguiente_cursor'] is not None)"
CURSOR=$(echo $PAGINA1 | python3 -c "import json,sys;print(json.load(sys.stdin)['siguiente_cursor'])")
curl -s "$BASE/cuentas/$CUENTA_A/movimientos?limite=2&cursor=$CURSOR" -H "Authorization: Bearer $TOKEN_A" | python3 -c "import json,sys;d=json.load(sys.stdin);print('página 2:', [m['id'][:8] for m in d['movimientos']])"
echo "  -> los ids de la página 1 y la página 2 no deben repetirse entre sí"

echo "=== 7. Filtro por tipo ==="
curl -s "$BASE/cuentas/$CUENTA_A/movimientos?tipo=TRANSFERENCIA_INTERNA" -H "Authorization: Bearer $TOKEN_A" | python3 -c "import json,sys;print(len(json.load(sys.stdin)['movimientos']), 'resultado(s)')"
echo "  -> esperado: 3"
curl -s "$BASE/cuentas/$CUENTA_A/movimientos?tipo=RETIRO_CAJERO" -H "Authorization: Bearer $TOKEN_A" | python3 -c "import json,sys;print(len(json.load(sys.stdin)['movimientos']), 'resultado(s)')"
echo "  -> esperado: 0"

echo "=== 8. Seguridad: A NO debe poder ver el historial de B ==="
curl -s -o /dev/null -w "%{http_code}\n" "$BASE/cuentas/$CUENTA_B/movimientos" -H "Authorization: Bearer $TOKEN_A"
echo "  -> esperado: 404"

echo "=== 9. Sin token ==="
curl -s -o /dev/null -w "%{http_code}\n" "$BASE/cuentas/$CUENTA_A/movimientos"
echo "  -> esperado: 401"

echo "=== 10. Cuenta inexistente con sesión válida ==="
curl -s -o /dev/null -w "%{http_code}\n" "$BASE/cuentas/000000000/movimientos" -H "Authorization: Bearer $TOKEN_A"
echo "  -> esperado: 404"
