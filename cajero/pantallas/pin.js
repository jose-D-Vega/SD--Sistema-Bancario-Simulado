/**
 * Pantalla 3: Ingreso de PIN de 4 dígitos con teclado visual y teclado físico.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  global.ATMScreens.pin = {
    titulo: '',

    render(contenedor, app) {
      app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });
      global.ATMState.limpiarPin();
      let validando = false;

      contenedor.innerHTML = `
        <div class="atm-grid-layout">
          <div class="atm-center-wide">
            <div class="pin-icon-wrap" aria-hidden="true">
              <svg width="84" height="72" viewBox="0 0 96 82" fill="none" xmlns="http://www.w3.org/2000/svg">
                <rect x="10" y="6" width="76" height="52" rx="7" fill="#176B46"/>
                <rect x="18" y="13" width="10" height="10" rx="2" fill="#FAFDF9"/>
                <rect x="32" y="13" width="10" height="10" rx="2" fill="#FAFDF9"/>
                <rect x="46" y="13" width="10" height="10" rx="2" fill="#FAFDF9"/>
                <rect x="18" y="27" width="10" height="10" rx="2" fill="#FAFDF9"/>
                <rect x="32" y="27" width="10" height="10" rx="2" fill="#FAFDF9"/>
                <rect x="46" y="27" width="10" height="10" rx="2" fill="#FAFDF9"/>
                <rect x="18" y="41" width="10" height="10" rx="2" fill="#FAFDF9"/>
                <rect x="46" y="41" width="10" height="10" rx="2" fill="#FAFDF9"/>
                <rect x="62" y="13" width="16" height="10" rx="2" fill="#FAFDF9"/>
                <rect x="62" y="27" width="16" height="10" rx="2" fill="#FAFDF9"/>
                <rect x="62" y="41" width="16" height="10" rx="2" fill="#FAFDF9"/>
                <path d="M37 32C37 29.8 38.8 28 41 28C43.2 28 45 29.8 45 32V46L48 44.5C50.5 43.3 53.5 44.2 55 46.5L59 53C60.5 55.5 60 59 58 61.5L54 66H34L30 57C28.8 54.4 29.5 51.3 31.6 49.4L37 44.5V32Z" fill="#064B35" stroke="#FAFDF9" stroke-width="2.5" stroke-linejoin="round"/>
                <rect x="33" y="69" width="22" height="8" rx="3" fill="#064B35"/>
              </svg>
            </div>

            <h2 class="pin-prompt">Digitá tu PIN</h2>

            <div class="pin-box" id="pin-dots" aria-label="PIN de 4 dígitos">
              <span class="pin-dot"></span>
              <span class="pin-dot"></span>
              <span class="pin-dot"></span>
              <span class="pin-dot"></span>
            </div>

            <div id="pin-teclado-slot"></div>

            <div class="feedback-msg" id="pin-feedback" role="alert"></div>
          </div>

          <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-pin-continuar">
            <span>Continuar</span>
          </button>
          <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R3" id="btn-pin-corregir">
            <span>Corregir</span>
          </button>
          <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-pin-cancelar">
            <span>Cancelar</span>
          </button>
        </div>
      `;

      const dots = contenedor.querySelectorAll('.pin-dot');
      const feedback = contenedor.querySelector('#pin-feedback');
      const tecladoSlot = contenedor.querySelector('#pin-teclado-slot');

      const actualizarPuntos = () => {
        const largo = global.ATMState.datos.pinIngresado.length;
        dots.forEach((dot, idx) => {
          dot.classList.toggle('filled', idx < largo);
        });
      };

      const mostrarMensaje = (texto, tipo) => {
        feedback.textContent = texto || '';
        feedback.className = 'feedback-msg' + (texto ? ` feedback-msg--${tipo || 'error'}` : '');
      };

      const agregarDigito = (digito) => {
        if (validando) return;
        if (global.ATMState.datos.pinIngresado.length >= 4) return;
        global.ATMState.datos.pinIngresado += digito;
        mostrarMensaje('');
        actualizarPuntos();
      };

      const corregir = () => {
        if (validando) return;
        const actual = global.ATMState.datos.pinIngresado;
        if (actual.length > 0) {
          global.ATMState.datos.pinIngresado = actual.slice(0, -1);
          actualizarPuntos();
        }
        mostrarMensaje('');
      };

      const cancelar = () => {
        app.cancelarYVolverInicio();
      };

      const continuar = () => {
        if (validando) return;
        const pinActual = global.ATMState.datos.pinIngresado;

        if (pinActual.length < 4) {
          mostrarMensaje('El PIN está incompleto. Ingresá los 4 dígitos.', 'error');
          return;
        }

        validando = true;
        mostrarMensaje('Validando PIN...', 'info');

        global.ATMState.registrarTimeout(() => {
          const resultado = global.ATMData.validarPin(pinActual);
          // No conservar el PIN luego de validar
          global.ATMState.limpiarPin();
          actualizarPuntos();

          if (resultado.ok) {
            global.ATMState.datos.autenticado = true;
            global.ATMState.datos.intentosPinFallidos = 0;
            app.irA('menu');
            return;
          }

          validando = false;
          global.ATMState.datos.intentosPinFallidos += 1;
          const intentos = global.ATMState.datos.intentosPinFallidos;

          if (intentos >= 3) {
            validando = true;
            mostrarMensaje('Superaste los 3 intentos fallidos. Retirá tu tarjeta.', 'error');
            global.ATMState.registrarTimeout(() => {
              app.irA('finalizada', { motivo: 'intentos' });
            }, 1800);
            return;
          }

          mostrarMensaje(`PIN incorrecto. Intentá nuevamente (${intentos}/3).`, 'error');
        }, 450);
      };

      const teclado = app.crearTecladoNumerico({
        onDigit: agregarDigito,
        onBackspace: corregir,
        onClear: () => {
          if (validando) return;
          global.ATMState.limpiarPin();
          actualizarPuntos();
          mostrarMensaje('');
        }
      });
      tecladoSlot.appendChild(teclado);

      contenedor.querySelector('#btn-pin-continuar').addEventListener('click', continuar);
      contenedor.querySelector('#btn-pin-corregir').addEventListener('click', corregir);
      contenedor.querySelector('#btn-pin-cancelar').addEventListener('click', cancelar);

      app.configurarBotonesLaterales({
        R2: continuar,
        R3: corregir,
        R4: cancelar
      });

      return {
        onKeyDown(e) {
          if (/^[0-9]$/.test(e.key)) {
            e.preventDefault();
            agregarDigito(e.key);
          } else if (e.key === 'Backspace') {
            e.preventDefault();
            corregir();
          } else if (e.key === 'Enter') {
            e.preventDefault();
            continuar();
          } else if (e.key === 'Escape') {
            e.preventDefault();
            cancelar();
          }
        }
      };
    }
  };
})(window);
