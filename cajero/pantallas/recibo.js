/**
 * Pantalla 6: Confirmación de tipo de recibo para la extracción.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  global.ATMScreens.recibo = {
    titulo: 'Extracción',

    render(contenedor, app) {
      const monto = global.ATMState.datos.extraccion.montoConfirmado;
      if (!global.ATMState.datos.autenticado || !monto) {
        app.irA('menu');
        return {};
      }

      app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });

      contenedor.innerHTML = `
        <div class="atm-grid-layout">
          <div class="atm-center-top">
            <h2 class="atm-headline">
              Billetes disponibles<br>
              ¿Deseás un recibo?
            </h2>

            <div class="amount-summary-pill">
              Monto a extraer: <strong>${app.formatearGuaranies(monto)}</strong>
            </div>
          </div>

          <button type="button" class="atm-btn atm-btn--secondary atm-btn--left slot-L4" id="btn-recibo-volver">
            <span class="atm-btn-arrow" aria-hidden="true">◀</span>
            <span>Volver</span>
          </button>

          <button type="button" class="atm-btn atm-btn--right slot-R2" data-recibo="imprimir">
            <span>Imprimir</span>
          </button>
          <button type="button" class="atm-btn atm-btn--right slot-R3" data-recibo="pantalla">
            <span>Pantalla</span>
          </button>
          <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R4" data-recibo="no">
            <span>No</span>
          </button>
        </div>
      `;

      const elegirRecibo = (tipo) => {
        global.ATMState.datos.extraccion.tipoRecibo = tipo;
        app.irA('procesando');
      };

      const volver = () => {
        app.irA('extraccion');
      };

      contenedor.querySelector('#btn-recibo-volver').addEventListener('click', volver);
      contenedor.querySelectorAll('[data-recibo]').forEach((btn) => {
        btn.addEventListener('click', () => elegirRecibo(btn.dataset.recibo));
      });

      app.configurarBotonesLaterales({
        L4: volver,
        R2: () => elegirRecibo('imprimir'),
        R3: () => elegirRecibo('pantalla'),
        R4: () => elegirRecibo('no')
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
