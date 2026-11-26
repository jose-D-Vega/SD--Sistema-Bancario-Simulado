// Modelo bancario local del prototipo SUDAMERIS.
// Cuando se conecte la API real, este archivo se reemplaza por llamadas HTTP.
(() => {
  const PASSWORD_HASH =
    "d684a7f74b1d440c7e2f4df18edad8e6e8957e395c915ba4f193b98112787b4d"; // sha256("Esmeralda123!")
  const BANCO = "SUDAMERIS";
  const MAX_CUENTAS = 5;

  const TIPO = { CORRIENTE: "CUENTA_CORRIENTE", AHORRO: "CAJA_AHORRO" };
  const TIPO_LABEL = {
    CUENTA_CORRIENTE: "Cuenta corriente",
    CAJA_AHORRO: "Caja de ahorro",
  };
  const ESTADO_LABEL = {
    ACTIVA: "Activa",
    BLOQUEADA: "Bloqueada",
    CERRADA: "Cerrada",
  };
  const USERNAME_RE = /^[a-z0-9._-]{3,30}$/;
  const DOCUMENTO_RE = /^[0-9]{5,12}$/;

  // El alias viaja entre bancos, así que se guarda normalizado: minúsculas y
  // sólo letras, números y "_". Un teléfono "+595 976 312 716" queda como
  // "595976312716" (el backend hoy sólo acepta [a-z0-9._-]).
  const normalizarAlias = (valor) =>
    String(valor ?? "")
      .toLowerCase()
      .replace(/[^a-z0-9_]/g, "");

  const formatearAlias = (alias) => {
    const valor = normalizarAlias(alias);
    if (!valor) return "";
    if (/^595\d{9}$/.test(valor))
      return (
        "+" +
        valor.slice(0, 3) +
        " " +
        valor.slice(3, 6) +
        " " +
        valor.slice(6, 9) +
        " " +
        valor.slice(9)
      );
    return valor;
  };

  const hacerCuenta = (type, number, balance, status = "ACTIVA", alias = null) => ({
    type,
    number,
    balance,
    status,
    alias: alias ? normalizarAlias(alias) : null,
    spent: 0,
    spentDate: "",
  });

  const hacerUsuario = (username, name, document, phone, accounts) => ({
    id: username,
    username,
    passwordHash: PASSWORD_HASH,
    profile: { name, document, email: username + "@example.com", phone },
    accounts,
    transactions: [],
    settings: { notifications: true },
    cardFrozen: false,
  });

  function seed() {
    const users = [
      hacerUsuario("maria", "María Teresa Bogado", "4567890", "+595 981 111 111", [
        hacerCuenta(TIPO.CORRIENTE, "025748021", 2500371, "ACTIVA", "4567890"),
        hacerCuenta(TIPO.AHORRO, "025748022", 350000, "ACTIVA", "595981111111"),
      ]),
      hacerUsuario("lucia", "Lucía Bogado", "4567891", "+595 981 222 222", [
        // Su cédula ya está tomada como alias por el Banco 2.
        hacerCuenta(TIPO.AHORRO, "025748031", 750000, "ACTIVA", "595981222222"),
        hacerCuenta(TIPO.CORRIENTE, "025748032", 180000),
      ]),
      hacerUsuario("martin", "Martín Benítez", "4567892", "", [
        // Sin celular cargado: sólo puede usar la cédula como alias.
        hacerCuenta(TIPO.CORRIENTE, "025748041", 0),
      ]),
      hacerUsuario("ana", "Ana López", "4567893", "+595 981 444 444", [
        hacerCuenta(TIPO.AHORRO, "025748051", 400000, "BLOQUEADA", "4567893"),
        hacerCuenta(TIPO.CORRIENTE, "025748052", 0, "CERRADA"),
      ]),
    ];
    users[0].transactions = [
      { id: "ESM-284103", title: "Lucía Bogado", amount: 150000, category: "Transferencia recibida", date: "Hoy, 09:41", status: "Completada", account: 0 },
      { id: "ESM-284102", title: "Cantina Tecnología UNCA", amount: -5500, category: "Pago QR", kind: "qr", date: "Hoy, 08:20", status: "Completada", account: 0 },
      { id: "ESM-284101", title: "Martín Benítez", amount: -35000, category: "Almuerzo compartido", date: "Ayer, 13:12", status: "Completada", account: 0 },
      { id: "ESM-284100", title: "Supermercado Los Jardines", amount: -84200, category: "Pago QR", kind: "qr", date: "Ayer, 18:36", status: "Completada", account: 0 },
      { id: "ESM-284099", title: "Ahorro del mes", amount: 350000, category: "Transferencia interna", date: "27 oct, 16:00", status: "Completada", account: 1 },
    ];
    return {
      version: 3,
      users,
      // Cuentas de los otros bancos del sistema: sirven para probar que un
      // alias es único ENTRE bancos, no sólo dentro de SUDAMERIS.
      external: [
        { bank: "Banco 2", number: "2001001", alias: "4567891", name: "Carlos Acosta", status: "ACTIVA", balance: 1000000 },
        { bank: "Banco 3", number: "3001001", alias: "595971000111", name: "Sofía Duarte", status: "ACTIVA", balance: 500000, pending: true },
        { bank: "Banco 4", number: "4001001", alias: "4123456", name: "Diego Rojas", status: "BLOQUEADA", balance: 250000 },
        { bank: BANCO, number: "025749001", alias: "80012345", name: "Cantina Tecnología UNCA", status: "ACTIVA", balance: 0 },
      ],
      operations: {},
    };
  }

  // Compatibilidad: convierte datos guardados de la versión anterior (v2).
  function migrate(saved) {
    if (!saved || !Array.isArray(saved.users)) return seed();
    for (const user of saved.users) {
      const id = String(user.id || "").toLowerCase();
      if (!user.username) user.username = USERNAME_RE.test(id) ? id : "usuario" + id.slice(0, 6);
      const legado = user.profile ? user.profile.alias : null;
      if (user.profile) delete user.profile.alias;
      if (!user.profile.phone) user.profile.phone = "";
      for (const cuenta of user.accounts || []) {
        cuenta.status = String(cuenta.status || "ACTIVA").toUpperCase();
        if (cuenta.type === "Cuenta corriente") cuenta.type = TIPO.CORRIENTE;
        if (cuenta.type === "Caja de ahorro") cuenta.type = TIPO.AHORRO;
        cuenta.alias = cuenta.alias ? normalizarAlias(cuenta.alias) : null;
      }
      if (legado && user.accounts && user.accounts.length && !user.accounts.some((c) => c.alias))
        user.accounts[0].alias = normalizarAlias(legado) || null;
    }
    for (const item of saved.external || [])
      item.status = String(item.status || "ACTIVA").toUpperCase();
    saved.version = 3;
    return saved;
  }

  // Índice global de alias: SUDAMERIS + los otros bancos del sistema.
  function aliasMap(db) {
    const mapa = new Map();
    for (const user of db.users)
      for (const cuenta of user.accounts) {
        const alias = normalizarAlias(cuenta.alias);
        if (alias)
          mapa.set(alias, { banco: BANCO, nombre: user.profile.name, numero: cuenta.number, usuarioId: user.id });
      }
    for (const item of db.external) {
      const alias = normalizarAlias(item.alias);
      if (alias && !mapa.has(alias))
        mapa.set(alias, { banco: item.bank, nombre: item.name, numero: item.number, usuarioId: null });
    }
    return mapa;
  }

  // Datos del titular que puede usar como alias: cédula y celular.
  // (El backend no puede verificarlos; es una regla de producto del cliente.)
  function aliasOptions(db, user, index) {
    const cuenta = user.accounts[index];
    const usados = aliasMap(db);
    const opciones = [];
    const agregar = (kind, label, valor) => {
      const value = normalizarAlias(valor);
      if (!value) return;
      const dueno = usados.get(value);
      const current = value === normalizarAlias(cuenta.alias);
      opciones.push({
        kind,
        label,
        value,
        display: formatearAlias(value),
        current,
        taken: !!dueno && !current,
        by: dueno ? dueno.banco + " · " + dueno.nombre : "",
      });
    };
    agregar("ci", "Cédula de identidad", user.profile.document);
    agregar("telefono", "N° de teléfono", user.profile.phone);
    return opciones;
  }

  function setAlias(db, user, index, valor) {
    const value = normalizarAlias(valor);
    if (!value) throw Error("Elegí uno de tus datos para usar como alias.");
    const opcion = aliasOptions(db, user, index).find((o) => o.value === value);
    if (!opcion)
      throw Error("El alias tiene que ser tu cédula o tu celular.");
    if (opcion.taken)
      throw Error("Ese alias ya está en uso en " + opcion.by + ". Elegí otro.");
    user.accounts[index].alias = value;
    return formatearAlias(value);
  }

  function usernameDisponible(db, username, exceptId) {
    const value = String(username || "").toLowerCase();
    return !db.users.some((u) => u.username === value && u.id !== exceptId);
  }

  function siguienteNumero(db) {
    const usados = new Set();
    for (const user of db.users) for (const c of user.accounts) usados.add(c.number);
    for (const item of db.external) usados.add(item.number);
    let n = 25748061;
    while (usados.has(String(n).padStart(9, "0"))) n += 1;
    return String(n).padStart(9, "0");
  }

  function openAccount(db, user, tipo) {
    if (!TIPO_LABEL[tipo]) throw Error("Elegí un tipo de cuenta válido.");
    if (user.accounts.length >= MAX_CUENTAS)
      throw Error("Llegaste al máximo de " + MAX_CUENTAS + " cuentas por persona.");
    const cuenta = hacerCuenta(tipo, siguienteNumero(db), 0);
    user.accounts.push(cuenta);
    return { account: cuenta, index: user.accounts.length - 1 };
  }

  function accountAlias(cuenta) {
    return cuenta && cuenta.alias ? formatearAlias(cuenta.alias) : "";
  }

  function findDestination(db, bank, identifier) {
    const key = normalizarAlias(identifier);
    if (!key) return null;
    if (bank === BANCO) {
      for (const user of db.users) {
        const i = user.accounts.findIndex(
          (a) => normalizarAlias(a.number) === key || normalizarAlias(a.alias) === key,
        );
        if (i >= 0)
          return { user, index: i, account: user.accounts[i], name: user.profile.name };
      }
    }
    const item = db.external.find(
      (a) =>
        a.bank === bank &&
        (normalizarAlias(a.number) === key || normalizarAlias(a.alias) === key),
    );
    return item ? { account: item, name: item.name } : null;
  }

  function validateDestination(db, user, accountIndex, bank, identifier) {
    const origin = user.accounts[accountIndex];
    if (origin.status !== "ACTIVA")
      throw Error(
        "Tu cuenta está " +
          ESTADO_LABEL[origin.status].toLowerCase() +
          ". Elegí una cuenta activa.",
      );
    const texto = String(identifier || "").trim();
    if (texto.startsWith("@"))
      throw Error(
        "El usuario (@) sirve para identificarte dentro de la app, no para transferir. Usá el alias o el número de cuenta.",
      );
    const dest = findDestination(db, bank, texto);
    if (!dest)
      throw Error("No encontramos esa cuenta o alias en el banco seleccionado.");
    if (dest.account === origin)
      throw Error("Elegí una cuenta de destino diferente de la cuenta de origen.");
    if (dest.account.status !== "ACTIVA")
      throw Error(
        "La cuenta de destino está " +
          ESTADO_LABEL[dest.account.status].toLowerCase() +
          ".",
      );
    return dest;
  }

  const displayId = (key) =>
    "ESM-" +
    String(key)
      .replace(/[^a-f0-9]/gi, "")
      .slice(0, 10)
      .toUpperCase();

  // `key` es la clave de idempotencia (UUID puro, como pide el backend).
  function execute(db, user, index, transfer, kind, key, now = new Date()) {
    if (db.operations[key]) return db.operations[key];
    const origin = user.accounts[index];
    const dest = validateDestination(db, user, index, transfer.bank, transfer.destination);
    const amount = Number(transfer.amount);
    if (!Number.isSafeInteger(amount) || amount < 1 || amount > 10000000)
      throw Error("Ingresá un importe entero entre 1 y 10.000.000 Gs.");
    const day = new Intl.DateTimeFormat("en-CA", {
      timeZone: "America/Asuncion",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).format(now);
    const spent = origin.spentDate === day ? origin.spent : 0;
    if (spent + amount > 10000000)
      throw Error("Este envío supera el límite diario de 10.000.000 Gs.");
    if (amount > origin.balance)
      throw Error("Tu saldo disponible no alcanza para este envío.");

    const pending = !!dest.account.pending;
    const date = now.toLocaleString("es-PY", {
      timeZone: "America/Asuncion",
      day: "2-digit",
      month: "short",
      hour: "2-digit",
      minute: "2-digit",
    });
    const id = displayId(key);
    const transaction = {
      id,
      title: dest.name,
      amount: -amount,
      category: transfer.concept || "Transferencia enviada",
      date,
      status: pending ? "Pendiente" : "Completada",
      account: index,
      kind,
    };
    origin.balance -= amount;
    origin.spent = spent + amount;
    origin.spentDate = day;
    user.transactions.unshift(transaction);
    if (!pending) {
      dest.account.balance += amount;
      if (dest.user)
        dest.user.transactions.unshift({
          ...transaction,
          title: user.profile.name,
          amount,
          category: transfer.concept || "Transferencia recibida",
          account: dest.index,
        });
    }
    db.operations[key] = transaction;
    return transaction;
  }

  globalThis.EsmeraldaBank = {
    BANCO,
    TIPO,
    TIPO_LABEL,
    ESTADO_LABEL,
    USERNAME_RE,
    DOCUMENTO_RE,
    MAX_CUENTAS,
    normalizarAlias,
    formatearAlias,
    seed,
    migrate,
    aliasMap,
    aliasOptions,
    setAlias,
    usernameDisponible,
    openAccount,
    accountAlias,
    findDestination,
    validateDestination,
    execute,
  };
})();
