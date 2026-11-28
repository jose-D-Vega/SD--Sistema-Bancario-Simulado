/**
 * Pantalla 1: Bienvenida e inserción simulada de tarjeta.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  global.ATMScreens.bienvenida = {
    titulo: '',

    render(contenedor, app) {
      app.actualizarIndicadoresHardware({ tarjeta: '', efectivo: '' });

      contenedor.innerHTML = `
        <div class="welcome-stage">
          <div>
            <h2 class="welcome-title">Bienvenido a Sudameris</h2>
            <p class="welcome-subtitle">
              Para comenzar a operar, insertá tu tarjeta de débito en el lector iluminado.
            </p>
          </div>

          <div class="card-insert-visual" id="btn-area-tarjeta" role="button" tabindex="0" aria-label="Simular inserción de tarjeta">
            <div class="debit-card-preview" aria-hidden="true">
              <div class="debit-card-top">
                <span>SUDAMERIS</span>
                <span>DÉBITO</span>
              </div>
              <div class="debit-card-chip"></div>
              <div class="debit-card-num">•••• &nbsp;•••• &nbsp;•••• &nbsp;4821</div>
            </div>

            <div class="card-reader-arrow" aria-hidden="true">
              <div class="reader-slot-box">
                <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M5 12h14"></path>
                  <path d="m13 6 6 6-6 6"></path>
                </svg>
              </div>
              <span>Insertar aquí</span>
            </div>
          </div>

          <button type="button" class="atm-btn btn-primary-large" id="btn-insertar-tarjeta">
            Insertar tarjeta
          </button>
        </div>
      `;

      const iniciar = () => {
        global.ATMState.reiniciarSesion();
        global.ATMState.datos.tarjetaInsertada = true;
        app.irA('validacion');
      };

      const btnInsertar = contenedor.querySelector('#btn-insertar-tarjeta');
      const areaTarjeta = contenedor.querySelector('#btn-area-tarjeta');

      btnInsertar.addEventListener('click', iniciar);
      areaTarjeta.addEventListener('click', iniciar);
      areaTarjeta.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          iniciar();
        }
      });

      app.configurarBotonesLaterales({
        R4: iniciar
      });

      return {
        onKeyDown(e) {
          if (e.key === 'Enter') {
            e.preventDefault();
            iniciar();
          }
        }
      };
    }
  };
})(window);
