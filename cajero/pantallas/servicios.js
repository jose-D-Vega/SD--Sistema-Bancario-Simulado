/**
 * Apartado Servicios: menú de servicios y flujo completo de pago de facturas simulado.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  function renderMenuServicios(contenedor, app) {
    app.establecerTitulo('Servicios');
    app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-wide">
          <h2 class="atm-headline" style="font-size: 24px;">Servicios y Recargas</h2>
          <p class="atm-subtext">Seleccioná el tipo de operación que deseás realizar.</p>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R1" id="btn-srv-pago">
          <span>Pago de servicios</span>
        </button>
        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-srv-recarga">
          <span>Recarga telefónica</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R4" id="btn-srv-volver">
          <span>Volver</span>
        </button>
      </div>
    `;

    const irPago = () => renderSeleccionProveedor(contenedor, app);
    const irRecarga = () => app.irA('recarga');
    const volver = () => app.irA('menu');

    contenedor.querySelector('#btn-srv-pago').addEventListener('click', irPago);
    contenedor.querySelector('#btn-srv-recarga').addEventListener('click', irRecarga);
    contenedor.querySelector('#btn-srv-volver').addEventListener('click', volver);

    app.configurarBotonesLaterales({ R1: irPago, R2: irRecarga, R4: volver });
  }

  function renderSeleccionProveedor(contenedor, app) {
    const servicios = global.ATMData.obtenerServicios();
    app.establecerTitulo('Pago de servicios');

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <button type="button" class="atm-btn atm-btn--left slot-L1" data-srv="${servicios[0].id}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${servicios[0].nombre}</span>
        </button>
        <button type="button" class="atm-btn atm-btn--left slot-L2" data-srv="${servicios[1].id}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${servicios[1].nombre}</span>
        </button>
        <button type="button" class="atm-btn atm-btn--left slot-L3" data-srv="${servicios[2].id}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${servicios[2].nombre}</span>
        </button>
        <button type="button" class="atm-btn atm-btn--left slot-L4" data-srv="${servicios[3].id}">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>${servicios[3].nombre}</span>
        </button>

        <div class="atm-center-stage">
          <h2 class="atm-headline" style="font-size: 21px;">Seleccioná el servicio</h2>
          <p class="atm-subtext" style="font-size: 13px;">Elegí la categoría o proveedor para consultar tu factura simulada.</p>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R1" data-srv="${servicios[4].id}">
          <span>${servicios[4].nombre}</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R4" id="btn-ps-volver">
          <span>Volver</span>
        </button>
      </div>
    `;

    const elegir = (id) => {
      const srv = servicios.find((s) => s.id === id);
      if (srv) renderIdentificarFactura(contenedor, app, srv);
    };
    const volver = () => renderMenuServicios(contenedor, app);

    contenedor.querySelectorAll('[data-srv]').forEach((btn) => {
      btn.addEventListener('click', () => elegir(btn.dataset.srv));
    });
    contenedor.querySelector('#btn-ps-volver').addEventListener('click', volver);

    app.configurarBotonesLaterales({
      L1: () => elegir(servicios[0].id),
      L2: () => elegir(servicios[1].id),
      L3: () => elegir(servicios[2].id),
      L4: () => elegir(servicios[3].id),
      R1: () => elegir(servicios[4].id),
      R4: volver
    });
  }

  function renderIdentificarFactura(contenedor, app, servicio) {
    app.establecerTitulo(`Pago · ${servicio.nombre}`);
    let referencia = '';

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-wide" style="gap: 8px;">
          <h2 class="atm-headline" style="font-size: 21px;">${servicio.etiquetaCampo}</h2>
          <div class="amount-display-box is-placeholder" id="srv-ref-display">Ingresá referencia</div>
          <div id="srv-teclado-slot"></div>
          <p class="extraction-hint">Referencia de prueba: ${servicio.ejemploRef}</p>
          <div class="feedback-msg" id="srv-feedback" role="alert"></div>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-srv-continuar">
          <span>Continuar</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R3" id="btn-srv-corregir">
          <span>Corregir</span>
        </button>
        <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-srv-cancelar">
          <span>Cancelar</span>
        </button>
      </div>
    `;

    const displayEl = contenedor.querySelector('#srv-ref-display');
    const feedbackEl = contenedor.querySelector('#srv-feedback');

    const mostrarError = (msg) => {
      feedbackEl.textContent = msg || '';
      feedbackEl.className = 'feedback-msg' + (msg ? ' feedback-msg--error' : '');
    };

    const actualizar = () => {
      displayEl.textContent = referencia || 'Ingresá referencia';
      displayEl.classList.toggle('is-placeholder', !referencia);
    };

    const agregar = (d) => {
      if (referencia.length >= 12) return;
      referencia += d;
      mostrarError('');
      actualizar();
    };

    const corregir = () => {
      if (referencia.length > 0) {
        referencia = referencia.slice(0, -1);
        actualizar();
      }
      mostrarError('');
    };

    const cancelar = () => renderMenuServicios(contenedor, app);

    const continuar = () => {
      const consulta = global.ATMData.consultarFacturaSimulada(servicio.id, referencia);
      if (!consulta.ok) {
        mostrarError(consulta.mensaje);
        return;
      }

      global.ATMCommon.renderSeleccionCuenta(contenedor, app, {
        titulo: `Pago · ${servicio.nombre}`,
        mensaje: 'Seleccioná la cuenta desde la cual deseás pagar',
        mostrarSaldo: true,
        etiquetaVolver: 'Cancelar',
        alElegir: (cuenta) => renderConfirmarPago(contenedor, app, consulta.factura, cuenta),
        alCancelar: () => renderMenuServicios(contenedor, app)
      });
    };

    contenedor.querySelector('#srv-teclado-slot').appendChild(
      app.crearTecladoNumerico({
        onDigit: agregar,
        onBackspace: corregir,
        onClear: () => {
          referencia = '';
          mostrarError('');
          actualizar();
        }
      })
    );

    contenedor.querySelector('#btn-srv-continuar').addEventListener('click', continuar);
    contenedor.querySelector('#btn-srv-corregir').addEventListener('click', corregir);
    contenedor.querySelector('#btn-srv-cancelar').addEventListener('click', cancelar);

    app.configurarBotonesLaterales({ R2: continuar, R3: corregir, R4: cancelar });
  }

  function renderConfirmarPago(contenedor, app, factura, cuenta) {
    app.establecerTitulo(`Pago · ${factura.servicioNombre}`);
    const saldoSuficiente = factura.importe <= cuenta.saldoDisponible;

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-top" style="gap: 10px;">
          <h2 class="atm-headline" style="font-size: 22px;">¿Confirmás el pago?</h2>
          <div class="info-summary-card">
            <h3>Factura consultada</h3>
            <div class="receipt-rows">
              <div class="receipt-row"><span>Servicio</span><strong>${factura.servicioNombre}</strong></div>
              <div class="receipt-row"><span>Empresa</span><strong>${factura.empresa}</strong></div>
              <div class="receipt-row"><span>Referencia</span><strong>${factura.referenciaEnmascarada}</strong></div>
              <div class="receipt-row"><span>Vencimiento</span><strong>${factura.vencimiento}</strong></div>
              <div class="receipt-row"><span>Estado</span><strong>${factura.estado}</strong></div>
              <div class="receipt-row"><span>Cuenta origen</span><strong>${cuenta.tipoCuenta} · ${cuenta.numeroEnmascarado}</strong></div>
              <div class="receipt-row"><span>Importe a pagar</span><strong>${app.formatearGuaranies(factura.importe)}</strong></div>
            </div>
          </div>
          ${
            saldoSuficiente
              ? ''
              : '<div class="feedback-msg feedback-msg--error">Saldo insuficiente en la cuenta seleccionada.</div>'
          }
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-psc-confirmar" ${saldoSuficiente ? '' : 'disabled'}>
          <span>Confirmar</span>
        </button>
        <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-psc-cancelar">
          <span>Cancelar</span>
        </button>
      </div>
    `;

    const cancelar = () => app.irA('menu');
    const confirmar = () => {
      if (!saldoSuficiente) return;
      global.ATMCommon.renderSeleccionRecibo(contenedor, app, {
        titulo: `Pago · ${factura.servicioNombre}`,
        subtitulo: '¿Deseás un recibo de pago?',
        resumenTexto: `Importe: <strong>${app.formatearGuaranies(factura.importe)}</strong>`,
        alVolver: () => renderConfirmarPago(contenedor, app, factura, cuenta),
        alElegir: (tipoRecibo) => {
          global.ATMCommon.renderProcesandoYResultado(contenedor, app, {
            titulo: `Pago · ${factura.servicioNombre}`,
            mensajeProceso: 'Procesando pago de servicio...',
            submensajeProceso: 'Registrando pago simulado...',
            tipoRecibo,
            tituloExito: 'Pago realizado correctamente',
            ejecutar: () => global.ATMData.pagarServicioSimulado(factura, cuenta.id, tipoRecibo),
            construirComprobante: (op) => ({
              tituloTicket: 'PAGO DE SERVICIO',
              mensajeSinRecibo: `Pagaste ${app.formatearGuaranies(op.monto)} por ${op.servicio}.`,
              filas: [
                { etiqueta: 'Fecha y hora', valor: op.fechaHora },
                { etiqueta: 'Servicio', valor: op.servicio },
                { etiqueta: 'Referencia', valor: op.referenciaEnmascarada },
                { etiqueta: 'Cuenta debitada', valor: op.cuenta },
                { etiqueta: 'Importe pagado', valor: app.formatearGuaranies(op.monto) },
                { etiqueta: 'Saldo restante', valor: app.formatearGuaranies(op.saldoRestante) }
              ]
            })
          });
        }
      });
    };

    if (saldoSuficiente) {
      contenedor.querySelector('#btn-psc-confirmar').addEventListener('click', confirmar);
    }
    contenedor.querySelector('#btn-psc-cancelar').addEventListener('click', cancelar);
    app.configurarBotonesLaterales({ R2: saldoSuficiente ? confirmar : null, R4: cancelar });
  }

  global.ATMScreens.servicios = {
    titulo: 'Servicios',

    render(contenedor, app) {
      if (!global.ATMState.datos.autenticado) {
        app.irA('bienvenida');
        return {};
      }
      renderMenuServicios(contenedor, app);
      return {
        onKeyDown(e) {
          if (e.key === 'Escape') {
            e.preventDefault();
            app.irA('menu');
          }
        }
      };
    }
  };
})(window);
