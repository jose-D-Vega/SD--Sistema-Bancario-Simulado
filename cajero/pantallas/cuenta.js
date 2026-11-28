/**
 * Pantalla Consulta de Saldo: selección de cuenta, elección Pantalla/Imprimir y resultado.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  function renderElegirMedio(contenedor, app, cuenta) {
    app.establecerTitulo('Consulta de saldo');
    app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-top" style="gap: 12px;">
          <span class="simulation-tag">${cuenta.tipoCuenta} · ${cuenta.numeroEnmascarado}</span>
          <h2 class="atm-headline" style="font-size: 25px;">
            ¿Cómo deseás consultar tu saldo?
          </h2>
          <p class="atm-subtext" style="font-size: 16px;">
            En caso que desees imprimir el recibo, seleccioná <strong>"Imprimir"</strong>.
          </p>
          <div class="consulta-arrow-wrap" aria-hidden="true">
            <svg width="42" height="36" viewBox="0 0 42 36" fill="none">
              <path d="M8 4C10 18 20 26 35 28" stroke="currentColor" stroke-width="3.2" stroke-linecap="round"/>
              <path d="M28 21L36 28L27 33" stroke="currentColor" stroke-width="3.2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </div>
        </div>

        <button type="button" class="atm-btn atm-btn--secondary atm-btn--left slot-L4" id="btn-cs-menu">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>Volver al menú</span>
        </button>

        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-cs-imprimir">
          <span>Imprimir</span>
        </button>
        <button type="button" class="atm-btn atm-btn--right slot-R3" id="btn-cs-pantalla">
          <span>Pantalla</span>
        </button>
        <button type="button" class="atm-btn atm-btn--danger atm-btn--right slot-R4" id="btn-cs-salir">
          <span>Salir</span>
        </button>
      </div>
    `;

    const volverMenu = () => app.irA('menu');
    const salirSesion = () => app.irA('finalizada', { motivo: 'salida' });
    const verPantalla = () => renderResultadoPantalla(contenedor, app, cuenta.id);
    const imprimirTicket = () => renderResultadoImpreso(contenedor, app, cuenta.id);

    contenedor.querySelector('#btn-cs-menu').addEventListener('click', volverMenu);
    contenedor.querySelector('#btn-cs-imprimir').addEventListener('click', imprimirTicket);
    contenedor.querySelector('#btn-cs-pantalla').addEventListener('click', verPantalla);
    contenedor.querySelector('#btn-cs-salir').addEventListener('click', salirSesion);

    app.configurarBotonesLaterales({
      L4: volverMenu,
      R2: imprimirTicket,
      R3: verPantalla,
      R4: salirSesion
    });
  }

  function renderResultadoPantalla(contenedor, app, cuentaId) {
    const cuenta = global.ATMData.obtenerResumenCuenta(cuentaId);
    app.establecerTitulo('Consulta de saldo');
    app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-full" style="gap: 10px;">
          <div class="status-badge-ok">✓ Consulta exitosa</div>

          <div class="account-card">
            <div class="account-balance-highlight">
              <span>Saldo disponible</span>
              <strong>${app.formatearGuaranies(cuenta.saldoDisponible)}</strong>
            </div>

            <div class="receipt-rows">
              <div class="receipt-row">
                <span>Tipo de cuenta</span>
                <strong>${cuenta.tipoCuenta}</strong>
              </div>
              <div class="receipt-row">
                <span>Cuenta</span>
                <strong>${cuenta.numeroEnmascarado}</strong>
              </div>
              <div class="receipt-row">
                <span>Moneda</span>
                <strong>${cuenta.moneda}</strong>
              </div>
            </div>
          </div>
        </div>

        <button type="button" class="atm-btn atm-btn--left slot-L4" id="btn-csp-otra">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>Consultar otra cuenta</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R4" id="btn-csp-volver">
          <span>Volver al menú</span>
        </button>
      </div>
    `;

    const otraConsulta = () => app.irA('cuenta');
    const volverMenu = () => app.irA('menu');

    contenedor.querySelector('#btn-csp-otra').addEventListener('click', otraConsulta);
    contenedor.querySelector('#btn-csp-volver').addEventListener('click', volverMenu);
    app.configurarBotonesLaterales({ L4: otraConsulta, R4: volverMenu });
  }

  function renderResultadoImpreso(contenedor, app, cuentaId) {
    const cuenta = global.ATMData.obtenerResumenCuenta(cuentaId);
    const fechaHora = new Date().toLocaleString('es-PY', { dateStyle: 'short', timeStyle: 'short' });

    global.ATMCommon.renderProcesandoYResultado(contenedor, app, {
      titulo: 'Consulta de saldo',
      mensajeProceso: 'Consultando saldo e imprimiendo recibo...',
      submensajeProceso: 'Generando comprobante simulado...',
      tipoRecibo: 'imprimir',
      tituloExito: 'Comprobante de consulta disponible',
      ejecutar: () => ({
        ok: true,
        operacion: {
          numeroOperacion: 'CS-' + String(Math.floor(100000 + Math.random() * 900000)),
          fechaHora,
          cuenta: cuenta.numeroEnmascarado,
          tipoCuenta: cuenta.tipoCuenta,
          saldoDisponible: cuenta.saldoDisponible
        }
      }),
      construirComprobante: (op) => ({
        tituloTicket: 'CONSULTA DE SALDO',
        filas: [
          { etiqueta: 'Banco', valor: 'SUDAMERIS' },
          { etiqueta: 'Fecha y hora', valor: op.fechaHora },
          { etiqueta: 'Operación', valor: 'Consulta de saldo' },
          { etiqueta: 'Tipo de cuenta', valor: op.tipoCuenta },
          { etiqueta: 'Cuenta', valor: op.cuenta },
          { etiqueta: 'Saldo disponible', valor: app.formatearGuaranies(op.saldoDisponible) }
        ]
      })
    });
  }

  global.ATMScreens.cuenta = {
    titulo: 'Consulta de saldo',

    render(contenedor, app) {
      if (!global.ATMState.datos.autenticado) {
        app.irA('bienvenida');
        return {};
      }

      return global.ATMCommon.renderSeleccionCuenta(contenedor, app, {
        titulo: 'Consulta de saldo',
        mensaje: 'Seleccioná la cuenta que deseás consultar',
        mostrarSaldo: false,
        etiquetaVolver: 'Volver al menú',
        alElegir: (cuenta) => renderElegirMedio(contenedor, app, cuenta),
        alCancelar: () => app.irA('menu')
      });
    }
  };
})(window);
