package handlers

import (
	"net/http"
	"testing"

	"github.com/sudameris/backend/internal/models"
)

// Regression test de un hueco encontrado en revisión de código (no es uno
// de los 20 casos del doc 04, pero ejercita la misma lógica compartida de
// limites.go que los casos 9 y 18): totalSalidoHoy solo sumaba
// operaciones en estado COMPLETADA. Una transferencia interbancaria
// saliente (transferencias_interbancarias.go) descuenta el saldo de
// origen apenas queda RESERVADA, antes de llamar al banco destino, y
// puede seguir PENDIENTE un buen rato si esa llamada da timeout. Mientras
// tanto, el dinero ya no está disponible pero no contaba para el límite
// diario combinado (doc 01 §4, G. 10.000.000) -- así que una segunda
// operación sobre la misma cuenta podía colarse por encima del límite.
//
// Se simula la operación "en vuelo" insertándola directo en la base con
// crearOperacionDirecta (el mismo helper que ya usa movimientos_test.go
// para estados que no salen naturalmente de llamar a un handler), en vez
// de armar todo mundoInterbancario y esperar un timeout real de
// timeoutLlamadaBanco (5s): lo que se quiere probar es que totalSalidoHoy
// la cuenta, sin importar por qué mecanismo haya quedado en ese estado.
func TestTransferenciaInterna_RespetaLimiteConInterbancariaEnVuelo(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := TransferenciaInterna(db, bancoID)

	for _, estadoEnVuelo := range []string{models.OperacionReservada, models.OperacionPendiente} {
		t.Run(estadoEnVuelo, func(t *testing.T) {
			usuarioID := crearUsuarioDePrueba(t)
			// Cuentas nuevas por subtest (números únicos) para que las dos
			// corridas de la tabla no compartan "lo ya gastado hoy".
			origen := "ORIGEN-" + idAleatorio(t)
			destino := "DESTINO-" + idAleatorio(t)
			crearCuentaDeUsuarioDirecta(t, bancoID, origen, 50_000_000, "", usuarioID)
			crearCuentaDirecta(t, bancoID, destino, 0, "")

			// Transferencia interbancaria saliente de G. 9.500.000 que
			// todavía no se resolvió. No tocamos el saldo acá a mano: lo
			// que importa para este test es solo si totalSalidoHoy la
			// cuenta o no -- el descuento real ya lo prueban los tests de
			// transferencias_interbancarias.go.
			crearOperacionDirecta(t, operacionDePrueba{
				Tipo:                models.TipoTransferenciaInterbancaria,
				BancoOrigen:         bancoID,
				NumeroCuentaOrigen:  origen,
				BancoDestino:        "OTRO-BANCO",
				NumeroCuentaDestino: "CTA-EN-OTRO-BANCO",
				Importe:             9_500_000,
				Estado:              estadoEnVuelo,
			})

			// 9.500.000 (en vuelo) + 1.000.000 (esta) > 10.000.000: tiene
			// que rechazarse por límite diario combinado, aunque la
			// primera todavía no esté COMPLETADA.
			req := transferenciaInternaRequest{
				NumeroCuentaOrigen:  origen,
				NumeroCuentaDestino: destino,
				Importe:             1_000_000,
				IdempotencyKey:      uuidDePrueba(t),
			}
			w, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/internas", req, nil, map[string]string{"usuarioID": usuarioID})
			esperarStatus(t, w, http.StatusUnprocessableEntity)

			op := decodificarOperacion(t, w)
			if op.Estado != models.OperacionRechazada {
				t.Fatalf("debería rechazarse por límite diario combinado (9.500.000 %s + 1.000.000 > 10.000.000), dio estado %q", estadoEnVuelo, op.Estado)
			}

			if saldo := saldoActual(t, bancoID, origen); saldo != 50_000_000 {
				t.Fatalf("el rechazo no debería tocar el saldo de origen, quedó en %d", saldo)
			}
		})
	}
}

// Complemento del test anterior: una interbancaria que YA se resolvió
// como RECHAZADA (el destino la rechazó, o la reconciliación confirmó que
// nunca se acreditó) no debe seguir "ocupando" cupo del límite diario --
// revertirReserva ya le devolvió el saldo a la cuenta, así que el límite
// diario tiene que liberarse junto con el saldo, no quedar aparte.
func TestTransferenciaInterna_InterbancariaRechazadaNoCuentaParaElLimite(t *testing.T) {
	db := requireTestDB(t)
	bancoID := bancoIDDePrueba(t)
	handler := TransferenciaInterna(db, bancoID)

	usuarioID := crearUsuarioDePrueba(t)
	crearCuentaDeUsuarioDirecta(t, bancoID, "ORIGEN", 50_000_000, "", usuarioID)
	crearCuentaDirecta(t, bancoID, "DESTINO", 0, "")

	crearOperacionDirecta(t, operacionDePrueba{
		Tipo:                models.TipoTransferenciaInterbancaria,
		BancoOrigen:         bancoID,
		NumeroCuentaOrigen:  "ORIGEN",
		BancoDestino:        "OTRO-BANCO",
		NumeroCuentaDestino: "CTA-EN-OTRO-BANCO",
		Importe:             9_500_000,
		Estado:              models.OperacionRechazada,
	})

	req := transferenciaInternaRequest{
		NumeroCuentaOrigen:  "ORIGEN",
		NumeroCuentaDestino: "DESTINO",
		Importe:             1_000_000,
		IdempotencyKey:      uuidDePrueba(t),
	}
	w, _ := ejecutarJSON(t, handler, http.MethodPost, "/transferencias/internas", req, nil, map[string]string{"usuarioID": usuarioID})
	esperarStatus(t, w, http.StatusCreated)

	op := decodificarOperacion(t, w)
	if op.Estado != models.OperacionCompletada {
		t.Fatalf("una interbancaria RECHAZADA no debería contar para el límite diario, dio estado %q", op.Estado)
	}
}
