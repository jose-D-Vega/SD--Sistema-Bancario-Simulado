/**
 * Pantalla 2: Secuencia simulada de lectura y validación del chip.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  global.ATMScreens.validacion = {
    titulo: '',

    render(contenedor, app) {
      app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });
      app.configurarBotonesLaterales({});

      contenedor.innerHTML = `
        <div class="center-message-stage">
          <h2 class="atm-headline" id="validacion-mensaje">
            Por favor, inserte su tarjeta nuevamente.
          </h2>
          <p class="atm-subtext" id="validacion-submensaje">
            Iniciando lectura segura del lector...
          </p>
          <div class="loading-squares" aria-label="Validando tarjeta">
            <span class="loading-square"></span>
            <span class="loading-square"></span>
            <span class="loading-square"></span>
          </div>
        </div>
      `;

      const msgEl = contenedor.querySelector('#validacion-mensaje');
      const subEl = contenedor.querySelector('#validacion-submensaje');

      global.ATMState.registrarTimeout(() => {
        msgEl.textContent = 'Por favor, espere que el chip sea validado antes de retirar su tarjeta.';
        subEl.textContent = 'Verificando credenciales de seguridad...';
      }, 1300);

      global.ATMState.registrarTimeout(() => {
        app.irA('pin');
      }, 3300);

      return {};
    }
  };
})(window);
