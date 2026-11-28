/**
 * Consulta de Movimientos / Extracto: muestra hasta los últimos 5 movimientos y permite imprimir extracto.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  function renderListaMovimientos(contenedor, app, cuenta) {
    const movimientos = global.ATMData.obtenerMovimientos(cuenta.id, 5);
    app.establecerTitulo('Consulta de movimientos');
    app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });

    const filasHtml = movimientos.length
      ? movimientos
          .map(
            (m) => `
          <div class="movement-row">
            <span class="movement-date">${m.fecha}</span>
            <span class="movement-desc">${m.descripcion}</span>
            <span class="movement-type movement-type--${m.tipo === 'Crédito' ? 'credito' : 'debito'}">${m.tipo}</span>
            <span class="movement-amount">${m.tipo === 'Crédito' ? '+' : '-'}${app.formatearGuaranies(m.importe)}</span>
          </div>
        `
          )
          .join('')
      : '<p class="atm-subtext">No se registran movimientos recientes en esta cuenta.</p>';

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-full" style="gap: 10px;">
          <span class="simulation-tag">${cuenta.tipoCuenta} · ${cuenta.numeroEnmascarado} · Últimos movimientos</span>
          <div class="movements-card">${filasHtml}</div>
        </div>

        <button type="button" class="atm-btn atm-btn--left slot-L4" id="btn-mov-imprimir">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>Imprimir extracto</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R4" id="btn-mov-volver">
          <span>Volver</span>
        </button>
      </div>
    `;

    const imprimir = () => {
      const filasTicket = [
        { etiqueta: 'Cuenta', valor: `${cuenta.tipoCuenta} · ${cuenta.numeroEnmascarado}` },
        { etiqueta: 'Saldo actual', valor: app.formatearGuaranies(cuenta.saldoDisponible) },
        ...movimientos.map((m) => ({
          etiqueta: `${m.fecha} · ${m.descripcion}`,
          valor: `${m.tipo === 'Crédito' ? '+' : '-'}${app.formatearGuaranies(m.importe)}`
        }))
      ];

      global.ATMCommon.renderProcesandoYResultado(contenedor, app, {
        titulo: 'Extracto de movimientos',
        mensajeProceso: 'Generando extracto impreso...',
        submensajeProceso: 'Imprimiendo últimos movimientos...',
        tipoRecibo: 'imprimir',
        tituloExito: 'Extracto impreso disponible',
        ejecutar: () => ({
          ok: true,
          operacion: {
            numeroOperacion: 'EX-' + String(Math.floor(100000 + Math.random() * 900000))
          }
        }),
        construirComprobante: () => ({
          tituloTicket: 'EXTRACTO DE CUENTA',
          filas: filasTicket
        })
      });
    };

    const volver = () => app.irA('otrasOperaciones');

    contenedor.querySelector('#btn-mov-imprimir').addEventListener('click', imprimir);
    contenedor.querySelector('#btn-mov-volver').addEventListener('click', volver);
    app.configurarBotonesLaterales({ L4: imprimir, R4: volver });
  }

  global.ATMScreens.movimientos = {
    titulo: 'Consulta de movimientos',

    render(contenedor, app) {
      if (!global.ATMState.datos.autenticado) {
        app.irA('bienvenida');
        return {};
      }

      return global.ATMCommon.renderSeleccionCuenta(contenedor, app, {
        titulo: 'Consulta de movimientos',
        mensaje: 'Seleccioná la cuenta para ver sus últimos movimientos',
        mostrarSaldo: false,
        etiquetaVolver: 'Volver',
        alElegir: (cuenta) => renderListaMovimientos(contenedor, app, cuenta),
        alCancelar: () => app.irA('otrasOperaciones')
      });
    }
  };
})(window);
