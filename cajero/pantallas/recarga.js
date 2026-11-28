/**
 * Apartado Recarga Telefónica: selección de operadora, número, monto y confirmación.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  function renderElegirOperadora(contenedor, app) {
    const ops = global.ATMData.obtenerOperadoras();
    app.establecerTitulo('Recarga telefónica');
    app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <button type="button" class="atm-btn atm-btn--left slot-L1" data-op="${ops[0]}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${ops[0]}</span>
        </button>
        <button type="button" class="atm-btn atm-btn--left slot-L2" data-op="${ops[1]}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${ops[1]}</span>
        </button>
        <button type="button" class="atm-btn atm-btn--left slot-L3" data-op="${ops[2]}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${ops[2]}</span>
        </button>

        <div class="atm-center-stage">
          <h2 class="atm-headline" style="font-size: 22px;">Seleccioná la operadora</h2>
          <p class="atm-subtext" style="font-size: 14px;">Recarga simulada de saldo móvil en Paraguay.</p>
        </div>

        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R4" id="btn-rec-volver">
          <span>Volver</span>
        </button>
      </div>
    `;

    const elegir = (op) => renderIngresarTelefono(contenedor, app, op);
    const volver = () => app.irA('servicios');

    contenedor.querySelectorAll('[data-op]').forEach((b) => {
      b.addEventListener('click', () => elegir(b.dataset.op));
    });
    contenedor.querySelector('#btn-rec-volver').addEventListener('click', volver);

    app.configurarBotonesLaterales({
      L1: () => elegir(ops[0]),
      L2: () => elegir(ops[1]),
      L3: () => elegir(ops[2]),
      R4: volver
    });
  }

  function renderIngresarTelefono(contenedor, app, operadora) {
    app.establecerTitulo(`Recarga · ${operadora}`);
    let telefono = '';

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-wide" style="gap: 8px;">
          <h2 class="atm-headline" style="font-size: 21px;">Ingresá el número de línea</h2>
          <div class="amount-display-box is-placeholder" id="tel-display">09XX XXX XXX</div>
          <div id="tel-teclado-slot"></div>
          <p class="extraction-hint">Formato Paraguay: 10 dígitos iniciando con 09 (ej. 0981123456)</p>
          <div class="feedback-msg" id="tel-feedback" role="alert"></div>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-tel-continuar">
          <span>Continuar</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R3" id="btn-tel-corregir">
          <span>Corregir</span>
        </button>
        <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-tel-cancelar">
          <span>Cancelar</span>
        </button>
      </div>
    `;

    const displayEl = contenedor.querySelector('#tel-display');
    const feedbackEl = contenedor.querySelector('#tel-feedback');

    const mostrarError = (msg) => {
      feedbackEl.textContent = msg || '';
      feedbackEl.className = 'feedback-msg' + (msg ? ' feedback-msg--error' : '');
    };

    const actualizar = () => {
      displayEl.textContent = telefono || '09XX XXX XXX';
      displayEl.classList.toggle('is-placeholder', !telefono);
    };

    const agregar = (d) => {
      if (telefono.length >= 10) return;
      telefono += d;
      mostrarError('');
      actualizar();
    };

    const corregir = () => {
      if (telefono.length > 0) {
        telefono = telefono.slice(0, -1);
        actualizar();
      }
      mostrarError('');
    };

    const cancelar = () => app.irA('servicios');

    const continuar = () => {
      const val = global.ATMData.validarTelefonoParaguay(telefono);
      if (!val.ok) {
        mostrarError(val.mensaje);
        return;
      }
      renderElegirMontoRecarga(contenedor, app, operadora, val.telefono);
    };

    contenedor.querySelector('#tel-teclado-slot').appendChild(
      app.crearTecladoNumerico({
        onDigit: agregar,
        onBackspace: corregir,
        onClear: () => {
          telefono = '';
          mostrarError('');
          actualizar();
        }
      })
    );

    contenedor.querySelector('#btn-tel-continuar').addEventListener('click', continuar);
    contenedor.querySelector('#btn-tel-corregir').addEventListener('click', corregir);
    contenedor.querySelector('#btn-tel-cancelar').addEventListener('click', cancelar);
    app.configurarBotonesLaterales({ R2: continuar, R3: corregir, R4: cancelar });
  }

  function renderElegirMontoRecarga(contenedor, app, operadora, telefono) {
    const montos = global.ATMData.CONFIG_DEMO.MONTOS_RECARGA;
    app.establecerTitulo(`Recarga · ${operadora}`);
    let montoInput = '';

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <button type="button" class="atm-btn atm-btn--left slot-L1" data-rm="${montos[0]}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${app.formatearGuaranies(montos[0])}</span>
        </button>
        <button type="button" class="atm-btn atm-btn--left slot-L2" data-rm="${montos[1]}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${app.formatearGuaranies(montos[1])}</span>
        </button>
        <button type="button" class="atm-btn atm-btn--left slot-L3" data-rm="${montos[2]}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${app.formatearGuaranies(montos[2])}</span>
        </button>
        <button type="button" class="atm-btn atm-btn--left slot-L4" data-rm="${montos[3]}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${app.formatearGuaranies(montos[3])}</span>
        </button>

        <div class="atm-center-stage">
          <span class="simulation-tag" style="margin-bottom: 6px;">${operadora} · ${global.ATMData.enmascararTelefono(telefono)}</span>
          <h2 class="atm-headline" style="font-size: 20px; margin-bottom: 6px;">Seleccioná o digitá el monto</h2>
          <div class="amount-display-box is-placeholder" id="rec-monto-display">Gs. 0</div>
          <div id="rec-teclado-slot"></div>
          <div class="feedback-msg" id="rec-monto-feedback" role="alert"></div>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-rm-continuar">
          <span>Continuar</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R3" id="btn-rm-corregir">
          <span>Corregir</span>
        </button>
        <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-rm-cancelar">
          <span>Cancelar</span>
        </button>
      </div>
    `;

    const displayEl = contenedor.querySelector('#rec-monto-display');
    const feedbackEl = contenedor.querySelector('#rec-monto-feedback');

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
      if (montoInput.length >= 6) return;
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

    const cancelar = () => app.irA('servicios');

    const continuar = () => {
      const monto = montoInput ? parseInt(montoInput, 10) : 0;
      const valMonto = global.ATMData.validarRecarga(monto);
      if (!valMonto.ok) {
        mostrarError(valMonto.mensaje);
        return;
      }

      global.ATMCommon.renderSeleccionCuenta(contenedor, app, {
        titulo: `Recarga · ${operadora}`,
        mensaje: 'Seleccioná la cuenta a debitar',
        mostrarSaldo: true,
        etiquetaVolver: 'Cancelar',
        alElegir: (cuenta) => renderConfirmarRecarga(contenedor, app, operadora, telefono, monto, cuenta),
        alCancelar: cancelar
      });
    };

    contenedor.querySelectorAll('[data-rm]').forEach((b) => {
      b.addEventListener('click', () => elegirRapido(Number(b.dataset.rm)));
    });

    contenedor.querySelector('#rec-teclado-slot').appendChild(
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

    contenedor.querySelector('#btn-rm-continuar').addEventListener('click', continuar);
    contenedor.querySelector('#btn-rm-corregir').addEventListener('click', corregir);
    contenedor.querySelector('#btn-rm-cancelar').addEventListener('click', cancelar);

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

  function renderConfirmarRecarga(contenedor, app, operadora, telefono, monto, cuenta) {
    app.establecerTitulo(`Recarga · ${operadora}`);
    const valSaldo = global.ATMData.validarRecarga(monto, cuenta.id);

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-top" style="gap: 10px;">
          <h2 class="atm-headline" style="font-size: 22px;">¿Confirmás la recarga?</h2>
          <div class="info-summary-card">
            <h3>Resumen de recarga</h3>
            <div class="receipt-rows">
              <div class="receipt-row"><span>Operadora</span><strong>${operadora}</strong></div>
              <div class="receipt-row"><span>Número móvil</span><strong>${global.ATMData.enmascararTelefono(telefono)}</strong></div>
              <div class="receipt-row"><span>Cuenta origen</span><strong>${cuenta.tipoCuenta} · ${cuenta.numeroEnmascarado}</strong></div>
              <div class="receipt-row"><span>Monto a recargar</span><strong>${app.formatearGuaranies(monto)}</strong></div>
            </div>
          </div>
          ${valSaldo.ok ? '' : `<div class="feedback-msg feedback-msg--error">${valSaldo.mensaje}</div>`}
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-rc-confirmar" ${valSaldo.ok ? '' : 'disabled'}>
          <span>Confirmar</span>
        </button>
        <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-rc-cancelar">
          <span>Cancelar</span>
        </button>
      </div>
    `;

    const cancelar = () => app.irA('menu');
    const confirmar = () => {
      if (!valSaldo.ok) return;
      global.ATMCommon.renderSeleccionRecibo(contenedor, app, {
        titulo: `Recarga · ${operadora}`,
        subtitulo: '¿Deseás un comprobante de recarga?',
        resumenTexto: `Recarga ${operadora}: <strong>${app.formatearGuaranies(monto)}</strong>`,
        alVolver: () => renderConfirmarRecarga(contenedor, app, operadora, telefono, monto, cuenta),
        alElegir: (tipoRecibo) => {
          global.ATMCommon.renderProcesandoYResultado(contenedor, app, {
            titulo: `Recarga · ${operadora}`,
            mensajeProceso: 'Procesando recarga telefónica...',
            submensajeProceso: 'Aplicando recarga simulada...',
            tipoRecibo,
            tituloExito: 'Recarga realizada correctamente',
            ejecutar: () => global.ATMData.ejecutarRecargaSimulada(operadora, telefono, monto, cuenta.id, tipoRecibo),
            construirComprobante: (op) => ({
              tituloTicket: 'RECARGA TELEFÓNICA',
              mensajeSinRecibo: `Recargaste ${app.formatearGuaranies(op.monto)} a ${op.operadora} (${op.telefonoEnmascarado}).`,
              filas: [
                { etiqueta: 'Fecha y hora', valor: op.fechaHora },
                { etiqueta: 'Operadora', valor: op.operadora },
                { etiqueta: 'Número', valor: op.telefonoEnmascarado },
                { etiqueta: 'Cuenta debitada', valor: op.cuenta },
                { etiqueta: 'Monto recargado', valor: app.formatearGuaranies(op.monto) },
                { etiqueta: 'Saldo restante', valor: app.formatearGuaranies(op.saldoRestante) }
              ]
            })
          });
        }
      });
    };

    if (valSaldo.ok) contenedor.querySelector('#btn-rc-confirmar').addEventListener('click', confirmar);
    contenedor.querySelector('#btn-rc-cancelar').addEventListener('click', cancelar);
    app.configurarBotonesLaterales({ R2: valSaldo.ok ? confirmar : null, R4: cancelar });
  }

  global.ATMScreens.recarga = {
    titulo: 'Recarga telefónica',

    render(contenedor, app) {
      if (!global.ATMState.datos.autenticado) {
        app.irA('bienvenida');
        return {};
      }
      renderElegirOperadora(contenedor, app);
      return {
        onKeyDown(e) {
          if (e.key === 'Escape') {
            e.preventDefault();
            app.irA('servicios');
          }
        }
      };
    }
  };
})(window);
