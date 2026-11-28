/**
 * Controlador principal de navegación, teclado, botones laterales y utilidades del ATM.
 */
(function (global) {
  'use strict';

  const formatoGs = new Intl.NumberFormat('es-PY');

  document.addEventListener('DOMContentLoaded', () => {
    const contenedor = document.getElementById('pantalla-contenido');
    const tituloEl = document.getElementById('screen-title');
    const relojEl = document.getElementById('atm-reloj');
    const slotTarjeta = document.getElementById('hw-card-slot');
    const slotEfectivo = document.getElementById('hw-cash-slot');
    const botonesLaterales = document.querySelectorAll('.hw-side-btn[data-side]');

    let manejadoresLaterales = {};
    let controladorPantallaActual = {};

    const actualizarReloj = () => {
      if (!relojEl) return;
      relojEl.textContent = new Date().toLocaleTimeString('es-PY', {
        hour: '2-digit',
        minute: '2-digit'
      });
    };

    actualizarReloj();
    global.setInterval(actualizarReloj, 15000);

    botonesLaterales.forEach((btn) => {
      btn.addEventListener('click', () => {
        const lado = btn.dataset.side;
        const accion = manejadoresLaterales[lado];
        if (typeof accion === 'function') {
          accion();
        }
      });
    });

    document.addEventListener('keydown', (e) => {
      const tag = e.target && e.target.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') {
        if (e.key !== 'Escape') return;
      }
      if (controladorPantallaActual && typeof controladorPantallaActual.onKeyDown === 'function') {
        controladorPantallaActual.onKeyDown(e);
      }
    });

    const app = {
      establecerTitulo(texto) {
        tituloEl.textContent = texto || '';
      },

      formatearGuaranies(monto) {
        const valor = Math.max(0, Math.round(Number(monto) || 0));
        return 'Gs. ' + formatoGs.format(valor);
      },

      configurarBotonesLaterales(mapa) {
        manejadoresLaterales = mapa || {};
        botonesLaterales.forEach((btn) => {
          const lado = btn.dataset.side;
          const activo = typeof manejadoresLaterales[lado] === 'function';
          btn.disabled = !activo;
        });
      },

      actualizarIndicadoresHardware({ tarjeta, efectivo }) {
        slotTarjeta.classList.remove('is-active', 'is-warning');
        slotEfectivo.classList.remove('is-active', 'is-warning');

        if (tarjeta === 'active') slotTarjeta.classList.add('is-active');
        if (tarjeta === 'warning') slotTarjeta.classList.add('is-warning');

        if (efectivo === 'active') slotEfectivo.classList.add('is-active');
        if (efectivo === 'warning') slotEfectivo.classList.add('is-warning');
      },

      crearTecladoNumerico({ onDigit, onBackspace, onClear }) {
        const grid = document.createElement('div');
        grid.className = 'keypad-grid';
        grid.setAttribute('role', 'group');
        grid.setAttribute('aria-label', 'Teclado numérico');

        const teclas = [
          { label: '1', tipo: 'digit', valor: '1' },
          { label: '2', tipo: 'digit', valor: '2' },
          { label: '3', tipo: 'digit', valor: '3' },
          { label: '4', tipo: 'digit', valor: '4' },
          { label: '5', tipo: 'digit', valor: '5' },
          { label: '6', tipo: 'digit', valor: '6' },
          { label: '7', tipo: 'digit', valor: '7' },
          { label: '8', tipo: 'digit', valor: '8' },
          { label: '9', tipo: 'digit', valor: '9' },
          { label: 'Limpiar', tipo: 'clear' },
          { label: '0', tipo: 'digit', valor: '0' },
          { label: '⌫', tipo: 'backspace', ariaLabel: 'Borrar último dígito' }
        ];

        teclas.forEach((tecla) => {
          const btn = document.createElement('button');
          btn.type = 'button';
          btn.className = 'keypad-key';
          btn.textContent = tecla.label;
          if (tecla.ariaLabel) btn.setAttribute('aria-label', tecla.ariaLabel);
          if (tecla.tipo === 'clear') btn.classList.add('keypad-key--clear');

          btn.addEventListener('click', () => {
            if (tecla.tipo === 'digit' && onDigit) onDigit(tecla.valor);
            if (tecla.tipo === 'backspace' && onBackspace) onBackspace();
            if (tecla.tipo === 'clear' && onClear) onClear();
          });

          grid.appendChild(btn);
        });

        return grid;
      },

      cancelarYVolverInicio() {
        global.ATMState.reiniciarSesion();
        this.irA('bienvenida');
      },

      irA(nombrePantalla, opciones) {
        const pantalla = global.ATMScreens && global.ATMScreens[nombrePantalla];
        if (!pantalla) {
          return;
        }

        global.ATMState.limpiarTemporizadores();
        global.ATMState.datos.pantallaActual = nombrePantalla;

        this.establecerTitulo(pantalla.titulo || '');
        this.configurarBotonesLaterales({});

        // Reinicia la animación de entrada sin duplicar nodos
        contenedor.style.animation = 'none';
        void contenedor.offsetWidth;
        contenedor.style.animation = '';

        controladorPantallaActual = pantalla.render(contenedor, this, opciones) || {};
      }
    };

    global.ATMApp = app;
    app.irA('bienvenida');
  });
})(window);
