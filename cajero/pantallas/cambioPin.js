/**
 * Flujo sensible simulado para Cambio de PIN en memoria (nunca persistido ni mostrado en texto).
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  function renderPasoPin(contenedor, app, { titulo, mensaje, onSubmit, onCancel }) {
    app.establecerTitulo('Cambio de PIN');
    app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });

    let valorPin = '';
    let bloqueado = false;

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-wide" style="gap: 8px;">
          <h2 class="atm-headline" style="font-size: 22px;">${titulo}</h2>
          <p class="atm-subtext" style="font-size: 13px;">${mensaje}</p>

          <div class="pin-box" id="cpin-dots" aria-label="PIN de 4 dígitos">
            <span class="pin-dot"></span>
            <span class="pin-dot"></span>
            <span class="pin-dot"></span>
            <span class="pin-dot"></span>
          </div>

          <div id="cpin-teclado-slot"></div>
          <div class="feedback-msg" id="cpin-feedback" role="alert"></div>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-cpin-continuar">
          <span>Continuar</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R3" id="btn-cpin-corregir">
          <span>Corregir</span>
        </button>
        <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-cpin-cancelar">
          <span>Cancelar</span>
        </button>
      </div>
    `;

    const dots = contenedor.querySelectorAll('.pin-dot');
    const feedbackEl = contenedor.querySelector('#cpin-feedback');

    const actualizarPuntos = () => {
      dots.forEach((d, idx) => d.classList.toggle('filled', idx < valorPin.length));
    };

    const mostrarError = (msg) => {
      feedbackEl.textContent = msg || '';
      feedbackEl.className = 'feedback-msg' + (msg ? ' feedback-msg--error' : '');
    };

    const agregar = (d) => {
      if (bloqueado || valorPin.length >= 4) return;
      valorPin += d;
      mostrarError('');
      actualizarPuntos();
    };

    const corregir = () => {
      if (bloqueado || valorPin.length === 0) return;
      valorPin = valorPin.slice(0, -1);
      mostrarError('');
      actualizarPuntos();
    };

    const cancelar = () => {
      valorPin = '';
      if (onCancel) onCancel();
    };

    const continuar = () => {
      if (bloqueado) return;
      if (valorPin.length < 4) {
        mostrarError('El PIN debe tener exactamente 4 dígitos.');
        return;
      }

      const ingresado = valorPin;
      valorPin = '';
      actualizarPuntos();

      onSubmit(ingresado, {
        mostrarError,
        bloquear: (v) => {
          bloqueado = v;
        }
      });
    };

    contenedor.querySelector('#cpin-teclado-slot').appendChild(
      app.crearTecladoNumerico({
        onDigit: agregar,
        onBackspace: corregir,
        onClear: () => {
          if (bloqueado) return;
          valorPin = '';
          mostrarError('');
          actualizarPuntos();
        }
      })
    );

    contenedor.querySelector('#btn-cpin-continuar').addEventListener('click', continuar);
    contenedor.querySelector('#btn-cpin-corregir').addEventListener('click', corregir);
    contenedor.querySelector('#btn-cpin-cancelar').addEventListener('click', cancelar);
    app.configurarBotonesLaterales({ R2: continuar, R3: corregir, R4: cancelar });

    return {
      onKeyDown(e) {
        if (/^[0-9]$/.test(e.key)) {
          e.preventDefault();
          agregar(e.key);
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

  global.ATMScreens.cambioPin = {
    titulo: 'Cambio de PIN',

    render(contenedor, app) {
      if (!global.ATMState.datos.autenticado) {
        app.irA('bienvenida');
        return {};
      }

      let intentosActual = 0;
      let nuevoPinTemporal = '';
      let controlador = {};

      const iniciarPaso1 = () => {
        controlador = renderPasoPin(contenedor, app, {
          titulo: 'Ingresá tu PIN actual',
          mensaje: 'Por seguridad, validá tu PIN vigente antes de cambiarlo.',
          onCancel: () => {
            nuevoPinTemporal = '';
            app.irA('otrasOperaciones');
          },
          onSubmit: (pinActual, ui) => {
            const res = global.ATMData.validarPin(pinActual);
            if (!res.ok) {
              intentosActual += 1;
              if (intentosActual >= 3) {
                ui.bloquear(true);
                ui.mostrarError('Superaste los 3 intentos permitidos. Operación cancelada.');
                global.ATMState.registrarTimeout(() => app.irA('otrasOperaciones'), 1800);
                return;
              }
              ui.mostrarError(`PIN actual incorrecto (${intentosActual}/3).`);
              return;
            }
            iniciarPaso2();
          }
        });
      };

      const iniciarPaso2 = () => {
        controlador = renderPasoPin(contenedor, app, {
          titulo: 'Ingresá tu nuevo PIN',
          mensaje: 'Elegí 4 dígitos. Evitá combinaciones simples como 0000, 1111 o 1234.',
          onCancel: () => {
            nuevoPinTemporal = '';
            app.irA('otrasOperaciones');
          },
          onSubmit: (pinNuevo, ui) => {
            const val = global.ATMData.validarNuevoPin(pinNuevo);
            if (!val.ok) {
              ui.mostrarError(val.mensaje);
              return;
            }
            nuevoPinTemporal = pinNuevo;
            iniciarPaso3();
          }
        });
      };

      const iniciarPaso3 = () => {
        controlador = renderPasoPin(contenedor, app, {
          titulo: 'Confirmá tu nuevo PIN',
          mensaje: 'Volvé a ingresar el nuevo PIN para confirmar el cambio.',
          onCancel: () => {
            nuevoPinTemporal = '';
            app.irA('otrasOperaciones');
          },
          onSubmit: (pinConfirmacion, ui) => {
            if (pinConfirmacion !== nuevoPinTemporal) {
              ui.mostrarError('Los PIN ingresados no coinciden. Intentá nuevamente.');
              return;
            }

            const resultado = global.ATMData.cambiarPinSimulado(nuevoPinTemporal);
            nuevoPinTemporal = '';

            if (!resultado.ok) {
              ui.mostrarError(resultado.mensaje);
              return;
            }

            app.configurarBotonesLaterales({});
            contenedor.innerHTML = `
              <div class="center-message-stage">
                <h2 class="atm-headline">Actualizando PIN...</h2>
                <div class="loading-squares">
                  <span class="loading-square"></span>
                  <span class="loading-square"></span>
                  <span class="loading-square"></span>
                </div>
              </div>
            `;

            global.ATMState.registrarTimeout(() => {
              const volverMenu = () => app.irA('menu');
              contenedor.innerHTML = `
                <div class="atm-grid-layout">
                  <div class="atm-center-full" style="gap: 14px;">
                    <div class="status-badge-ok">✓ Operación segura</div>
                    <h2 class="atm-headline">PIN cambiado correctamente</h2>
                    <p class="atm-subtext">
                      El nuevo PIN quedó activo en memoria para esta sesión (sin persistencia).
                    </p>
                  </div>
                  <button type="button" class="atm-btn atm-btn--right slot-R4" id="btn-cpin-fin">
                    <span>Volver al menú</span>
                  </button>
                </div>
              `;
              contenedor.querySelector('#btn-cpin-fin').addEventListener('click', volverMenu);
              app.configurarBotonesLaterales({ R4: volverMenu });
            }, 1200);
          }
        });
      };

      iniciarPaso1();

      return {
        onKeyDown(e) {
          if (controlador && typeof controlador.onKeyDown === 'function') {
            controlador.onKeyDown(e);
          }
        }
      };
    }
  };
})(window);
