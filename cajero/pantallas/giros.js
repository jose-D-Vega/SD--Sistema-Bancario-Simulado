/**
 * Apartado Giros Simple: retiro simulado de un giro recibido mediante código.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  function renderIngresoCodigo(contenedor, app) {
    app.establecerTitulo('Giros Simple');
    app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });

    let codigo = '';

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-wide" style="gap: 10px;">
          <h2 class="atm-headline" style="font-size: 23px;">Retiro de giro</h2>
          <p class="atm-subtext" style="font-size: 14px;">
            Ingresá el código del giro que deseás retirar.
          </p>

          <div class="amount-display-box is-placeholder" id="giro-codigo-display">------</div>
          <div id="giro-teclado-slot"></div>
          <p class="extraction-hint">Códigos de prueba: 741852 · 852963</p>
          <div class="feedback-msg" id="giro-feedback" role="alert"></div>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-giro-continuar">
          <span>Continuar</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R3" id="btn-giro-corregir">
          <span>Corregir</span>
        </button>
        <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-giro-cancelar">
          <span>Cancelar</span>
        </button>
      </div>
    `;

    const displayEl = contenedor.querySelector('#giro-codigo-display');
    const feedbackEl = contenedor.querySelector('#giro-feedback');

    const mostrarError = (msg) => {
      feedbackEl.textContent = msg || '';
      feedbackEl.className = 'feedback-msg' + (msg ? ' feedback-msg--error' : '');
    };

    const actualizarDisplay = () => {
      displayEl.textContent = codigo || '------';
      displayEl.classList.toggle('is-placeholder', !codigo);
    };

    const agregar = (d) => {
      if (codigo.length >= 10) return;
      codigo += d;
      mostrarError('');
      actualizarDisplay();
    };

    const corregir = () => {
      if (codigo.length > 0) {
        codigo = codigo.slice(0, -1);
        actualizarDisplay();
      }
      mostrarError('');
    };

    const cancelar = () => app.irA('menu');

    const continuar = () => {
      const res = global.ATMData.consultarGiro(codigo);
      if (!res.ok) {
        mostrarError(res.mensaje);
        return;
      }
      renderConfirmacionGiro(contenedor, app, res.giro);
    };

    contenedor.querySelector('#giro-teclado-slot').appendChild(
      app.crearTecladoNumerico({
        onDigit: agregar,
        onBackspace: corregir,
        onClear: () => {
          codigo = '';
          mostrarError('');
          actualizarDisplay();
        }
      })
    );

    contenedor.querySelector('#btn-giro-continuar').addEventListener('click', continuar);
    contenedor.querySelector('#btn-giro-corregir').addEventListener('click', corregir);
    contenedor.querySelector('#btn-giro-cancelar').addEventListener('click', cancelar);

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

  function renderConfirmacionGiro(contenedor, app, giro) {
    app.establecerTitulo('Retiro de giro');

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-top" style="gap: 12px;">
          <h2 class="atm-headline" style="font-size: 23px;">¿Deseás retirar este giro?</h2>
          <div class="info-summary-card">
            <h3>Detalle del giro</h3>
            <div class="receipt-rows">
              <div class="receipt-row"><span>Código</span><strong>${giro.codigoEnmascarado}</strong></div>
              <div class="receipt-row"><span>Remitente</span><strong>${giro.remitente}</strong></div>
              <div class="receipt-row"><span>Estado</span><strong>${giro.estado}</strong></div>
              <div class="receipt-row"><span>Moneda</span><strong>${giro.moneda}</strong></div>
              <div class="receipt-row"><span>Monto a entregar</span><strong>${app.formatearGuaranies(giro.monto)}</strong></div>
            </div>
          </div>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-giroc-confirmar">
          <span>Confirmar</span>
        </button>
        <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-giroc-cancelar">
          <span>Cancelar</span>
        </button>
      </div>
    `;

    const confirmar = () => {
      global.ATMCommon.renderSeleccionRecibo(contenedor, app, {
        titulo: 'Retiro de giro',
        subtitulo: '¿Deseás un recibo por tu retiro?',
        resumenTexto: `Monto a retirar: <strong>${app.formatearGuaranies(giro.monto)}</strong>`,
        alVolver: () => renderConfirmacionGiro(contenedor, app, giro),
        alElegir: (tipoRecibo) => {
          global.ATMCommon.renderProcesandoYResultado(contenedor, app, {
            titulo: 'Retiro de giro',
            mensajeProceso: 'Procesando retiro de giro...',
            submensajeProceso: 'Preparando efectivo...',
            tipoRecibo,
            tituloExito: 'Giro retirado correctamente',
            ejecutar: () => global.ATMData.retirarGiroSimulado(giro.codigo, tipoRecibo),
            construirComprobante: (op) => ({
              tituloTicket: 'RETIRO DE GIRO',
              mensajeSinRecibo: `Se entregó ${app.formatearGuaranies(op.monto)}. No olvides retirar tu tarjeta al finalizar.`,
              filas: [
                { etiqueta: 'Fecha y hora', valor: op.fechaHora },
                { etiqueta: 'Cajero', valor: op.cajero },
                { etiqueta: 'Código giro', valor: op.codigoEnmascarado },
                { etiqueta: 'Remitente', valor: op.remitente },
                { etiqueta: 'Monto retirado', valor: app.formatearGuaranies(op.monto) }
              ]
            })
          });
        }
      });
    };

    const cancelar = () => app.irA('menu');

    contenedor.querySelector('#btn-giroc-confirmar').addEventListener('click', confirmar);
    contenedor.querySelector('#btn-giroc-cancelar').addEventListener('click', cancelar);
    app.configurarBotonesLaterales({ R2: confirmar, R4: cancelar });
  }

  global.ATMScreens.giros = {
    titulo: 'Giros Simple',

    render(contenedor, app) {
      if (!global.ATMState.datos.autenticado) {
        app.irA('bienvenida');
        return {};
      }
      return renderIngresoCodigo(contenedor, app);
    }
  };
})(window);
