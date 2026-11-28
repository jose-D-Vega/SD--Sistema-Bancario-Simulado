/**
 * Flujo simulado de Transferencias: entre cuentas propias y a otra cuenta Sudameris.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  function renderTipoTransferencia(contenedor, app) {
    app.establecerTitulo('Transferencias');
    app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-wide">
          <h2 class="atm-headline" style="font-size: 23px;">Seleccioná el tipo de transferencia</h2>
          <p class="atm-subtext" style="font-size: 14px;">
            Operación simulada dentro de la red Sudameris.
          </p>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R1" id="btn-tr-propias">
          <span>Entre mis cuentas</span>
        </button>
        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-tr-tercero">
          <span>A otra cuenta Sudameris</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R4" id="btn-tr-volver">
          <span>Volver</span>
        </button>
      </div>
    `;

    const iniciar = (tipo) => {
      global.ATMCommon.renderSeleccionCuenta(contenedor, app, {
        titulo: 'Transferencias',
        mensaje: 'Seleccioná la cuenta de origen',
        mostrarSaldo: true,
        etiquetaVolver: 'Cancelar',
        alElegir: (origen) => {
          if (tipo === 'propia') {
            renderDestinoPropia(contenedor, app, origen);
          } else {
            renderDestinoTercero(contenedor, app, origen);
          }
        },
        alCancelar: () => renderTipoTransferencia(contenedor, app)
      });
    };

    const volver = () => app.irA('otrasOperaciones');

    contenedor.querySelector('#btn-tr-propias').addEventListener('click', () => iniciar('propia'));
    contenedor.querySelector('#btn-tr-tercero').addEventListener('click', () => iniciar('tercero'));
    contenedor.querySelector('#btn-tr-volver').addEventListener('click', volver);

    app.configurarBotonesLaterales({
      R1: () => iniciar('propia'),
      R2: () => iniciar('tercero'),
      R4: volver
    });
  }

  function renderDestinoPropia(contenedor, app, origen) {
    const otras = global.ATMData.obtenerCuentas().filter((c) => c.id !== origen.id);
    if (otras.length === 0) {
      app.irA('menu');
      return;
    }

    global.ATMCommon.renderSeleccionCuenta(contenedor, app, {
      titulo: 'Transferencia · Destino',
      mensaje: 'Seleccioná tu cuenta de destino',
      cuentas: otras,
      mostrarSaldo: true,
      etiquetaVolver: 'Cancelar',
      alElegir: (destino) => {
        renderMontoTransferencia(contenedor, app, {
          tipo: 'propia',
          origen,
          destinoCuentaId: destino.id,
          destinoEnmascarado: `${destino.tipoCuenta} · ${destino.numeroEnmascarado}`,
          destinoNombre: 'Cuenta propia',
          referencia: 'Traspaso entre cuentas'
        });
      },
      alCancelar: () => renderTipoTransferencia(contenedor, app)
    });
  }

  function renderDestinoTercero(contenedor, app, origen) {
    app.establecerTitulo('Transferencia · Destino');
    const demos = global.ATMData.obtenerDestinatariosDemo();

    const chipsHtml = demos
      .map(
        (d, i) =>
          `<button type="button" class="quick-chip-btn" data-demo-dest="${i}">${d.numeroCuenta} · ${d.nombre}</button>`
      )
      .join('');

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-wide" style="gap: 10px;">
          <h2 class="atm-headline" style="font-size: 20px;">Datos de la cuenta Sudameris destino</h2>

          <div class="quick-chips-row" aria-label="Destinatarios de prueba">${chipsHtml}</div>

          <div class="atm-form-stack">
            <label>
              Número de cuenta destino
              <input type="text" inputmode="numeric" maxlength="12" class="atm-text-field" id="inp-dest-cuenta" placeholder="Ej. 10020030">
            </label>
            <label>
              Nombre ficticio del destinatario (opcional)
              <input type="text" maxlength="40" class="atm-text-field" id="inp-dest-nombre" placeholder="Ej. Lucía B. (Demo)">
            </label>
            <label>
              Referencia (opcional)
              <input type="text" maxlength="50" class="atm-text-field" id="inp-dest-ref" placeholder="Ej. Pago compartido">
            </label>
          </div>

          <div class="feedback-msg" id="tr-dest-feedback" role="alert"></div>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-trd-continuar">
          <span>Continuar</span>
        </button>
        <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-trd-cancelar">
          <span>Cancelar</span>
        </button>
      </div>
    `;

    const inpCuenta = contenedor.querySelector('#inp-dest-cuenta');
    const inpNombre = contenedor.querySelector('#inp-dest-nombre');
    const inpRef = contenedor.querySelector('#inp-dest-ref');
    const feedback = contenedor.querySelector('#tr-dest-feedback');

    const mostrarError = (msg) => {
      feedback.textContent = msg || '';
      feedback.className = 'feedback-msg' + (msg ? ' feedback-msg--error' : '');
    };

    contenedor.querySelectorAll('[data-demo-dest]').forEach((btn) => {
      btn.addEventListener('click', () => {
        const d = demos[Number(btn.dataset.demoDest)];
        inpCuenta.value = d.numeroCuenta;
        inpNombre.value = d.nombre;
        mostrarError('');
      });
    });

    const continuar = () => {
      const consulta = global.ATMData.consultarDestinatarioSudameris(inpCuenta.value);
      if (!consulta.ok) {
        mostrarError(consulta.mensaje);
        return;
      }

      const nombreFinal = inpNombre.value.trim() || consulta.destinatario.nombre;
      renderMontoTransferencia(contenedor, app, {
        tipo: 'tercero',
        origen,
        destinoEnmascarado: consulta.destinatario.enmascarado,
        destinoNombre: nombreFinal,
        referencia: inpRef.value.trim() || 'Transferencia Sudameris'
      });
    };

    const cancelar = () => renderTipoTransferencia(contenedor, app);

    contenedor.querySelector('#btn-trd-continuar').addEventListener('click', continuar);
    contenedor.querySelector('#btn-trd-cancelar').addEventListener('click', cancelar);
    app.configurarBotonesLaterales({ R2: continuar, R4: cancelar });
  }

  function renderMontoTransferencia(contenedor, app, datos) {
    app.establecerTitulo('Transferencia · Monto');
    let montoInput = '';

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-wide" style="gap: 8px;">
          <span class="simulation-tag">Disponible: ${app.formatearGuaranies(datos.origen.saldoDisponible)}</span>
          <h2 class="atm-headline" style="font-size: 21px;">Ingresá el monto a transferir</h2>
          <div class="amount-display-box is-placeholder" id="tr-monto-display">Gs. 0</div>
          <div id="tr-teclado-slot"></div>
          <div class="feedback-msg" id="tr-monto-feedback" role="alert"></div>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-trm-continuar">
          <span>Continuar</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R3" id="btn-trm-corregir">
          <span>Corregir</span>
        </button>
        <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-trm-cancelar">
          <span>Cancelar</span>
        </button>
      </div>
    `;

    const displayEl = contenedor.querySelector('#tr-monto-display');
    const feedbackEl = contenedor.querySelector('#tr-monto-feedback');

    const mostrarError = (msg) => {
      feedbackEl.textContent = msg || '';
      feedbackEl.className = 'feedback-msg' + (msg ? ' feedback-msg--error' : '');
    };

    const actualizar = () => {
      const val = montoInput ? parseInt(montoInput, 10) : 0;
      displayEl.textContent = app.formatearGuaranies(val);
      displayEl.classList.toggle('is-placeholder', val === 0);
    };

    const agregar = (d) => {
      if (montoInput.length === 0 && d === '0') return;
      if (montoInput.length >= 8) return;
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
      const val = global.ATMData.validarTransferencia(datos.origen.id, monto);
      if (!val.ok) {
        mostrarError(val.mensaje);
        return;
      }
      renderConfirmacionTransferencia(contenedor, app, { ...datos, monto });
    };

    contenedor.querySelector('#tr-teclado-slot').appendChild(
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

    contenedor.querySelector('#btn-trm-continuar').addEventListener('click', continuar);
    contenedor.querySelector('#btn-trm-corregir').addEventListener('click', corregir);
    contenedor.querySelector('#btn-trm-cancelar').addEventListener('click', cancelar);
    app.configurarBotonesLaterales({ R2: continuar, R3: corregir, R4: cancelar });
  }

  function renderConfirmacionTransferencia(contenedor, app, datos) {
    app.establecerTitulo('Confirmar transferencia');

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-top" style="gap: 10px;">
          <h2 class="atm-headline" style="font-size: 22px;">¿Confirmás la transferencia?</h2>
          <div class="info-summary-card">
            <h3>Resumen</h3>
            <div class="receipt-rows">
              <div class="receipt-row"><span>Cuenta origen</span><strong>${datos.origen.tipoCuenta} · ${datos.origen.numeroEnmascarado}</strong></div>
              <div class="receipt-row"><span>Destino</span><strong>${datos.destinoEnmascarado}</strong></div>
              <div class="receipt-row"><span>Beneficiario</span><strong>${datos.destinoNombre}</strong></div>
              <div class="receipt-row"><span>Referencia</span><strong>${datos.referencia}</strong></div>
              <div class="receipt-row"><span>Monto</span><strong>${app.formatearGuaranies(datos.monto)}</strong></div>
            </div>
          </div>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-trc-confirmar">
          <span>Confirmar</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R3" id="btn-trc-corregir">
          <span>Corregir</span>
        </button>
        <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-trc-cancelar">
          <span>Cancelar</span>
        </button>
      </div>
    `;

    const corregir = () => renderMontoTransferencia(contenedor, app, datos);
    const cancelar = () => app.irA('menu');

    const confirmar = () => {
      global.ATMCommon.renderSeleccionRecibo(contenedor, app, {
        titulo: 'Transferencias',
        subtitulo: '¿Deseás un comprobante de transferencia?',
        resumenTexto: `Monto: <strong>${app.formatearGuaranies(datos.monto)}</strong>`,
        alVolver: () => renderConfirmacionTransferencia(contenedor, app, datos),
        alElegir: (tipoRecibo) => {
          global.ATMCommon.renderProcesandoYResultado(contenedor, app, {
            titulo: 'Transferencias',
            mensajeProceso: 'Procesando transferencia...',
            submensajeProceso: 'Actualizando saldos simulados...',
            tipoRecibo,
            tituloExito: 'Transferencia realizada correctamente',
            ejecutar: () =>
              global.ATMData.ejecutarTransferenciaSimulada({
                origenId: datos.origen.id,
                tipo: datos.tipo,
                destinoCuentaId: datos.destinoCuentaId,
                destinoEnmascarado: datos.destinoEnmascarado,
                destinoNombre: datos.destinoNombre,
                referencia: datos.referencia,
                monto: datos.monto,
                tipoRecibo
              }),
            construirComprobante: (op) => ({
              tituloTicket: 'TRANSFERENCIA',
              mensajeSinRecibo: `Transferiste ${app.formatearGuaranies(op.monto)} a ${op.beneficiario}.`,
              filas: [
                { etiqueta: 'Fecha y hora', valor: op.fechaHora },
                { etiqueta: 'Origen', valor: op.origen },
                { etiqueta: 'Destino', valor: op.destino },
                { etiqueta: 'Beneficiario', valor: op.beneficiario },
                { etiqueta: 'Referencia', valor: op.referencia },
                { etiqueta: 'Monto transferido', valor: app.formatearGuaranies(op.monto) },
                { etiqueta: 'Saldo restante', valor: app.formatearGuaranies(op.saldoRestante) }
              ]
            })
          });
        }
      });
    };

    contenedor.querySelector('#btn-trc-confirmar').addEventListener('click', confirmar);
    contenedor.querySelector('#btn-trc-corregir').addEventListener('click', corregir);
    contenedor.querySelector('#btn-trc-cancelar').addEventListener('click', cancelar);
    app.configurarBotonesLaterales({ R2: confirmar, R3: corregir, R4: cancelar });
  }

  global.ATMScreens.transferencias = {
    titulo: 'Transferencias',

    render(contenedor, app) {
      if (!global.ATMState.datos.autenticado) {
        app.irA('bienvenida');
        return {};
      }
      renderTipoTransferencia(contenedor, app);
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
