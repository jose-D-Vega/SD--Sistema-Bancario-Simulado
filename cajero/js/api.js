/**
 * Capa centralizada de datos ficticios y simulación local.
 * Todas las operaciones son simuladas en memoria; luego se conectarán
 * con los endpoints reales del backend en Go.
 */
(function (global) {
  'use strict';

  const CONFIG_DEMO = {
    PIN_INICIAL: '1234',
    PINES_TRIVIALES: ['0000', '1111', '1234', '2222', '3333', '4444', '5555', '6666', '7777', '8888', '9999'],
    EFECTIVO_CAJERO_GS: 15000000,
    MONTO_MINIMO_GS: 20000,
    MONTO_MAXIMO_GS: 1500000,
    MULTIPLO_BILLETE_GS: 10000,
    MONTOS_RAPIDOS: [100000, 200000, 300000, 500000],
    MONTOS_RECARGA: [10000, 20000, 30000, 50000],
    RECARGA_MIN_GS: 5000,
    RECARGA_MAX_GS: 200000,
    COMISION_ADELANTO_PCT: 0.05
  };

  // PIN vigente en memoria durante la sesión del navegador (no se persiste)
  let pinVigente = CONFIG_DEMO.PIN_INICIAL;

  const cajeroSimulado = {
    codigo: 'ATM-ASU-001',
    ubicacion: 'Sucursal Central · Asunción',
    efectivoDisponible: CONFIG_DEMO.EFECTIVO_CAJERO_GS
  };

  // Cuentas asociadas a la tarjeta de demostración
  const cuentasSimuladas = [
    {
      id: 'cta-ca-1',
      titular: 'Titular Demo Sudameris',
      numeroEnmascarado: '•••• •••• 4821',
      tipoCuenta: 'Caja de Ahorro',
      moneda: 'Guaraníes (PYG)',
      saldoDisponible: 2500000
    },
    {
      id: 'cta-cc-2',
      titular: 'Titular Demo Sudameris',
      numeroEnmascarado: '•••• •••• 7390',
      tipoCuenta: 'Cuenta Corriente',
      moneda: 'Guaraníes (PYG)',
      saldoDisponible: 1200000
    }
  ];

  // Movimientos iniciales por cuenta (máximo 5 visibles en extracto)
  const movimientosPorCuenta = {
    'cta-ca-1': [
      { fecha: '27/11/2026', descripcion: 'Transferencia recibida', tipo: 'Crédito', importe: 850000 },
      { fecha: '25/11/2026', descripcion: 'Compra con tarjeta', tipo: 'Débito', importe: 145000 },
      { fecha: '23/11/2026', descripcion: 'Extracción ATM', tipo: 'Débito', importe: 300000 },
      { fecha: '20/11/2026', descripcion: 'Pago de servicio ANDE', tipo: 'Débito', importe: 185000 },
      { fecha: '18/11/2026', descripcion: 'Depósito en ventanilla', tipo: 'Crédito', importe: 1200000 }
    ],
    'cta-cc-2': [
      { fecha: '26/11/2026', descripcion: 'Depósito acreditado', tipo: 'Crédito', importe: 600000 },
      { fecha: '24/11/2026', descripcion: 'Pago de servicio Internet', tipo: 'Débito', importe: 160000 },
      { fecha: '21/11/2026', descripcion: 'Compra con tarjeta', tipo: 'Débito', importe: 95000 },
      { fecha: '19/11/2026', descripcion: 'Extracción ATM', tipo: 'Débito', importe: 200000 }
    ]
  };

  // Tarjetas ficticias
  const tarjetaDebito = {
    banco: 'SUDAMERIS',
    tipo: 'Tarjeta de Débito',
    numeroEnmascarado: '•••• •••• •••• 4821',
    ultimos4: '4821',
    estado: 'Activa',
    vencimiento: '••/29',
    cuentaVinculada: '•••• •••• 4821',
    tipoCuentaAsociada: 'Caja de Ahorro'
  };

  const tarjetaCredito = {
    disponible: true,
    banco: 'SUDAMERIS',
    tipo: 'Tarjeta de Crédito Clásica',
    numeroEnmascarado: '•••• •••• •••• 9154',
    ultimos4: '9154',
    estado: 'Activa',
    vencimiento: '••/28',
    limiteCredito: 6000000,
    saldoUtilizado: 1450000,
    limiteAdelanto: 2000000
  };

  // Giros disponibles de prueba para "Giros Simple"
  const girosSimulados = {
    '741852': {
      codigo: '741852',
      codigoEnmascarado: '••••52',
      remitente: 'Carlos M. (Remitente Demo)',
      monto: 450000,
      moneda: 'Guaraníes (PYG)',
      estado: 'Disponible para retirar'
    },
    '852963': {
      codigo: '852963',
      codigoEnmascarado: '••••63',
      remitente: 'Laura R. (Remitente Demo)',
      monto: 800000,
      moneda: 'Guaraníes (PYG)',
      estado: 'Disponible para retirar'
    }
  };

  // Catálogo de servicios simulados
  const serviciosCatalogo = [
    { id: 'ande', nombre: 'ANDE', etiquetaCampo: 'Número de NIS', ejemploRef: '123456', empresa: 'ANDE · Energía Eléctrica (Simulación)', importeBase: 185000 },
    { id: 'agua', nombre: 'Agua', etiquetaCampo: 'Número de cuenta / ISSAN', ejemploRef: '234567', empresa: 'Servicio de Agua Potable (Simulación)', importeBase: 68000 },
    { id: 'telefonia', nombre: 'Telefonía', etiquetaCampo: 'Número de línea o factura', ejemploRef: '0981123456', empresa: 'Telefonía Móvil (Simulación)', importeBase: 110000 },
    { id: 'internet', nombre: 'Internet', etiquetaCampo: 'Número de contrato', ejemploRef: '345678', empresa: 'Fibra Hogar (Simulación)', importeBase: 165000 },
    { id: 'otros', nombre: 'Otros servicios', etiquetaCampo: 'Referencia de pago', ejemploRef: '456789', empresa: 'Servicio General (Simulación)', importeBase: 95000 }
  ];

  // Facturas predefinidas + resolución simulada para referencias válidas
  const facturasPredefinidas = {
    'ande:123456': { importe: 185000, vencimiento: '10/12/2026', estado: 'Pendiente de pago' },
    'agua:234567': { importe: 68000, vencimiento: '12/12/2026', estado: 'Pendiente de pago' },
    'telefonia:0981123456': { importe: 110000, vencimiento: '08/12/2026', estado: 'Pendiente de pago' },
    'internet:345678': { importe: 165000, vencimiento: '15/12/2026', estado: 'Pendiente de pago' },
    'otros:456789': { importe: 95000, vencimiento: '18/12/2026', estado: 'Pendiente de pago' }
  };

  const operadorasRecarga = ['Tigo', 'Personal', 'Claro'];

  const destinatariosDemo = [
    { numeroCuenta: '10020030', enmascarado: '•••• •••• 0030', nombre: 'Lucía B. (Cuenta Demo)' },
    { numeroCuenta: '20030040', enmascarado: '•••• •••• 0040', nombre: 'Martín G. (Cuenta Demo)' }
  ];

  function generarCodigoOperacion() {
    return 'OP-' + String(Math.floor(100000 + Math.random() * 900000));
  }

  function fechaActualCorta() {
    const ahora = new Date();
    return {
      fecha: ahora.toLocaleDateString('es-PY'),
      fechaHora: ahora.toLocaleString('es-PY', { dateStyle: 'short', timeStyle: 'short' })
    };
  }

  function registrarMovimiento(cuentaId, descripcion, tipo, importe) {
    const lista = movimientosPorCuenta[cuentaId] || (movimientosPorCuenta[cuentaId] = []);
    lista.unshift({
      fecha: fechaActualCorta().fecha,
      descripcion,
      tipo,
      importe
    });
  }

  function buscarCuenta(cuentaId) {
    if (!cuentaId) return cuentasSimuladas[0];
    return cuentasSimuladas.find((c) => c.id === cuentaId) || cuentasSimuladas[0];
  }

  function enmascararTexto(texto, visiblesFinal = 4) {
    const limpio = String(texto || '').trim();
    if (limpio.length <= visiblesFinal) return '••••';
    return '•••• ' + limpio.slice(-visiblesFinal);
  }

  const ATMData = {
    CONFIG_DEMO,

    obtenerCuentas() {
      return cuentasSimuladas.map((c) => ({ ...c }));
    },

    obtenerResumenCuenta(cuentaId) {
      const c = buscarCuenta(cuentaId);
      return { ...c };
    },

    obtenerMovimientos(cuentaId, limite = 5) {
      const cuenta = buscarCuenta(cuentaId);
      const lista = movimientosPorCuenta[cuenta.id] || [];
      return lista.slice(0, limite).map((m) => ({ ...m }));
    },

    validarPin(pin) {
      if (!pin || pin.length !== 4) {
        return { ok: false, codigo: 'INCOMPLETO', mensaje: 'El PIN debe tener exactamente 4 dígitos.' };
      }
      if (pin !== pinVigente) {
        return { ok: false, codigo: 'INCORRECTO', mensaje: 'PIN incorrecto. Intentá nuevamente.' };
      }
      return { ok: true, codigo: 'OK', mensaje: 'PIN verificado.' };
    },

    validarNuevoPin(pinNuevo) {
      if (!pinNuevo || pinNuevo.length !== 4 || !/^\d{4}$/.test(pinNuevo)) {
        return { ok: false, mensaje: 'El nuevo PIN debe tener exactamente 4 dígitos.' };
      }
      if (pinNuevo === pinVigente) {
        return { ok: false, mensaje: 'El nuevo PIN debe ser distinto al PIN actual.' };
      }
      if (CONFIG_DEMO.PINES_TRIVIALES.includes(pinNuevo)) {
        return { ok: false, mensaje: 'Elegí un PIN menos predecible (evitá 0000, 1111 o 1234).' };
      }
      return { ok: true };
    },

    cambiarPinSimulado(pinNuevo) {
      const check = this.validarNuevoPin(pinNuevo);
      if (!check.ok) return check;
      pinVigente = pinNuevo;
      return { ok: true, mensaje: 'PIN actualizado en memoria para esta sesión.' };
    },

    validarExtraccion(monto, cuentaId) {
      const cuenta = buscarCuenta(cuentaId);
      if (!Number.isFinite(monto) || monto <= 0) {
        return { ok: false, mensaje: 'Ingresá un monto válido mayor a Gs. 0.' };
      }
      if (monto < CONFIG_DEMO.MONTO_MINIMO_GS) {
        return { ok: false, mensaje: 'El monto mínimo de extracción es Gs. 20.000.' };
      }
      if (monto % CONFIG_DEMO.MULTIPLO_BILLETE_GS !== 0) {
        return { ok: false, mensaje: 'Ingresá un monto múltiplo de Gs. 10.000 (ej.: 50.000 o 100.000).' };
      }
      if (monto > CONFIG_DEMO.MONTO_MAXIMO_GS) {
        return { ok: false, mensaje: 'El límite máximo por extracción es Gs. 1.500.000.' };
      }
      if (monto > cuenta.saldoDisponible) {
        return { ok: false, mensaje: 'Saldo insuficiente en la cuenta seleccionada.' };
      }
      if (monto > cajeroSimulado.efectivoDisponible) {
        return { ok: false, mensaje: 'El cajero no dispone de efectivo suficiente para ese monto.' };
      }
      return { ok: true, mensaje: 'Monto válido.' };
    },

    ejecutarExtraccionSimulada(monto, tipoRecibo, cuentaId) {
      const cuenta = buscarCuenta(cuentaId);
      const validacion = this.validarExtraccion(monto, cuenta.id);
      if (!validacion.ok) return { ok: false, mensaje: validacion.mensaje };

      cuenta.saldoDisponible -= monto;
      cajeroSimulado.efectivoDisponible -= monto;
      registrarMovimiento(cuenta.id, 'Extracción ATM', 'Débito', monto);

      return {
        ok: true,
        operacion: {
          numeroOperacion: generarCodigoOperacion(),
          fechaHora: fechaActualCorta().fechaHora,
          cajero: cajeroSimulado.codigo,
          ubicacion: cajeroSimulado.ubicacion,
          cuenta: cuenta.numeroEnmascarado,
          tipoCuenta: cuenta.tipoCuenta,
          montoExtraido: monto,
          saldoRestante: cuenta.saldoDisponible,
          tipoRecibo
        }
      };
    },

    obtenerTarjetaDebito() {
      return { ...tarjetaDebito };
    },

    obtenerTarjetaCredito() {
      if (!tarjetaCredito.disponible) return { disponible: false };
      const disponibleCredito = Math.max(0, tarjetaCredito.limiteCredito - tarjetaCredito.saldoUtilizado);
      const disponibleAdelanto = Math.min(tarjetaCredito.limiteAdelanto, disponibleCredito);
      return {
        ...tarjetaCredito,
        disponibleCredito,
        disponibleAdelanto
      };
    },

    validarAdelanto(monto) {
      const tc = this.obtenerTarjetaCredito();
      if (!tc.disponible) {
        return { ok: false, mensaje: 'No tenés una tarjeta de crédito habilitada para adelantos.' };
      }
      if (!Number.isFinite(monto) || monto <= 0) {
        return { ok: false, mensaje: 'Ingresá un monto válido mayor a Gs. 0.' };
      }
      if (monto < CONFIG_DEMO.MONTO_MINIMO_GS) {
        return { ok: false, mensaje: 'El monto mínimo para adelanto es Gs. 20.000.' };
      }
      if (monto % CONFIG_DEMO.MULTIPLO_BILLETE_GS !== 0) {
        return { ok: false, mensaje: 'Ingresá un monto múltiplo de Gs. 10.000.' };
      }
      const comision = Math.round(monto * CONFIG_DEMO.COMISION_ADELANTO_PCT);
      const total = monto + comision;
      if (monto > tc.disponibleAdelanto || total > tc.disponibleCredito) {
        return { ok: false, mensaje: 'El monto supera tu límite disponible para adelanto.' };
      }
      if (monto > cajeroSimulado.efectivoDisponible) {
        return { ok: false, mensaje: 'El cajero no dispone de efectivo suficiente.' };
      }
      return { ok: true, comision, total };
    },

    ejecutarAdelantoSimulado(monto, tipoRecibo) {
      const validacion = this.validarAdelanto(monto);
      if (!validacion.ok) return validacion;

      tarjetaCredito.saldoUtilizado += validacion.total;
      cajeroSimulado.efectivoDisponible -= monto;

      const tcActualizada = this.obtenerTarjetaCredito();
      return {
        ok: true,
        operacion: {
          numeroOperacion: generarCodigoOperacion(),
          fechaHora: fechaActualCorta().fechaHora,
          cajero: cajeroSimulado.codigo,
          tarjeta: tarjetaCredito.numeroEnmascarado,
          montoSolicitado: monto,
          comisionSimulada: validacion.comision,
          totalCargado: validacion.total,
          disponibleRestante: tcActualizada.disponibleCredito,
          tipoRecibo
        }
      };
    },

    consultarGiro(codigo) {
      const limpio = String(codigo || '').trim();
      if (limpio.length < 4) {
        return { ok: false, mensaje: 'Ingresá un código de giro válido (ej. 741852).' };
      }
      const giro = girosSimulados[limpio];
      if (!giro) {
        return { ok: false, mensaje: 'No encontramos un giro con ese código. Probá con 741852 u 852963.' };
      }
      if (giro.estado !== 'Disponible para retirar') {
        return { ok: false, mensaje: 'Este giro ya fue retirado anteriormente.' };
      }
      return { ok: true, giro: { ...giro } };
    },

    retirarGiroSimulado(codigo, tipoRecibo) {
      const consulta = this.consultarGiro(codigo);
      if (!consulta.ok) return consulta;

      const giro = girosSimulados[codigo];
      if (giro.monto > cajeroSimulado.efectivoDisponible) {
        return { ok: false, mensaje: 'El cajero no dispone de efectivo suficiente para este giro.' };
      }

      giro.estado = 'Retirado';
      cajeroSimulado.efectivoDisponible -= giro.monto;

      return {
        ok: true,
        operacion: {
          numeroOperacion: generarCodigoOperacion(),
          fechaHora: fechaActualCorta().fechaHora,
          cajero: cajeroSimulado.codigo,
          codigoEnmascarado: giro.codigoEnmascarado,
          remitente: giro.remitente,
          monto: giro.monto,
          moneda: giro.moneda,
          tipoRecibo
        }
      };
    },

    obtenerServicios() {
      return serviciosCatalogo.map((s) => ({ ...s }));
    },

    consultarFacturaSimulada(servicioId, referencia) {
      const servicio = serviciosCatalogo.find((s) => s.id === servicioId);
      const refLimpia = String(referencia || '').trim();

      if (!servicio) {
        return { ok: false, mensaje: 'Servicio no reconocido.' };
      }
      if (!/^\d{5,12}$/.test(refLimpia) || /^0+$/.test(refLimpia)) {
        return {
          ok: false,
          mensaje: `Ingresá una referencia numérica válida de 5 a 12 dígitos (ej. ${servicio.ejemploRef}).`
        };
      }

      const clave = `${servicio.id}:${refLimpia}`;
      const predef = facturasPredefinidas[clave];
      const sumaDigitos = refLimpia.split('').reduce((acc, d) => acc + Number(d), 0);
      const importe = predef ? predef.importe : servicio.importeBase + (sumaDigitos % 5) * 15000;

      return {
        ok: true,
        factura: {
          servicioId: servicio.id,
          servicioNombre: servicio.nombre,
          empresa: servicio.empresa,
          referencia: refLimpia,
          referenciaEnmascarada: enmascararTexto(refLimpia, 4),
          importe,
          vencimiento: predef ? predef.vencimiento : '15/12/2026',
          estado: predef ? predef.estado : 'Pendiente de pago'
        }
      };
    },

    pagarServicioSimulado(factura, cuentaId, tipoRecibo) {
      const cuenta = buscarCuenta(cuentaId);
      if (factura.importe > cuenta.saldoDisponible) {
        return { ok: false, mensaje: 'Saldo insuficiente en la cuenta seleccionada para abonar esta factura.' };
      }

      cuenta.saldoDisponible -= factura.importe;
      registrarMovimiento(cuenta.id, `Pago de servicio ${factura.servicioNombre}`, 'Débito', factura.importe);

      return {
        ok: true,
        operacion: {
          numeroOperacion: generarCodigoOperacion(),
          fechaHora: fechaActualCorta().fechaHora,
          cajero: cajeroSimulado.codigo,
          servicio: factura.servicioNombre,
          empresa: factura.empresa,
          referenciaEnmascarada: factura.referenciaEnmascarada,
          cuenta: cuenta.numeroEnmascarado,
          monto: factura.importe,
          saldoRestante: cuenta.saldoDisponible,
          tipoRecibo
        }
      };
    },

    obtenerOperadoras() {
      return [...operadorasRecarga];
    },

    validarTelefonoParaguay(telefono) {
      const limpio = String(telefono || '').replace(/\D/g, '');
      if (!/^09\d{8}$/.test(limpio)) {
        return { ok: false, mensaje: 'Ingresá un número móvil válido de Paraguay (10 dígitos, ej. 0981123456).' };
      }
      return { ok: true, telefono: limpio };
    },

    enmascararTelefono(telefono) {
      const t = String(telefono || '');
      if (t.length < 7) return '••••';
      return `${t.slice(0, 4)} ••• ${t.slice(-3)}`;
    },

    validarRecarga(monto, cuentaId) {
      const cuenta = buscarCuenta(cuentaId);
      if (!Number.isFinite(monto) || monto <= 0) {
        return { ok: false, mensaje: 'Seleccioná o ingresá un monto válido.' };
      }
      if (monto < CONFIG_DEMO.RECARGA_MIN_GS || monto > CONFIG_DEMO.RECARGA_MAX_GS) {
        return { ok: false, mensaje: 'El monto de recarga debe estar entre Gs. 5.000 y Gs. 200.000.' };
      }
      if (monto % 1000 !== 0) {
        return { ok: false, mensaje: 'Ingresá un monto múltiplo de Gs. 1.000.' };
      }
      if (monto > cuenta.saldoDisponible) {
        return { ok: false, mensaje: 'Saldo insuficiente en la cuenta seleccionada.' };
      }
      return { ok: true };
    },

    ejecutarRecargaSimulada(operadora, telefono, monto, cuentaId, tipoRecibo) {
      const cuenta = buscarCuenta(cuentaId);
      const valTel = this.validarTelefonoParaguay(telefono);
      if (!valTel.ok) return valTel;
      const valMonto = this.validarRecarga(monto, cuenta.id);
      if (!valMonto.ok) return valMonto;

      cuenta.saldoDisponible -= monto;
      registrarMovimiento(cuenta.id, `Recarga ${operadora}`, 'Débito', monto);

      return {
        ok: true,
        operacion: {
          numeroOperacion: generarCodigoOperacion(),
          fechaHora: fechaActualCorta().fechaHora,
          cajero: cajeroSimulado.codigo,
          operadora,
          telefonoEnmascarado: this.enmascararTelefono(valTel.telefono),
          cuenta: cuenta.numeroEnmascarado,
          monto,
          saldoRestante: cuenta.saldoDisponible,
          tipoRecibo
        }
      };
    },

    obtenerDestinatariosDemo() {
      return destinatariosDemo.map((d) => ({ ...d }));
    },

    consultarDestinatarioSudameris(numeroCuenta) {
      const limpio = String(numeroCuenta || '').replace(/\D/g, '');
      if (!/^\d{6,12}$/.test(limpio)) {
        return { ok: false, mensaje: 'Ingresá un número de cuenta válido (6 a 12 dígitos).' };
      }
      const encontrado = destinatariosDemo.find((d) => d.numeroCuenta === limpio);
      return {
        ok: true,
        destinatario: encontrado || {
          numeroCuenta: limpio,
          enmascarado: enmascararTexto(limpio, 4),
          nombre: 'Destinatario Demo Sudameris'
        }
      };
    },

    validarTransferencia(origenId, monto) {
      const cuenta = buscarCuenta(origenId);
      if (!Number.isFinite(monto) || monto <= 0) {
        return { ok: false, mensaje: 'Ingresá un monto mayor a Gs. 0.' };
      }
      if (monto > cuenta.saldoDisponible) {
        return { ok: false, mensaje: 'Saldo insuficiente en la cuenta de origen.' };
      }
      return { ok: true };
    },

    ejecutarTransferenciaSimulada(datos) {
      const origen = buscarCuenta(datos.origenId);
      const val = this.validarTransferencia(origen.id, datos.monto);
      if (!val.ok) return val;

      origen.saldoDisponible -= datos.monto;
      registrarMovimiento(origen.id, `Transferencia a ${datos.destinoNombre}`, 'Débito', datos.monto);

      let destinoTexto = datos.destinoEnmascarado;
      if (datos.tipo === 'propia' && datos.destinoCuentaId) {
        const destino = buscarCuenta(datos.destinoCuentaId);
        if (destino.id === origen.id) {
          origen.saldoDisponible += datos.monto;
          return { ok: false, mensaje: 'La cuenta de destino debe ser distinta a la de origen.' };
        }
        destino.saldoDisponible += datos.monto;
        registrarMovimiento(destino.id, `Transferencia desde ${origen.tipoCuenta}`, 'Crédito', datos.monto);
        destinoTexto = `${destino.tipoCuenta} · ${destino.numeroEnmascarado}`;
      }

      return {
        ok: true,
        operacion: {
          numeroOperacion: generarCodigoOperacion(),
          fechaHora: fechaActualCorta().fechaHora,
          cajero: cajeroSimulado.codigo,
          origen: `${origen.tipoCuenta} · ${origen.numeroEnmascarado}`,
          destino: destinoTexto,
          beneficiario: datos.destinoNombre,
          referencia: datos.referencia || 'Transferencia ATM (Simulación)',
          monto: datos.monto,
          saldoRestante: origen.saldoDisponible,
          tipoRecibo: datos.tipoRecibo
        }
      };
    }
  };

  global.ATMData = ATMData;
})(window);
