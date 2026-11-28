/**
 * Flujo simulado de Adelanto de Efectivo con tarjeta de crédito asociada.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  function renderNoDisponible(contenedor, app) {
    app.establecerTitulo('Adelanto de efectivo');
    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-full" style="gap: 12px;">
          <h2 class="atm-headline" style="font-size: 22px;">Servicio no disponible</h2>
          <p class="atm-subtext">
            La tarjeta actual no posee una tarjeta de crédito vinculada habilitada para adelantos.
          </p>
        </div>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R4" id="btn-adnd-volver">
          <span>Volver</span>
        </button>
      </div>
    `;
    const volver = () => app.irA('otrasOperaciones');
    contenedor.querySelector('#btn-adnd-volver').addEventListener('click', volver);
    app.configurarBotonesLaterales({ R4: volver });
  }

  function renderIngresoMontoAdelanto(contenedor, app, tc) {
    app.establecerTitulo('Adelanto de efectivo');
    const montos = global.ATMData.CONFIG_DEMO.MONTOS_RAPIDOS;
    let montoInput = '';

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <button type="button" class="atm-btn atm-btn--left slot-L1" data-am="${montos[0]}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${app.formatearGuaranies(montos[0])}</span>
        </button>
        <button type="button" class="atm-btn atm-btn--left slot-L2" data-am="${montos[1]}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${app.formatearGuaranies(montos[1])}</span>
        </button>
        <button type="button" class="atm-btn atm-btn--left slot-L3" data-am="${montos[2]}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${app.formatearGuaranies(montos[2])}</span>
        </button>
        <button type="button" class="atm-btn atm-btn--left slot-L4" data-am="${montos[3]}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${app.formatearGuaranies(montos[3])}</span>
        </button>

        <div class="atm-center-stage">
          <span class="simulation-tag" style="margin-bottom: 6px;">
            Crédito ${tc.numeroEnmascarado} · Límite adelanto: ${app.formatearGuaranies(tc.disponibleAdelanto)}
          </span>
          <h2 class="atm-headline" style="font-size: 20px; margin-bottom: 6px;">Monto del adelanto</h2>
          <div class="amount-display-box is-placeholder" id="ad-monto-display">Gs. 0</div>
          <div id="ad-teclado-slot"></div>
          <div class="feedback-msg" id="ad-feedback" role="alert"></div>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-ad-continuar">
          <span>Continuar</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R3" id="btn-ad-corregir">
          <span>Corregir</span>
        </button>
        <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-ad-cancelar">
          <span>Cancelar</span>
        </button>
      </div>
    `;

    const displayEl = contenedor.querySelector('#ad-monto-display');
    const feedbackEl = contenedor.querySelector('#ad-feedback');

    const mostrarError = (msg) => {
      feedbackEl.textContent = msg || '';
      feedbackEl.className = 'feedback-msg' + (msg ? ' feedback-msg--error' : '');
    };

    const actualizar = () => {
      const val = montoInput ? parseInt(montoInput, 10) : 0;
      displayEl.textContent = app.formatearGuaranies(val);
      displayEl.classList.toggle('is-placeholder', val === 0);
    };

    const elegirRapido = (m) => {
      montoInput = String(m);
      mostrarError('');
      actualizar();
    };

    const agregar = (d) => {
      if (montoInput.length === 0 && d === '0') return;
      if (montoInput.length >= 7) return;
      montoInput += d;
      mostrarError('');
      actualizar();
    };

    const corregir = () => {
      if (montoInput.length > 0) {
        montoInput = montoInput.slice(0, -1);
        actualizar();
      }
      mostrarError('');
    };

    const cancelar = () => app.irA('menu');

    const continuar = () => {
      const monto = montoInput ? parseInt(montoInput, 10) : 0;
      const val = global.ATMData.validarAdelanto(monto);
      if (!val.ok) {
        mostrarError(val.mensaje);
        return;
      }
      renderConfirmarAdelanto(contenedor, app, tc, monto, val.comision, val.total);
    };

    contenedor.querySelectorAll('[data-am]').forEach((b) => {
      b.addEventListener('click', () => elegirRapido(Number(b.dataset.am)));
    });

    contenedor.querySelector('#ad-teclado-slot').appendChild(
      app.crearTecladoNumerico({
        onDigit: agregar,
        onBackspace: corregir,
        onClear: () => {
          montoInput = '';
          mostrarError('');
          actualizar();
        }
      })
    );

    contenedor.querySelector('#btn-ad-continuar').addEventListener('click', continuar);
    contenedor.querySelector('#btn-ad-corregir').addEventListener('click', corregir);
    contenedor.querySelector('#btn-ad-cancelar').addEventListener('click', cancelar);

    app.configurarBotonesLaterales({
      L1: () => elegirRapido(montos[0]),
      L2: () => elegirRapido(montos[1]),
      L3: () => elegirRapido(montos[2]),
      L4: () => elegirRapido(montos[3]),
      R2: continuar,
      R3: corregir,
      R4: cancelar
    });
  }

  function renderConfirmarAdelanto(contenedor, app, tc, monto, comision, total) {
    app.establecerTitulo('Confirmar adelanto');

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-top" style="gap: 10px;">
          <h2 class="atm-headline" style="font-size: 22px;">¿Confirmás el adelanto de efectivo?</h2>
          <div class="info-summary-card">
            <h3>Detalle de demostración</h3>
            <div class="receipt-rows">
              <div class="receipt-row"><span>Tarjeta de crédito</span><strong>${tc.numeroEnmascarado}</strong></div>
              <div class="receipt-row"><span>Monto a retirar</span><strong>${app.formatearGuaranies(monto)}</strong></div>
              <div class="receipt-row"><span>Comisión simulada (5%)</span><strong>${app.formatearGuaranies(comision)}</strong></div>
              <div class="receipt-row"><span>Total a confirmar</span><strong>${app.formatearGuaranies(total)}</strong></div>
            </div>
          </div>
          <span class="simulation-tag">Comisión ficticia de demostración, no representa cargos reales</span>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-adc-confirmar">
          <span>Confirmar</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R3" id="btn-adc-corregir">
          <span>Corregir</span>
        </button>
        <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-adc-cancelar">
          <span>Cancelar</span>
        </button>
      </div>
    `;

    const corregir = () => renderIngresoMontoAdelanto(contenedor, app, tc);
    const cancelar = () => app.irA('menu');

    const confirmar = () => {
      global.ATMCommon.renderSeleccionRecibo(contenedor, app, {
        titulo: 'Adelanto de efectivo',
        subtitulo: '¿Deseás un comprobante de la operación?',
        resumenTexto: `Efectivo a entregar: <strong>${app.formatearGuaranies(monto)}</strong>`,
        alVolver: () => renderConfirmarAdelanto(contenedor, app, tc, monto, comision, total),
        alElegir: (tipoRecibo) => {
          global.ATMCommon.renderProcesandoYResultado(contenedor, app, {
            titulo: 'Adelanto de efectivo',
            mensajeProceso: 'Procesando adelanto de efectivo...',
            submensajeProceso: 'Dispensando billetes...',
            tipoRecibo,
            tituloExito: 'Adelanto realizado correctamente',
            ejecutar: () => global.ATMData.ejecutarAdelantoSimulado(monto, tipoRecibo),
            construirComprobante: (op) => ({
              tituloTicket: 'ADELANTO DE EFECTIVO',
              mensajeSinRecibo: `Retirá ${app.formatearGuaranies(op.montoSolicitado)}. Disponible crédito restante: ${app.formatearGuaranies(op.disponibleRestante)}.`,
              filas: [
                { etiqueta: 'Fecha y hora', valor: op.fechaHora },
                { etiqueta: 'Tarjeta', valor: op.tarjeta },
                { etiqueta: 'Efectivo entregado', valor: app.formatearGuaranies(op.montoSolicitado) },
                { etiqueta: 'Comisión simulada', valor: app.formatearGuaranies(op.comisionSimulada) },
                { etiqueta: 'Total cargado', valor: app.formatearGuaranies(op.totalCargado) },
                { etiqueta: 'Crédito disponible', valor: app.formatearGuaranies(op.disponibleRestante) }
              ]
            })
          });
        }
      });
    };

    contenedor.querySelector('#btn-adc-confirmar').addEventListener('click', confirmar);
    contenedor.querySelector('#btn-adc-corregir').addEventListener('click', corregir);
    contenedor.querySelector('#btn-adc-cancelar').addEventListener('click', cancelar);
    app.configurarBotonesLaterales({ R2: confirmar, R3: corregir, R4: cancelar });
  }

  global.ATMScreens.adelanto = {
    titulo: 'Adelanto de efectivo',

    render(contenedor, app) {
      if (!global.ATMState.datos.autenticado) {
        app.irA('bienvenida');
        return {};
      }

      const tc = global.ATMData.obtenerTarjetaCredito();
      if (!tc.disponible) {
        renderNoDisponible(contenedor, app);
      } else {
        renderIngresoMontoAdelanto(contenedor, app, tc);
      }

      return {
        onKeyDown(e) {
          if (e.key === 'Escape') {
            e.preventDefault();
            app.irA('otrasOperaciones');
          }
        }
      };
    }
  };
})(window);
