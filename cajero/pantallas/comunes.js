/**
 * Componentes reutilizables para flujos del ATM:
 * - Selección de cuenta
 * - Selección de recibo
 * - Procesamiento animado y comprobante final
 */
(function (global) {
  'use strict';

  function crearHtmlComprobanteGenerico(tituloTicket, numeroOperacion, filas, esImpreso) {
    const rowsHtml = filas
      .map(
        (fila) =>
          `<div class="receipt-row"><span>${fila.etiqueta}</span><strong>${fila.valor}</strong></div>`
      )
      .join('');

    return `
      <div class="receipt-card ${esImpreso ? 'is-thermal' : ''}">
        <div class="receipt-header">
          <strong>SUDAMERIS · ${esImpreso ? 'TICKET IMPRESO' : tituloTicket}</strong>
          <span>${numeroOperacion}</span>
        </div>
        <div class="receipt-rows">${rowsHtml}</div>
      </div>
    `;
  }

  const ATMCommon = {
    /**
     * Renderiza la selección de cuenta si hay más de una disponible.
     * Si hay una sola cuenta, la selecciona automáticamente y continúa.
     */
    renderSeleccionCuenta(contenedor, app, {
      titulo,
      mensaje,
      cuentas,
      mostrarSaldo = false,
      etiquetaVolver = 'Cancelar',
      alElegir,
      alCancelar
    }) {
      const lista = cuentas || global.ATMData.obtenerCuentas();
      if (lista.length === 0) {
        if (alCancelar) alCancelar();
        return {};
      }

      if (lista.length === 1) {
        global.ATMState.seleccionarCuenta(lista[0].id);
        alElegir(lista[0]);
        return {};
      }

      app.establecerTitulo(titulo || 'Seleccionar cuenta');
      app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });

      const opcionesHtml = lista
        .map(
          (cta, idx) => `
          <button type="button" class="account-choice-btn" data-cta-idx="${idx}">
            <div class="account-choice-meta">
              <strong>${cta.tipoCuenta}</strong>
              <span>${cta.numeroEnmascarado} · ${cta.moneda}</span>
            </div>
            ${
              mostrarSaldo
                ? `<div class="account-choice-balance">${app.formatearGuaranies(cta.saldoDisponible)}</div>`
                : '<span class="atm-btn-arrow" aria-hidden="true">▶</span>'
            }
          </button>
        `
        )
        .join('');

      contenedor.innerHTML = `
        <div class="atm-grid-layout">
          <div class="atm-center-full" style="gap: 14px;">
            <h2 class="atm-headline" style="font-size: 22px;">
              ${mensaje || 'Seleccioná la cuenta con la que deseás operar'}
            </h2>
            <div class="account-choice-list">${opcionesHtml}</div>
          </div>

          <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-cta-cancelar">
            <span>${etiquetaVolver}</span>
          </button>
        </div>
      `;

      contenedor.querySelectorAll('[data-cta-idx]').forEach((btn) => {
        btn.addEventListener('click', () => {
          const cuenta = lista[Number(btn.dataset.ctaIdx)];
          global.ATMState.seleccionarCuenta(cuenta.id);
          alElegir(cuenta);
        });
      });

      const cancelar = () => {
        if (alCancelar) alCancelar();
        else app.irA('menu');
      };

      contenedor.querySelector('#btn-cta-cancelar').addEventListener('click', cancelar);

      const mapaLaterales = { R4: cancelar };
      if (lista[0]) mapaLaterales.L1 = () => { global.ATMState.seleccionarCuenta(lista[0].id); alElegir(lista[0]); };
      if (lista[1]) mapaLaterales.L2 = () => { global.ATMState.seleccionarCuenta(lista[1].id); alElegir(lista[1]); };
      app.configurarBotonesLaterales(mapaLaterales);

      return {
        onKeyDown(e) {
          if (e.key === 'Escape') {
            e.preventDefault();
            cancelar();
          }
        }
      };
    },

    /**
     * Renderiza la pantalla reutilizable de selección de comprobante.
     */
    renderSeleccionRecibo(contenedor, app, {
      titulo,
      subtitulo = '¿Deseás un recibo?',
      resumenTexto = '',
      alElegir,
      alVolver
    }) {
      app.establecerTitulo(titulo || 'Comprobante');
      app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });

      contenedor.innerHTML = `
        <div class="atm-grid-layout">
          <div class="atm-center-top">
            <h2 class="atm-headline">${subtitulo}</h2>
            ${resumenTexto ? `<div class="amount-summary-pill">${resumenTexto}</div>` : ''}
          </div>

          <button type="button" class="atm-btn atm-btn--secondary atm-btn--left slot-L4" id="btn-rec-volver">
            <span class="atm-btn-arrow" aria-hidden="true">◀</span>
            <span>Volver</span>
          </button>

          <button type="button" class="atm-btn atm-btn--right slot-R2" data-rec="imprimir">
            <span>Imprimir</span>
          </button>
          <button type="button" class="atm-btn atm-btn--right slot-R3" data-rec="pantalla">
            <span>Pantalla</span>
          </button>
          <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R4" data-rec="no">
            <span>No</span>
          </button>
        </div>
      `;

      const volver = () => {
        if (alVolver) alVolver();
        else app.irA('menu');
      };

      contenedor.querySelector('#btn-rec-volver').addEventListener('click', volver);
      contenedor.querySelectorAll('[data-rec]').forEach((btn) => {
        btn.addEventListener('click', () => alElegir(btn.dataset.rec));
      });

      app.configurarBotonesLaterales({
        L4: volver,
        R2: () => alElegir('imprimir'),
        R3: () => alElegir('pantalla'),
        R4: () => alElegir('no')
      });

      return {
        onKeyDown(e) {
          if (e.key === 'Escape') {
            e.preventDefault();
            volver();
          }
        }
      };
    },

    /**
     * Ejecuta animación de procesamiento y muestra comprobante o resultado.
     */
    renderProcesandoYResultado(contenedor, app, {
      titulo,
      mensajeProceso = 'Procesando operación...',
      submensajeProceso = 'Por favor, esperá un momento...',
      tipoRecibo = 'no',
      ejecutar,
      tituloExito = 'Operación realizada correctamente',
      construirComprobante,
      permitirVolverMenu = true,
      permitirFinalizarSesion = true
    }) {
      app.establecerTitulo(titulo || 'Procesando');
      app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: 'active' });
      app.configurarBotonesLaterales({});

      contenedor.innerHTML = `
        <div class="center-message-stage">
          <h2 class="atm-headline">${mensajeProceso}</h2>
          <p class="atm-subtext">${submensajeProceso}</p>
          <div class="loading-squares" aria-label="Procesando">
            <span class="loading-square"></span>
            <span class="loading-square"></span>
            <span class="loading-square"></span>
          </div>
        </div>
      `;

      global.ATMState.registrarTimeout(() => {
        const res = ejecutar();
        if (!res || !res.ok) {
          app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });
          const volverMenu = () => app.irA('menu');

          contenedor.innerHTML = `
            <div class="atm-grid-layout">
              <div class="atm-center-full">
                <div class="feedback-msg feedback-msg--error" style="font-size: 15px;">
                  ${(res && res.mensaje) || 'No se pudo completar la operación.'}
                </div>
              </div>
              <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R4" id="btn-err-volver">
                <span>Volver al menú</span>
              </button>
            </div>
          `;
          contenedor.querySelector('#btn-err-volver').addEventListener('click', volverMenu);
          app.configurarBotonesLaterales({ R4: volverMenu });
          return;
        }

        const info = construirComprobante(res.operacion);
        const esImpreso = tipoRecibo === 'imprimir';
        const muestraTicket = tipoRecibo === 'imprimir' || tipoRecibo === 'pantalla';

        app.actualizarIndicadoresHardware({
          tarjeta: 'active',
          efectivo: esImpreso ? 'warning' : ''
        });

        const irMenu = () => {
          global.ATMState.limpiarOperacionTemporal();
          app.irA('menu');
        };
        const irFinalizada = () => {
          global.ATMState.limpiarOperacionTemporal();
          app.irA('finalizada');
        };

        const botonesHtml = [
          permitirVolverMenu
            ? '<button type="button" class="atm-btn atm-btn--secondary atm-btn--left slot-L4" id="btn-res-menu"><span class="atm-btn-arrow" aria-hidden="true">◀</span><span>Volver al menú</span></button>'
            : '',
          permitirFinalizarSesion
            ? '<button type="button" class="atm-btn atm-btn--right slot-R4" id="btn-res-salir"><span>Finalizar sesión</span></button>'
            : ''
        ].join('');

        contenedor.innerHTML = `
          <div class="atm-grid-layout">
            <div class="atm-center-full" style="gap: 12px;">
              <div class="status-badge-ok">✓ ${tituloExito}</div>
              ${
                muestraTicket
                  ? crearHtmlComprobanteGenerico(
                      info.tituloTicket || 'COMPROBANTE',
                      res.operacion.numeroOperacion,
                      info.filas,
                      esImpreso
                    )
                  : `<p class="atm-headline" style="font-size: 21px;">${info.mensajeSinRecibo || tituloExito}</p>
                     <span class="simulation-tag">Operación simulada · ${res.operacion.numeroOperacion}</span>`
              }
            </div>
            ${botonesHtml}
          </div>
        `;

        const mapa = {};
        if (permitirVolverMenu) {
          contenedor.querySelector('#btn-res-menu').addEventListener('click', irMenu);
          mapa.L4 = irMenu;
        }
        if (permitirFinalizarSesion) {
          contenedor.querySelector('#btn-res-salir').addEventListener('click', irFinalizada);
          mapa.R4 = irFinalizada;
        }
        app.configurarBotonesLaterales(mapa);
      }, 1450);

      return {};
    }
  };

  global.ATMCommon = ATMCommon;
})(window);
