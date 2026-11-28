/**
 * Apartado Tarjetas: consulta de tarjeta de débito, tarjeta de crédito asociada y acceso a adelanto.
 */
(function (global) {
  'use strict';

  global.ATMScreens = global.ATMScreens || {};

  function renderPrincipalTarjetas(contenedor, app) {
    const debito = global.ATMData.obtenerTarjetaDebito();
    const credito = global.ATMData.obtenerTarjetaCredito();

    app.establecerTitulo('Tarjetas');
    app.actualizarIndicadoresHardware({ tarjeta: 'active', efectivo: '' });

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-top" style="gap: 12px;">
          <div class="card-visual-mini">
            <div class="card-visual-mini-top">
              <span>${debito.banco}</span>
              <span>${debito.estado.toUpperCase()}</span>
            </div>
            <div class="card-visual-mini-num">${debito.numeroEnmascarado}</div>
            <div class="card-visual-mini-foot">
              <span>${debito.tipo}</span>
              <span>Cuenta: ${debito.cuentaVinculada}</span>
            </div>
          </div>

          <div class="info-summary-card">
            <div class="receipt-rows">
              <div class="receipt-row"><span>Últimos 4 dígitos</span><strong>${debito.ultimos4}</strong></div>
              <div class="receipt-row"><span>Estado</span><strong>${debito.estado}</strong></div>
              <div class="receipt-row"><span>Cuenta vinculada</span><strong>${debito.tipoCuentaAsociada} · ${debito.cuentaVinculada}</strong></div>
            </div>
          </div>
        </div>

        <button type="button" class="atm-btn atm-btn--right slot-R1" id="btn-tar-consultar">
          <span>Consultar tarjeta</span>
        </button>
        <button type="button" class="atm-btn atm-btn--right slot-R2" id="btn-tar-credito" ${credito.disponible ? '' : 'disabled'}>
          <span>Saldo tarjeta de crédito</span>
        </button>
        <button type="button" class="atm-btn atm-btn--right slot-R3" id="btn-tar-adelanto" ${credito.disponible ? '' : 'disabled'}>
          <span>Adelanto de efectivo</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R4" id="btn-tar-volver">
          <span>Volver</span>
        </button>
      </div>
    `;

    const irConsulta = () => renderConsultaDebito(contenedor, app, debito);
    const irCredito = () => credito.disponible && renderConsultaCredito(contenedor, app);
    const irAdelanto = () => credito.disponible && app.irA('adelanto');
    const volver = () => app.irA('menu');

    contenedor.querySelector('#btn-tar-consultar').addEventListener('click', irConsulta);
    if (credito.disponible) {
      contenedor.querySelector('#btn-tar-credito').addEventListener('click', irCredito);
      contenedor.querySelector('#btn-tar-adelanto').addEventListener('click', irAdelanto);
    }
    contenedor.querySelector('#btn-tar-volver').addEventListener('click', volver);

    app.configurarBotonesLaterales({
      R1: irConsulta,
      R2: credito.disponible ? irCredito : null,
      R3: credito.disponible ? irAdelanto : null,
      R4: volver
    });
  }

  function renderConsultaDebito(contenedor, app, debito) {
    app.establecerTitulo('Consultar tarjeta');

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-full" style="gap: 12px;">
          <div class="status-badge-ok">✓ Consulta de tarjeta exitosa</div>
          <div class="info-summary-card">
            <h3>Datos de la tarjeta</h3>
            <div class="receipt-rows">
              <div class="receipt-row"><span>Banco</span><strong>${debito.banco}</strong></div>
              <div class="receipt-row"><span>Tipo</span><strong>${debito.tipo}</strong></div>
              <div class="receipt-row"><span>Estado</span><strong>${debito.estado}</strong></div>
              <div class="receipt-row"><span>Últimos 4 dígitos</span><strong>•••• ${debito.ultimos4}</strong></div>
              <div class="receipt-row"><span>Vencimiento</span><strong>${debito.vencimiento}</strong></div>
              <div class="receipt-row"><span>Cuenta vinculada</span><strong>${debito.tipoCuentaAsociada} · ${debito.cuentaVinculada}</strong></div>
            </div>
          </div>
        </div>

        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R4" id="btn-td-volver">
          <span>Volver</span>
        </button>
      </div>
    `;

    const volver = () => renderPrincipalTarjetas(contenedor, app);
    contenedor.querySelector('#btn-td-volver').addEventListener('click', volver);
    app.configurarBotonesLaterales({ R4: volver });
  }

  function renderConsultaCredito(contenedor, app) {
    const tc = global.ATMData.obtenerTarjetaCredito();
    app.establecerTitulo('Tarjeta de Crédito');

    contenedor.innerHTML = `
      <div class="atm-grid-layout">
        <div class="atm-center-full" style="gap: 12px;">
          <div class="card-visual-mini card-visual-mini--credito">
            <div class="card-visual-mini-top">
              <span>${tc.banco}</span>
              <span>${tc.estado.toUpperCase()}</span>
            </div>
            <div class="card-visual-mini-num">${tc.numeroEnmascarado}</div>
            <div class="card-visual-mini-foot">
              <span>${tc.tipo}</span>
              <span>Vence: ${tc.vencimiento}</span>
            </div>
          </div>

          <div class="info-summary-card">
            <div class="receipt-rows">
              <div class="receipt-row"><span>Límite de crédito</span><strong>${app.formatearGuaranies(tc.limiteCredito)}</strong></div>
              <div class="receipt-row"><span>Saldo utilizado</span><strong>${app.formatearGuaranies(tc.saldoUtilizado)}</strong></div>
              <div class="receipt-row"><span>Disponible para compras</span><strong>${app.formatearGuaranies(tc.disponibleCredito)}</strong></div>
              <div class="receipt-row"><span>Disponible para adelanto</span><strong>${app.formatearGuaranies(tc.disponibleAdelanto)}</strong></div>
            </div>
          </div>
          <span class="simulation-tag">Saldos ficticios independientes de tus cuentas bancarias</span>
        </div>

        <button type="button" class="atm-btn atm-btn--left slot-L4" id="btn-tc-adelanto">
          <span class="atm-btn-arrow" aria-hidden="true">◀</span>
          <span>Adelanto de efectivo</span>
        </button>
        <button type="button" class="atm-btn atm-btn--secondary atm-btn--right slot-R4" id="btn-tc-volver">
          <span>Volver</span>
        </button>
      </div>
    `;

    const irAdelanto = () => app.irA('adelanto');
    const volver = () => renderPrincipalTarjetas(contenedor, app);

    contenedor.querySelector('#btn-tc-adelanto').addEventListener('click', irAdelanto);
    contenedor.querySelector('#btn-tc-volver').addEventListener('click', volver);
    app.configurarBotonesLaterales({ L4: irAdelanto, R4: volver });
  }

  global.ATMScreens.tarjetas = {
    titulo: 'Tarjetas',

    render(contenedor, app) {
      if (!global.ATMState.datos.autenticado) {
        app.irA('bienvenida');
        return {};
      }
      renderPrincipalTarjetas(contenedor, app);
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
