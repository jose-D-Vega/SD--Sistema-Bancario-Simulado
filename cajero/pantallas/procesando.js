/**
 * Pantalla 7: Simulación de procesamiento de extracción y entrega de comprobante.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  function crearHtmlComprobante(op, app, esImpreso) {
    return `
      <div class="receipt-card ${esImpreso ? 'is-thermal' : ''}">
        <div class="receipt-header">
          <strong>SUDAMERIS · ${esImpreso ? 'TICKET IMPRESO' : 'COMPROBANTE'}</strong>
          <span>${op.numeroOperacion}</span>
        </div>
        <div class="receipt-rows">
          <div class="receipt-row"><span>Fecha y hora</span><strong>${op.fechaHora}</strong></div>
          <div class="receipt-row"><span>Cajero</span><strong>${op.cajero}</strong></div>
          <div class="receipt-row"><span>Cuenta</span><strong>${op.tipoCuenta} · ${op.cuenta}</strong></div>
          <div class="receipt-row"><span>Operación</span><strong>Extracción ATM</strong></div>
          <div class="receipt-row"><span>Monto extraído</span><strong>${app.formatearGuaranies(op.montoExtraido)}</strong></div>
          <div class="receipt-row"><span>Saldo disponible</span><strong>${app.formatearGuaranies(op.saldoRestante)}</strong></div>
        </div>
      </div>
    `;
  }

  global.ATMScreens.procesando = {
    titulo: 'Extracción',

    render(contenedor, app) {
      const { montoConfirmado, tipoRecibo } = global.ATMState.datos.extraccion;
      const cuentaId = global.ATMState.datos.cuentaSeleccionadaId;

      if (!global.ATMState.datos.autenticado || !montoConfirmado) {
        app.irA('menu');
        return {};
      }

      app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: 'active' });
      app.configurarBotonesLaterales({});

      contenedor.innerHTML = `
        <div class="center-message-stage">
          <h2 class="atm-headline" id="proc-titulo">Procesando tu extracción...</h2>
          <p class="atm-subtext" id="proc-subtitulo">Verificando disponibilidad de efectivo...</p>
          <div class="loading-squares" aria-label="Procesando operación">
            <span class="loading-square"></span>
            <span class="loading-square"></span>
            <span class="loading-square"></span>
          </div>
        </div>
      `;

      const tituloEl = contenedor.querySelector('#proc-titulo');
      const subtituloEl = contenedor.querySelector('#proc-subtitulo');

      global.ATMState.registrarTimeout(() => {
        subtituloEl.textContent = 'Dispensando billetes...';
      }, 900);

      global.ATMState.registrarTimeout(() => {
        const resultado = global.ATMData.ejecutarExtraccionSimulada(montoConfirmado, tipoRecibo, cuentaId);

        if (!resultado.ok) {
          app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });
          tituloEl.textContent = 'No pudimos completar la extracción';
          subtituloEl.textContent = resultado.mensaje;
          const volverMenu = () => app.irA('menu');
          app.configurarBotonesLaterales({ R4: volverMenu });
          return;
        }

        const op = resultado.operacion;
        global.ATMState.datos.extraccion.ultimaOperacion = op;
        global.ATMState.datos.extraccion.montoInput = '';
        global.ATMState.datos.extraccion.montoConfirmado = 0;

        app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: 'warning' });

        const finalizar = () => app.irA('finalizada', { motivo: 'extraccion' });
        const volverMenu = () => {
          global.ATMState.limpiarExtraccion();
          app.irA('menu');
        };

        if (tipoRecibo === 'no') {
          contenedor.innerHTML = `
            <div class="atm-grid-layout">
              <div class="atm-center-full">
                <div class="status-badge-ok">✓ Operación exitosa</div>
                <h2 class="atm-headline">Retirá tu efectivo por el dispensador</h2>
                <p class="atm-subtext">
                  Se extrajo <strong>${app.formatearGuaranies(op.montoExtraido)}</strong> correctamente.
                </p>
              </div>
              <button type="button" class="atm-btn atm-btn--secondary atm-btn--left slot-L4" id="btn-proc-menu">
                <span class="atm-btn-arrow" aria-hidden="true">◀</span>
                <span>Volver al menú</span>
              </button>
              <button type="button" class="atm-btn atm-btn--right slot-R4" id="btn-proc-continuar">
                <span>Finalizar</span>
              </button>
            </div>
          `;

          contenedor.querySelector('#btn-proc-menu').addEventListener('click', volverMenu);
          contenedor.querySelector('#btn-proc-continuar').addEventListener('click', finalizar);
          app.configurarBotonesLaterales({ L4: volverMenu, R4: finalizar });
          global.ATMState.registrarTimeout(finalizar, 3500);
          return;
        }

        const esImpreso = tipoRecibo === 'imprimir';
        const tituloComprobante = esImpreso
          ? 'Retirá tu efectivo y el comprobante impreso'
          : 'Operación realizada con éxito';

        contenedor.innerHTML = `
          <div class="atm-grid-layout">
            <div class="atm-center-full" style="gap: 12px;">
              <div class="status-badge-ok">✓ ${tituloComprobante}</div>
              ${crearHtmlComprobante(op, app, esImpreso)}
            </div>
            <button type="button" class="atm-btn atm-btn--secondary atm-btn--left slot-L4" id="btn-proc-menu">
              <span class="atm-btn-arrow" aria-hidden="true">◀</span>
              <span>Volver al menú</span>
            </button>
            <button type="button" class="atm-btn atm-btn--right slot-R4" id="btn-proc-continuar">
              <span>Finalizar</span>
            </button>
          </div>
        `;

        contenedor.querySelector('#btn-proc-menu').addEventListener('click', volverMenu);
        contenedor.querySelector('#btn-proc-continuar').addEventListener('click', finalizar);
        app.configurarBotonesLaterales({ L4: volverMenu, R4: finalizar });
        global.ATMState.registrarTimeout(finalizar, 15000);
      }, 1800);

      return {
        onKeyDown(e) {
          if (e.key === 'Enter' && global.ATMState.datos.extraccion.ultimaOperacion) {
            e.preventDefault();
            app.irA('finalizada', { motivo: 'extraccion' });
          }
        }
      };
    }
  };
})(window);
