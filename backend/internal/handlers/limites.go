package handlers

import "github.com/sudameris/backend/internal/models"

// Límites del doc 01, sección 4.
const (
	LimiteMaximoTransferencia = 10_000_000
	LimiteMaximoRetiroCajero  = 1_500_000
	LimiteDiarioCombinado     = 10_000_000
)

// getter lo satisfacen tanto *sqlx.DB como *sqlx.Tx.
type getter interface {
	Get(dest interface{}, query string, args ...interface{}) error
}

// esDuenoDeCuenta verifica que numeroCuenta, en este banco, le pertenezca
// al usuario logueado (usuarioID). La usa toda operación de banca personal
// donde el usuario actúa sobre "su" cuenta: transferencias (como origen),
// movimientos, cambio de alias. No reemplaza al contrato del BCP (ahí no
// hay usuario humano, solo bancos autenticándose entre sí con firma).
func esDuenoDeCuenta(q getter, bancoID, numeroCuenta, usuarioID string) (bool, error) {
	// Sin usuarioID no hay sesión que verificar: nunca es dueño. Cortamos
	// acá antes de ir a la base porque usuario_id es UUID en la tabla, y
	// compararlo contra "" haría fallar el cast en vez de simplemente dar
	// "no coincide" -- con RequireSesion de por medio esto no debería
	// pasar nunca en producción, pero lo dejamos a prueba de fallos.
	if usuarioID == "" {
		return false, nil
	}
	var esDueno bool
	err := q.Get(&esDueno, `
		SELECT EXISTS(
			SELECT 1 FROM cuentas WHERE banco_id = $1 AND numero_cuenta = $2 AND usuario_id = $3
		)
	`, bancoID, numeroCuenta, usuarioID)
	return esDueno, err
}

// totalSalidoHoy suma transferencias salientes (internas + interbancarias)
// y retiros de cajero ya COMPLETADOS de una cuenta, desde las 00:00 hora
// de Paraguay (doc 01, sección 4). Para transferencias, la cuenta es
// nuestra si banco_origen = bancoID; para retiros de cajero, la cuenta es
// nuestra si banco_destino = bancoID (el cajero puede ser de otro banco).
func totalSalidoHoy(q getter, bancoID, numeroCuenta string) (int64, error) {
	var total int64
	err := q.Get(&total, `
		SELECT COALESCE(SUM(importe), 0) FROM operaciones
		WHERE numero_cuenta_origen = $1
		  AND estado = $2
		  AND creado_en >= (date_trunc('day', now() AT TIME ZONE 'America/Asuncion') AT TIME ZONE 'America/Asuncion')
		  AND (
		    (banco_origen = $3 AND tipo IN ($4, $5))
		    OR (banco_destino = $3 AND tipo = $6)
		  )
	`, numeroCuenta, models.OperacionCompletada, bancoID,
		models.TipoTransferenciaInterna, models.TipoTransferenciaInterbancaria, models.TipoRetiroCajero)
	return total, err
}
