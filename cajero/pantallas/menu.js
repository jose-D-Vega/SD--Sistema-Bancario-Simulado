/**
 * Pantalla 4: Menú Principal de operaciones del ATM.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  global.ATMScreens.menu = {
    titulo: 'Menú Principal',

    render(contenedor, app) {
      if (!global.ATMState.datos.autenticado) {
        app.irA('bienvenida');
        return {};
      }

      global.ATMState.limpiarOperacionTemporal();
      global.ATMState.limpiarExtraccion();
      global.ATMState.seleccionarCuenta(null);
      app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });

      contenedor.innerHTML = `
        <div class="atm-grid-layout">
          <button type="button" class="atm-btn atm-btn--left slot-L1" data-accion="giros">
            <span class="atm-btn-arrow" aria-hidden="true">◀</span>
            <span>Giros Simple</span>
          </button>
          <button type="button" class="atm-btn atm-btn--left slot-L2" data-accion="tarjetas">
            <span class="atm-btn-arrow" aria-hidden="true">◀</span>
            <span>Tarjetas</span>
          </button>
          <button type="button" class="atm-btn atm-btn--left slot-L3" data-accion="extraccion">
            <span class="atm-btn-arrow" aria-hidden="true">◀</span>
            <span>Extracción</span>
          </button>
          <button type="button" class="atm-btn atm-btn--left slot-L4" data-accion="cuenta">
            <span class="atm-btn-arrow" aria-hidden="true">◀</span>
            <span>Consultar Cuenta</span>
          </button>

          <div class="atm-center-stage">
            <h2 class="atm-headline" style="font-size: 22px;">¿Qué operación deseás realizar?</h2>
            <p class="atm-subtext" style="font-size: 14px; margin-top: 8px;">
              Seleccioná una opción en pantalla o presioná el botón lateral correspondiente.
            </p>
          </div>

          <button type="button" class="atm-btn atm-btn--right slot-R1" data-accion="servicios">
            <span>Servicios</span>
            <span class="atm-btn-arrow" aria-hidden="true">▶</span>
          </button>
          <button type="button" class="atm-btn atm-btn--right slot-R2" data-accion="otras">
            <span>Otras Operaciones</span>
            <span class="atm-btn-arrow" aria-hidden="true">▶</span>
          </button>
          <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" data-accion="salir">
            <span>Salir</span>
            <span class="atm-btn-arrow" aria-hidden="true">▶</span>
          </button>
        </div>
      `;

      const acciones = {
        giros: () => app.irA('giros'),
        tarjetas: () => app.irA('tarjetas'),
        extraccion: () => app.irA('extraccion'),
        cuenta: () => app.irA('cuenta'),
        servicios: () => app.irA('servicios'),
        otras: () => app.irA('otrasOperaciones'),
        salir: () => app.irA('finalizada', { motivo: 'salida' })
      };

      contenedor.querySelectorAll('[data-accion]').forEach((btn) => {
        btn.addEventListener('click', () => {
          const fn = acciones[btn.dataset.accion];
          if (fn) fn();
        });
      });

      app.configurarBotonesLaterales({
        L1: acciones.giros,
        L2: acciones.tarjetas,
        L3: acciones.extraccion,
        L4: acciones.cuenta,
        R1: acciones.servicios,
        R2: acciones.otras,
        R4: acciones.salir
      });

      return {
        onKeyDown(e) {
          if (e.key === 'Escape') {
            e.preventDefault();
            acciones.salir();
          }
        }
      };
    }
  };
})(window);
