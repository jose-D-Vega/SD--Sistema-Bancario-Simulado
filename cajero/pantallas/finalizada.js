/**
 * Pantalla 8: Transacción finalizada con recordatorio destacado de retirar la tarjeta.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  global.ATMScreens.finalizada = {
    titulo: '',

    render(contenedor, app) {
      app.actualizarIndicadoresHardware({ tarjeta: 'warning', efectivo: '' });

      let segundosRestantes = 10;

      contenedor.innerHTML = `
        <div class="atm-grid-layout">
          <div class="atm-center-full" style="gap: 20px;">
            <div>
              <h2 class="atm-headline" style="font-size: 30px; margin-bottom: 10px;">
                Transacción finalizada
              </h2>
              <p class="atm-subtext" style="font-size: 21px; color: var(--verde-principal); font-weight: 600;">
                Gracias por utilizar nuestro cajero.
              </p>
            </div>

            <div class="card-reminder-box" role="alert">
              <svg width="34" height="26" viewBox="0 0 36 26" fill="none" aria-hidden="true">
                <rect x="1.5" y="1.5" width="33" height="23" rx="4" stroke="currentColor" stroke-width="2.5"/>
                <rect x="5" y="6" width="8" height="6" rx="1.5" fill="currentColor"/>
                <path d="M5 18h14" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"/>
              </svg>
              <span>No olvides retirar tu tarjeta.</span>
            </div>

            <p class="countdown-text" id="final-countdown">
              Volviendo a la pantalla inicial en ${segundosRestantes} segundos...
            </p>
          </div>

          <button type="button" class="atm-btn atm-btn--right slot-R4" id="btn-retirar-tarjeta">
            <span>Retirar tarjeta</span>
          </button>
        </div>
      `;

      const countdownEl = contenedor.querySelector('#final-countdown');

      const terminarYSalir = () => {
        app.cancelarYVolverInicio();
      };

      contenedor.querySelector('#btn-retirar-tarjeta').addEventListener('click', terminarYSalir);
      app.configurarBotonesLaterales({ R4: terminarYSalir });

      global.ATMState.registrarInterval(() => {
        segundosRestantes -= 1;
        if (segundosRestantes <= 0) {
          terminarYSalir();
          return;
        }
        countdownEl.textContent = `Volviendo a la pantalla inicial en ${segundosRestantes} segundos...`;
      }, 1000);

      return {
        onKeyDown(e) {
          if (e.key === 'Enter' || e.key === 'Escape') {
            e.preventDefault();
            terminarYSalir();
          }
        }
      };
    }
  };
})(window);
