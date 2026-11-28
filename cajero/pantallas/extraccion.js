/**
 * Pantalla 5: Extracción de efectivo con selección de cuenta, validación y confirmación.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  function renderConfirmacionExtraccion(contenedor, app, cuenta, monto) {
    app.establecerTitulo('Extracción');
    app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-top" style="gap: 12px;">
          <h2 class="atm-headline" style="font-size: 24px;">¿Confirmás la extracción?</h2>
          <div class="info-summary-card">
            <h3>Resumen de la operación</h3>
            <div class="receipt-rows">
              <div class="receipt-row"><span>Cuenta de origen</span><strong>${cuenta.tipoCuenta} · ${cuenta.numeroEnmascarado}</strong></div>
              <div class="receipt-row"><span>Moneda</span><strong>${cuenta.moneda}</strong></div>
              <div class="receipt-row"><span>Monto a extraer</span><strong>${app.formatearGuaranies(monto)}</strong></div>
            </div>
          </div>
          <span class="simulation-tag">El saldo se actualizará solo al confirmar</span>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-extc-confirmar">
          <span>Confirmar</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R3" id="btn-extc-corregir">
          <span>Corregir</span>
        </button>
        <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-extc-cancelar">
          <span>Cancelar</span>
        </button>
      </div>
    `;

    const confirmar = () => {
      global.ATMState.datos.extraccion.montoConfirmado = monto;
      app.irA('recibo');
    };
    const corregir = () => renderIngresoMonto(contenedor, app, cuenta);
    const cancelar = () => {
      global.ATMState.limpiarExtraccion();
      app.irA('menu');
    };

    contenedor.querySelector('#btn-extc-confirmar').addEventListener('click', confirmar);
    contenedor.querySelector('#btn-extc-corregir').addEventListener('click', corregir);
    contenedor.querySelector('#btn-extc-cancelar').addEventListener('click', cancelar);

    app.configurarBotonesLaterales({
      R2: confirmar,
      R3: corregir,
      R4: cancelar
    });
  }

  function renderIngresoMonto(contenedor, app, cuenta) {
    app.establecerTitulo('Extracción');
    app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });

    const montosRapidos = global.ATMData.CONFIG_DEMO.MONTOS_RAPIDOS;

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <button type="button" class="atm-btn atm-btn--left slot-L1" data-monto="${montosRapidos[0]}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${app.formatearGuaranies(montosRapidos[0])}</span>
        </button>
        <button type="button" class="atm-btn atm-btn--left slot-L2" data-monto="${montosRapidos[1]}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${app.formatearGuaranies(montosRapidos[1])}</span>
        </button>
        <button type="button" class="atm-btn atm-btn--left slot-L3" data-monto="${montosRapidos[2]}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${app.formatearGuaranies(montosRapidos[2])}</span>
        </button>
        <button type="button" class="atm-btn atm-btn--left slot-L4" data-monto="${montosRapidos[3]}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${app.formatearGuaranies(montosRapidos[3])}</span>
        </button>

        <div class="atm-center-stage">
          <span class="simulation-tag" style="margin-bottom: 6px;">${cuenta.tipoCuenta} · ${cuenta.numeroEnmascarado}</span>
          <h2 class="extraction-banner">Billetes disponibles</h2>
          <div class="amount-label">Ingrese monto:</div>

          <div class="amount-display-box is-placeholder" id="monto-display" aria-live="polite">
            Gs. 0
          </div>

          <div id="extraccion-teclado-slot"></div>

          <p class="extraction-hint">Múltiplos de Gs. 10.000 · Máx. Gs. 1.500.000</p>
          <div class="feedback-msg" id="extraccion-feedback" role="alert"></div>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-ext-continuar">
          <span>Continuar</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R3" id="btn-ext-corregir">
          <span>Corregir</span>
        </button>
        <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-ext-cancelar">
          <span>Cancelar</span>
        </button>
      </div>
    `;

    const displayEl = contenedor.querySelector('#monto-display');
    const feedbackEl = contenedor.querySelector('#extraccion-feedback');
    const tecladoSlot = contenedor.querySelector('#extraccion-teclado-slot');

    const mostrarError = (mensaje) => {
      feedbackEl.textContent = mensaje || '';
      feedbackEl.className = 'feedback-msg' + (mensaje ? ' feedback-msg--error' : '');
    };

    const actualizarVistaMonto = () => {
      const raw = global.ATMState.datos.extraccion.montoInput;
      const valor = raw ? parseInt(raw, 10) : 0;
      displayEl.textContent = app.formatearGuaranies(valor);
      displayEl.classList.toggle('is-placeholder', valor === 0);
    };

    actualizarVistaMonto();

    const seleccionarMontoRapido = (monto) => {
      global.ATMState.datos.extraccion.montoInput = String(monto);
      mostrarError('');
      actualizarVistaMonto();
    };

    const agregarDigito = (digito) => {
      const actual = global.ATMState.datos.extraccion.montoInput;
      if (actual.length === 0 && digito === '0') return;
      if (actual.length >= 7) return;
      global.ATMState.datos.extraccion.montoInput = actual + digito;
      mostrarError('');
      actualizarVistaMonto();
    };

    const corregir = () => {
      const actual = global.ATMState.datos.extraccion.montoInput;
      if (actual.length > 0) {
        global.ATMState.datos.extraccion.montoInput = actual.slice(0, -1);
        actualizarVistaMonto();
      }
      mostrarError('');
    };

    const cancelar = () => {
      global.ATMState.limpiarExtraccion();
      app.irA('menu');
    };

    const continuar = () => {
      const raw = global.ATMState.datos.extraccion.montoInput;
      const monto = raw ? parseInt(raw, 10) : 0;
      const validacion = global.ATMData.validarExtraccion(monto, cuenta.id);

      if (!validacion.ok) {
        mostrarError(validacion.mensaje);
        return;
      }

      renderConfirmacionExtraccion(contenedor, app, cuenta, monto);
    };

    contenedor.querySelectorAll('[data-monto]').forEach((btn) => {
      btn.addEventListener('click', () => seleccionarMontoRapido(Number(btn.dataset.monto)));
    });

    tecladoSlot.appendChild(
      app.crearTecladoNumerico({
        onDigit: agregarDigito,
        onBackspace: corregir,
        onClear: () => {
          global.ATMState.datos.extraccion.montoInput = '';
          mostrarError('');
          actualizarVistaMonto();
        }
      })
    );

    contenedor.querySelector('#btn-ext-continuar').addEventListener('click', continuar);
    contenedor.querySelector('#btn-ext-corregir').addEventListener('click', corregir);
    contenedor.querySelector('#btn-ext-cancelar').addEventListener('click', cancelar);

    app.configurarBotonesLaterales({
      L1: () => seleccionarMontoRapido(montosRapidos[0]),
      L2: () => seleccionarMontoRapido(montosRapidos[1]),
      L3: () => seleccionarMontoRapido(montosRapidos[2]),
      L4: () => seleccionarMontoRapido(montosRapidos[3]),
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

  global.ATMScreens.extraccion = {
    titulo: 'Extracción',

    render(contenedor, app) {
      if (!global.ATMState.datos.autenticado) {
        app.irA('bienvenida');
        return {};
      }

      const cuentaYaElegida = global.ATMState.datos.cuentaSeleccionadaId;
      if (cuentaYaElegida && global.ATMState.datos.extraccion.montoConfirmado > 0) {
        const cta = global.ATMData.obtenerResumenCuenta(cuentaYaElegida);
        global.ATMState.datos.extraccion.montoConfirmado = 0;
        return renderIngresoMonto(contenedor, app, cta);
      }

      global.ATMState.datos.extraccion.montoInput = '';
      global.ATMState.datos.extraccion.montoConfirmado = 0;

      let controladorActual = {};
      const resSeleccion = global.ATMCommon.renderSeleccionCuenta(contenedor, app, {
        titulo: 'Extracción',
        mensaje: 'Seleccioná la cuenta de origen',
        mostrarSaldo: false,
        etiquetaVolver: 'Cancelar',
        alElegir: (cuenta) => {
          controladorActual = renderIngresoMonto(contenedor, app, cuenta) || {};
        },
        alCancelar: () => {
          global.ATMState.limpiarExtraccion();
          app.irA('menu');
        }
      });

      return {
        onKeyDown(e) {
          if (controladorActual && typeof controladorActual.onKeyDown === 'function') {
            controladorActual.onKeyDown(e);
          } else if (resSeleccion && typeof resSeleccion.onKeyDown === 'function') {
            resSeleccion.onKeyDown(e);
          }
        }
      };
    }
  };
})(window);
