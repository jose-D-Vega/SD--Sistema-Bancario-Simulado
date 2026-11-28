/**
 * Submenú Otras Operaciones: Transferencias, Movimientos, Cambio de PIN y Adelanto de efectivo.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  global.ATMScreens.otrasOperaciones = {
    titulo: 'Otras Operaciones',

    render(contenedor, app) {
      if (!global.ATMState.datos.autenticado) {
        app.irA('bienvenida');
        return {};
      }

      const tc = global.ATMData.obtenerTarjetaCredito();
      app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });

      contenedor.innerHTML = `
        <div class="atm-grid-layout">
          <button type="button" class="atm-btn atm-btn--left slot-L1" id="btn-oo-adelanto">
            <span class="atm-btn-arrow" aria-hidden="true">◀</span>
            <span>Adelanto de efectivo</span>
          </button>

          <div class="atm-center-stage">
            <h2 class="atm-headline" style="font-size: 22px;">Otras Operaciones</h2>
            <p class="atm-subtext" style="font-size: 14px;">
              Seleccioná la operación adicional que deseás realizar.
            </p>
            ${
              tc.disponible
                ? ''
                : '<span class="simulation-tag" style="margin-top: 8px;">Adelanto requiere tarjeta de crédito habilitada</span>'
            }
          </div>

          <button type="button" class="atm-btn atm-btn--right slot-R1" id="btn-oo-transferencias">
            <span>Transferencias</span>
          </button>
          <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-oo-movimientos">
            <span>Movimientos / Extracto</span>
          </button>
          <button type="button" class="atm-btn atm-btn--right slot-R3" id="btn-oo-pin">
            <span>Cambio de PIN</span>
          </button>
          <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R4" id="btn-oo-volver">
            <span>Volver</span>
          </button>
        </div>
      `;

      const irAdelanto = () => app.irA('adelanto');
      const irTransferencias = () => app.irA('transferencias');
      const irMovimientos = () => app.irA('movimientos');
      const irCambioPin = () => app.irA('cambioPin');
      const volver = () => app.irA('menu');

      contenedor.querySelector('#btn-oo-adelanto').addEventListener('click', irAdelanto);
      contenedor.querySelector('#btn-oo-transferencias').addEventListener('click', irTransferencias);
      contenedor.querySelector('#btn-oo-movimientos').addEventListener('click', irMovimientos);
      contenedor.querySelector('#btn-oo-pin').addEventListener('click', irCambioPin);
      contenedor.querySelector('#btn-oo-volver').addEventListener('click', volver);

      app.configurarBotonesLaterales({
        L1: irAdelanto,
        R1: irTransferencias,
        R2: irMovimientos,
        R3: irCambioPin,
        R4: volver
      });

      return {
        onKeyDown(e) {
          if (e.key === 'Escape') {
            e.preventDefault();
            volver();
          }
        }
      };
    }
  };
})(window);
