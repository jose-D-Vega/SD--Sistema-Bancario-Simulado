/**
 * Estado temporal en memoria de la sesión actual del ATM.
 * No persiste credenciales ni PIN en archivos ni almacenamiento del navegador.
 */
(function (global) {
  'use strict';

  const estadoInicial = () => ({
    pantallaActual: 'bienvenida',
    tarjetaInsertada: false,
    autenticado: false,
    pinIngresado: '',
    intentosPinFallidos: 0,
    cuentaSeleccionadaId: null,
    extraccion: {
      montoInput: '',
      montoConfirmado: 0,
      tipoRecibo: null,
      ultimaOperacion: null
    },
    operacionTemporal: {},
    temporizadores: [],
    intervalos: []
  });

  const ATMState = {
    datos: estadoInicial(),

    registrarTimeout(fn, ms) {
      const id = global.setTimeout(fn, ms);
      this.datos.temporizadores.push(id);
      return id;
    },

    registrarInterval(fn, ms) {
      const id = global.setInterval(fn, ms);
      this.datos.intervalos.push(id);
      return id;
    },

    limpiarTemporizadores() {
      this.datos.temporizadores.forEach((id) => global.clearTimeout(id));
      this.datos.intervalos.forEach((id) => global.clearInterval(id));
      this.datos.temporizadores = [];
      this.datos.intervalos = [];
    },

    limpiarPin() {
      this.datos.pinIngresado = '';
    },

    limpiarExtraccion() {
      this.datos.extraccion = {
        montoInput: '',
        montoConfirmado: 0,
        tipoRecibo: null,
        ultimaOperacion: null
      };
    },

    limpiarOperacionTemporal() {
      this.datos.operacionTemporal = {};
    },

    seleccionarCuenta(cuentaId) {
      this.datos.cuentaSeleccionadaId = cuentaId || null;
    },

    reiniciarSesion() {
      this.limpiarTemporizadores();
      this.datos = estadoInicial();
    }
  };

  global.ATMState = ATMState;
})(window);
