const { createApp, computed, ref } = Vue;
const paths = {
  plus: "M12 4v16M4 12h16",
  menu: "M4 5h16M4 12h16M4 19h16",
  home: "M3 10 12 3l9 7v10a1 1 0 0 1-1 1h-5v-7H9v7H4a1 1 0 0 1-1-1Z",
  send: "M5 19 19 5M5 5h14v14",
  receive: "M19 5 5 19M5 5v14h14",
  grid: "M4 4h6v6H4ZM14 4h6v6h-6ZM4 14h6v6H4ZM14 14h6v6h-6Z",
  user: "M20 21v-2a6 6 0 0 0-6-6h-4a6 6 0 0 0-6 6v2M16 6a4 4 0 1 1-8 0 4 4 0 0 1 8 0",
  bell: "M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4",
  eye: "M2 12s4-7 10-7 10 7 10 7-4 7-10 7S2 12 2 12Zm13 0a3 3 0 1 1-6 0 3 3 0 0 1 6 0",
  qr: "M3 3h7v7H3ZM14 3h7v7h-7ZM3 14h7v7H3ZM14 14h3v3h4v4h-7ZM21 14v3M17 21v-3",
  history: "M3 5v5h5M3 10a9 9 0 1 1 1 8M12 7v5l3 2",
  wallet: "M3 6h17v14H3ZM3 6V4h15v2M15 11h6v5h-6Z",
  shield: "M12 2 3 6v6c0 5 9 10 9 10s9-5 9-10V6ZM8 12l3 3 5-6",
  copy: "M8 8h13v13H8ZM16 8V3H3v13h5",
  search: "M16 16l5 5M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0",
  help: "M9 8a3 3 0 1 1 5 2c-2 1-2 2-2 3M12 17v.1M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0",
};
const app = createApp({
  setup() {
    const Bank = EsmeraldaBank;
    if (new URLSearchParams(location.search).get("restablecer") === "1") {
      localStorage.removeItem("esmeralda-banca-v2");
      history.replaceState(null, "", location.pathname);
    }
    let saved;
    try {
      saved = JSON.parse(localStorage.getItem("esmeralda-banca-v2"));
    } catch {}
    const db = Vue.reactive(Bank.migrate(saved));
    for (const item of db.external)
      if (item.bank === "Esmeralda") item.bank = "SUDAMERIS";
    const currentUser = ref(db.users[0]);
    const profile = computed(() => currentUser.value.profile),
      username = computed(() => currentUser.value.username),
      accountAlias = computed(() => Bank.accountAlias(account.value)),
      aliasRaw = computed(() => (account.value && account.value.alias) || ""),
      accounts = computed(() => currentUser.value.accounts),
      transactions = computed(() => currentUser.value.transactions),
      settings = computed(() => currentUser.value.settings),
      cardFrozen = computed({
        get: () => currentUser.value.cardFrozen,
        set: (v) => (currentUser.value.cardFrozen = v),
      });
    const paymentCode = ref(""),
      paymentError = ref(""),
      support = ref({ subject: "Transferencias", message: "" }),
      supportError = ref("");
    const saveSupport = () => {
      if (!support.value.message.trim()) {
        supportError.value = "Escribí tu consulta.";
        return;
      }
      supportError.value = "";
      if (!db.support) db.support = [];
      db.support.push({
        id: "AY-" + Date.now(),
        user: currentUser.value.id,
        ...support.value,
      });
      persist();
      supportSent.value = true;
    };
    const digest = async (text) =>
      Array.from(
        new Uint8Array(
          await crypto.subtle.digest("SHA-256", new TextEncoder().encode(text)),
        ),
      )
        .map((x) => x.toString(16).padStart(2, "0"))
        .join("");
    const DOCUMENT_PATTERN = /^[0-9]{5,12}$/,
      EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    const digitsOf = (v) => String(v || "").replace(/\D/g, "");
    const passwordHasWhitespace = (value) => /\s/.test(value);
    const logged = ref(false),
      page = ref("inicio"),
      auth = ref("login"),
      credentials = ref({ user: "", password: "" }),
      registration = ref({
        name: "",
        username: "",
        confirm: "",
        document: "",
        type: Bank.TIPO.AHORRO,
        accepted: false,
      }),
      usernameEdited = ref(false),
      authError = ref(""),
      showPassword = ref(false),
      activeAccount = ref(0),
      balanceVisible = ref(true),
      homeTab = ref("cuentas"),
      toast = ref(""),
      modal = ref(""),
      search = ref(""),
      filter = ref("Todos"),
      transferStep = ref(1),
      transfer = ref({}),
      transferError = ref(""),
      busy = ref(false),
      paymentKind = ref("transfer"),
      receipt = ref(null),
      unread = ref(true),
      supportSent = ref(false),
      passwords = ref({ current: "", next: "", confirm: "" }),
      passwordErrors = ref({}),
      profileErrors = ref({}),
      fieldErrors = ref({}),
      aliasChoice = ref(""),
      aliasTarget = ref(-1),
      aliasError = ref(""),
      nuevaCuenta = ref({ type: Bank.TIPO.AHORRO }),
      nuevaCuentaError = ref("");
    let toastTimer;
    const account = computed(() => accounts.value[activeAccount.value]),
      initials = computed(() =>
        profile.value.name
          .split(" ")
          .filter(Boolean)
          .slice(0, 2)
          .map((x) => x[0])
          .join(""),
      ),
      firstName = computed(() => profile.value.name.split(" ")[0]);
    const money = (n) =>
      new Intl.NumberFormat("es-PY", { maximumFractionDigits: 0 }).format(n);
    // El backend guarda los códigos (CUENTA_CORRIENTE, ACTIVA); la app los
    // muestra lindos sin perder el dato real.
    const tipoLabel = (cuenta) => Bank.TIPO_LABEL[cuenta.type] || cuenta.type;
    const estadoLabel = (estado) => Bank.ESTADO_LABEL[estado] || estado;
    const formatearAlias = (valor) => Bank.formatearAlias(valor);
    const tiposCuenta = Object.entries(Bank.TIPO_LABEL).map(([value, label]) => ({
      value,
      label,
    }));
    const persist = () => {
      try {
        localStorage.setItem("esmeralda-banca-v2", JSON.stringify(db));
      } catch {
        notify("No se pudo guardar en este navegador");
      }
    };
    const notify = (t) => {
      toast.value = t;
      clearTimeout(toastTimer);
      toastTimer = setTimeout(() => (toast.value = ""), 3500);
    };
    const go = (p) => {
      if (p === "perfil") {
        profileErrors.value = {};
      }
      if (p === "alias") {
        aliasError.value = "";
        aliasTarget.value = -1;
      }
      page.value = p;
      window.scrollTo({ top: 0, behavior: "instant" });
    };
    const resetSession = () => {
      activeAccount.value = 0;
      search.value = "";
      filter.value = "Todos";
      homeTab.value = "cuentas";
      receipt.value = null;
      unread.value = false;
      notices.value = [];
      transferStep.value = 1;
      transferError.value = "";
      supportSent.value = false;
      passwords.value = { current: "", next: "", confirm: "" };
      passwordErrors.value = {};
      profileErrors.value = {};
      fieldErrors.value = {};
      aliasChoice.value = "";
      aliasTarget.value = -1;
      aliasError.value = "";
      nuevaCuentaError.value = "";
      supportError.value = "";
      paymentCode.value = "";
      paymentError.value = "";
      balanceVisible.value = true;
    };
    const logout = () => {
      logged.value = false;
      modal.value = "";
      auth.value = "login";
      credentials.value = { user: "", password: "" };
      resetSession();
    };
    const setAuth = (mode) => {
      auth.value = mode;
      authError.value = "";
      fieldErrors.value = {};
      showPassword.value = false;
      credentials.value.user = "";
      registration.value.confirm = "";
      registration.value.accepted = false;
      usernameEdited.value = false;
    };
    // Sugiere un @ a partir del nombre mientras el usuario no lo edite a mano.
    const sugerirUsuario = () => {
      if (usernameEdited.value) return;
      registration.value.username = registration.value.name
        .toLowerCase()
        .normalize("NFD")
        .replace(/[\u0300-\u036f]/g, "")
        .replace(/[^a-z0-9]+/g, ".")
        .replace(/^\.+|\.+$/g, "")
        .slice(0, 30);
    };
    const authenticate = async () => {
      authError.value = "";
      if (busy.value) return;
      const password = credentials.value.password,
        errors = {};
      if (auth.value === "recover") {
        const email = credentials.value.user.trim().toLowerCase();
        if (!EMAIL_PATTERN.test(email))
          errors.user = "Ingresá un correo válido.";
        fieldErrors.value = errors;
        if (!errors.user)
          authError.value =
            "Para recuperar tu acceso, contactá a atención al cliente.";
        return;
      }
      // El "usuario" es el @ de la app. También aceptamos la cédula para
      // entrar más rápido (el backend hoy sólo busca por username).
      const registering = auth.value === "register",
        reg = registration.value,
        id = (registering ? reg.username : credentials.value.user)
          .trim()
          .toLowerCase()
          .replace(/^@+/, "");
      if (registering) {
        if (!id) errors.username = "Elegí tu usuario (@).";
        else if (!Bank.USERNAME_RE.test(id))
          errors.username =
            "Usá de 3 a 30 caracteres: minúsculas, números, punto, guion o guion bajo.";
        else if (!Bank.usernameDisponible(db, id))
          errors.username = "Ese usuario (@) ya está en uso.";
      } else if (!id) errors.user = "Ingresá tu usuario (@) o tu cédula.";
      if (!password) errors.password = "Ingresá tu contraseña.";
      else if (passwordHasWhitespace(password))
        errors.password = "La contraseña no puede contener espacios.";
      if (registering) {
        const documento = (reg.document || "").trim();
        if (!reg.name.trim()) errors.name = "Ingresá tu nombre y apellido.";
        if (!documento) errors.document = "Ingresá tu número de documento.";
        else if (!DOCUMENT_PATTERN.test(documento))
          errors.document = "Ingresá un documento de 5 a 12 dígitos.";
        else if (db.users.some((u) => u.profile.document === documento))
          errors.document = "Ya existe una cuenta con ese documento.";
        if (!reg.confirm) errors.confirm = "Repetí tu contraseña.";
        else if (!errors.password && password !== reg.confirm)
          errors.confirm = "Las contraseñas no coinciden.";
        if (!reg.accepted)
          errors.terms = "Tenés que aceptar las condiciones de uso.";
      }
      fieldErrors.value = errors;
      if (Object.keys(errors).length) return;
      busy.value = true;
      try {
        if (registering) {
          const user = {
            id: crypto.randomUUID(),
            username: id,
            passwordHash: await digest(password),
            profile: {
              name: reg.name.trim(),
              document: (reg.document || "").trim(),
              email: "",
              phone: "",
            },
            accounts: [],
            transactions: [],
            settings: { notifications: true },
            cardFrozen: false,
          };
          Bank.openAccount(db, user, reg.type);
          db.users.push(user);
          currentUser.value = user;
          persist();
        } else {
          const user = db.users.find(
            (u) => u.username === id || u.profile.document === id,
          );
          if (!user || user.passwordHash !== (await digest(password)))
            throw Error("Usuario o contraseña incorrectos.");
          currentUser.value = user;
        }
        credentials.value = { user: "", password: "" };
        registration.value = {
          name: "",
          username: "",
          confirm: "",
          document: "",
          type: Bank.TIPO.AHORRO,
          accepted: false,
        };
        usernameEdited.value = false;
        resetSession();
        logged.value = true;
        go(registering ? "alias" : "inicio");
        if (registering) {
          notify("Cuenta creada. Elegí tu alias.");
          abrirAlias(0);
        }
      } catch (e) {
        authError.value = e.message;
      } finally {
        busy.value = false;
      }
    };
    const startTransfer = () => {
      transfer.value = {
        bank: "SUDAMERIS",
        destination: "",
        name: "",
        amount: "",
        concept: "",
      };
      transferStep.value = 1;
      transferError.value = "";
      paymentKind.value = "transfer";
      go("enviar");
    };
    const advanceTransfer = () => {
      transferError.value = "";
      if (busy.value) return;
      try {
        const dest = Bank.validateDestination(
          db,
          currentUser.value,
          activeAccount.value,
          transfer.value.bank,
          transfer.value.destination,
        );
        transfer.value.name = dest.name;
        if (transferStep.value === 1) {
          transferStep.value = 2;
          return;
        }
        const amount = Number(transfer.value.amount);
        if (!Number.isSafeInteger(amount) || amount <= 0 || amount > 10000000)
          throw Error("Ingresá un importe entero entre 1 y 10.000.000 Gs.");
        if (amount > account.value.balance)
          throw Error("Tu saldo disponible no alcanza para este envío.");
        if (transferStep.value === 2) {
          transferStep.value = 3;
          return;
        }
        busy.value = true;
        const t = Bank.execute(
          db,
          currentUser.value,
          activeAccount.value,
          transfer.value,
          paymentKind.value,
          crypto.randomUUID(),
        );
        persist();
        if (settings.value.notifications) {
          notices.value.unshift({
            title:
              t.status === "Pendiente"
                ? "Envío pendiente de confirmación"
                : "Tu envío se completó",
            text: "Envío de " + money(amount) + " Gs. a " + t.title,
            date: "Ahora",
          });
          unread.value = true;
        }
        openReceipt(t);
      } catch (e) {
        transferError.value = e.message;
      } finally {
        busy.value = false;
      }
    };
    const readPaymentCode = () => {
      paymentError.value = "";
      if (paymentCode.value.trim().toUpperCase() !== "ESM-CANTINA-5500") {
        paymentError.value = "No encontramos ese código de pago.";
        return;
      }
      transfer.value = {
        bank: "SUDAMERIS",
        destination: "80012345",
        name: "Cantina Tecnología UNCA",
        amount: 5500,
        concept: "Pago QR",
      };
      transferStep.value = 2;
      transferError.value = "";
      paymentKind.value = "qr";
      go("enviar");
    };
    const openReceipt = (t) => {
      receipt.value = t;
      go("comprobante");
    };
    const filteredTransactions = computed(() =>
      transactions.value.filter(
        (t) =>
          t.account === activeAccount.value &&
          (filter.value === "Todos" ||
            (filter.value === "Ingresos" ? t.amount > 0 : t.amount < 0)) &&
          (t.title + " " + t.category + " " + t.id)
            .toLowerCase()
            .includes(search.value.toLowerCase()),
      ),
    );
    const copy = async (t) => {
      try {
        await navigator.clipboard.writeText(t);
        notify("Copiado al portapapeles");
      } catch {
        notify("Seleccioná el texto para copiarlo: " + t);
      }
    };
    const shareAccount = async () => {
      const text =
        profile.value.name +
        " · Banco SUDAMERIS · Cuenta " +
        account.value.number +
        (accountAlias.value ? " · Alias " + accountAlias.value : "");
      if (navigator.share) {
        try {
          await navigator.share({ title: "Mis datos bancarios", text });
        } catch (e) {
          if (e.name !== "AbortError") copy(text);
        }
      } else copy(text);
    };
    const download = (name, text, type = "text/plain;charset=utf-8") => {
      const url = URL.createObjectURL(new Blob([text], { type }));
      const a = document.createElement("a");
      a.href = url;
      a.download = name;
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    };
    const downloadReceipt = () => {
      const t = receipt.value;
      download(
        t.id + ".txt",
        "SUDAMERIS · COMPROBANTE\n\nOperación: " +
          t.id +
          "\nDestino/origen: " +
          t.title +
          "\nImporte: " +
          money(Math.abs(t.amount)) +
          " Gs.\nConcepto: " +
          t.category +
          "\nFecha: " +
          t.date +
          "\nEstado: " +
          t.status +
          "\n\nSUDAMERIS · Banca móvil",
      );
      notify("Comprobante descargado");
    };
    const exportHistory = () => {
      const esc = (x) => '"' + String(x).replaceAll('"', '""') + '"';
      download(
        "movimientos-sudameris.csv",
        "\uFEFF" +
          [
            ["Operación", "Descripción", "Importe PYG", "Fecha", "Estado"],
            ...filteredTransactions.value.map((t) => [
              t.id,
              t.title,
              t.amount,
              t.date,
              t.status,
            ]),
          ]
            .map((r) => r.map(esc).join(";"))
            .join("\r\n"),
        "text/csv;charset=utf-8",
      );
      notify("Movimientos exportados");
    };
    // -----------------------------------------------------------------
    // Alias: es de la CUENTA (no del perfil) y es único en todo el sistema.
    // El usuario (@) es otra cosa: sirve para identificarte en la app.
    // -----------------------------------------------------------------
    const aliasOptions = computed(() =>
      aliasTarget.value >= 0
        ? Bank.aliasOptions(db, currentUser.value, aliasTarget.value)
        : [],
    );
    const abrirAlias = (index) => {
      aliasTarget.value = index;
      aliasChoice.value = currentUser.value.accounts[index].alias || "";
      aliasError.value = "";
      modal.value = "alias";
    };
    const cerrarAlias = () => {
      modal.value = "";
      aliasError.value = "";
    };
    const guardarAlias = () => {
      try {
        const valor = Bank.setAlias(
          db,
          currentUser.value,
          aliasTarget.value,
          aliasChoice.value,
        );
        persist();
        modal.value = "";
        notify("Alias actualizado: " + valor);
      } catch (e) {
        aliasError.value = e.message;
      }
    };
    // -----------------------------------------------------------------
    // Abrir otra cuenta sin volver a registrarse.
    // -----------------------------------------------------------------
    const abrirNuevaCuenta = () => {
      nuevaCuenta.value = { type: Bank.TIPO.AHORRO };
      nuevaCuentaError.value = "";
      modal.value = "nueva-cuenta";
    };
    const crearCuenta = () => {
      try {
        const { index } = Bank.openAccount(
          db,
          currentUser.value,
          nuevaCuenta.value.type,
        );
        persist();
        activeAccount.value = index;
        modal.value = "";
        notify("Cuenta abierta. Elegí su alias.");
        abrirAlias(index);
      } catch (e) {
        nuevaCuentaError.value = e.message;
      }
    };
    const saveProfile = () => {
      const p = profile.value,
        errors = {};
      p.name = p.name.trim();
      p.email = (p.email || "").trim().toLowerCase();
      p.phone = (p.phone || "").trim();
      if (!p.name) errors.name = "Ingresá tu nombre y apellido.";
      if (p.email && !EMAIL_PATTERN.test(p.email))
        errors.email = "Ingresá un correo válido.";
      if (p.phone && digitsOf(p.phone).length < 7)
        errors.phone = "Ingresá un celular válido.";
      profileErrors.value = errors;
      if (Object.keys(errors).length) return;
      persist();
      notify("Perfil actualizado");
    };
    const changePassword = async () => {
      const p = passwords.value,
        errors = {};
      if (!p.current) errors.current = "Ingresá tu contraseña actual.";
      if (!p.next) errors.next = "Ingresá una contraseña nueva.";
      else if (passwordHasWhitespace(p.next))
        errors.next = "La contraseña no puede contener espacios.";
      if (!p.confirm) errors.confirm = "Repetí la nueva contraseña.";
      else if (p.next && p.next !== p.confirm)
        errors.confirm = "Las contraseñas no coinciden.";
      if (
        !errors.current &&
        (await digest(p.current)) !== currentUser.value.passwordHash
      )
        errors.current = "La contraseña actual no coincide.";
      passwordErrors.value = errors;
      if (Object.keys(errors).length) return;
      currentUser.value.passwordHash = await digest(p.next);
      persist();
      passwords.value = { current: "", next: "", confirm: "" };
      modal.value = "";
      notify("Contraseña actualizada");
    };
    Vue.watch(modal, (m) => {
      if (m === "password") {
        passwords.value = { current: "", next: "", confirm: "" };
        passwordErrors.value = {};
      }
    });
    // Contactos: personas de la app, identificadas por su @. Para transferir
    // se usa su alias (o el número de cuenta si todavía no eligió uno).
    const contacts = computed(() => {
      const lista = [];
      for (const user of db.users) {
        if (user.id === currentUser.value.id) continue;
        const cuenta = user.accounts.find((a) => a.status === "ACTIVA");
        if (!cuenta) continue;
        lista.push({
          name: user.profile.name,
          handle: user.username,
          alias: cuenta.alias || "",
          aliasVisible: cuenta.alias
            ? Bank.formatearAlias(cuenta.alias)
            : cuenta.number,
          destination: cuenta.alias || cuenta.number,
          hasAlias: !!cuenta.alias,
        });
      }
      return lista;
    });
    const menu = [
      { title: "Pagar QR", subtitle: "", icon: "qr", page: "qr" },
      {
        title: "Mis tarjetas",
        subtitle: "Estado, controles y límites",
        icon: "wallet",
        page: "tarjetas",
      },
      {
        title: "Mis alias",
        subtitle: "Tu cédula o celular para recibir",
        icon: "copy",
        page: "alias",
      },
      {
        title: "Movimientos",
        subtitle: "Consultá y exportá tu actividad",
        icon: "history",
        page: "movimientos",
      },
      {
        title: "Seguridad y ajustes",
        subtitle: "Acceso, privacidad y avisos",
        icon: "shield",
        page: "ajustes",
      },
      {
        title: "Notificaciones",
        subtitle: "Las novedades de tu cuenta",
        icon: "bell",
        page: "notificaciones",
      },
      {
        title: "Ayuda y contacto",
        subtitle: "Respuestas cuando las necesitás",
        icon: "help",
        page: "ayuda",
      },
    ];
    const notices = ref([]);
    const faqs = [
      {
        q: "¿Qué diferencia hay entre mi usuario (@) y mi alias?",
        a: "El usuario (@) es tu identidad dentro de la app: lo usás para entrar y para que te encuentren tus contactos. El alias es tu cédula o tu celular, y es el dato que se comparte entre bancos para que te transfieran.",
      },
      {
        q: "¿Por qué no puedo elegir mi cédula como alias?",
        a: "Porque el alias es único en todo el sistema. Si ese dato ya está usado como alias en otro banco, no se puede repetir.",
      },
      {
        q: "¿Cuántas cuentas y alias puedo tener?",
        a: "Hasta cinco cuentas. Como el alias se elige entre tu cédula y tu celular, como máximo podés tener dos alias.",
      },
      {
        q: "¿Cómo pido otra cuenta?",
        a: "Desde Inicio, tocá «Solicitar nuevo producto» y elegí cuenta corriente o caja de ahorro. No hace falta volver a registrarte.",
      },
      {
        q: "¿Cómo envío dinero a otro banco?",
        a: "Entrá a Enviar, elegí el banco destino e ingresá el alias o cuenta. Indicá el importe y revisá los datos antes de confirmar.",
      },
      {
        q: "¿Qué pasa si una transferencia queda pendiente?",
        a: "Todavía no hay confirmación del banco destino. Revisá el estado en tus movimientos antes de iniciar otra operación. El importe queda reservado.",
      },
      {
        q: "¿Cómo recibo dinero?",
        a: "Desde Recibir podés copiar tu alias o compartir los datos de tu cuenta. El QR contiene el banco, titular, cuenta y alias.",
      },
      {
        q: "¿Cuáles son los límites?",
        a: "Hasta 10.000.000 Gs. por transferencia y 10.000.000 Gs. diarios combinando transferencias salientes y retiros.",
      },
      {
        q: "¿Qué significa una cuenta bloqueada o cerrada?",
        a: "Una cuenta bloqueada no puede enviar ni recibir dinero. Una cuenta cerrada no admite operaciones. Podés consultar ambas desde Cuentas.",
      },
    ];
    const qrData = computed(() => {
      const code = qrcode(0, "M");
      code.addData(
        JSON.stringify({
          banco: "SUDAMERIS",
          cuenta: account.value.number,
          alias: accountAlias.value,
          titular: profile.value.name,
        }),
        "Byte",
      );
      code.make();
      return code;
    });
    const qrSize = computed(() => qrData.value.getModuleCount() + 8);
    const qrPath = computed(() => {
      let d = "";
      const code = qrData.value;
      for (let y = 0; y < code.getModuleCount(); y++)
        for (let x = 0; x < code.getModuleCount(); x++)
          if (code.isDark(y, x)) d += `M${x + 4} ${y + 4}h1v1h-1Z`;
      return d;
    });
    return {
      accountAlias,
      aliasRaw,
      profile,
      username,
      accounts,
      transactions,
      settings,
      cardFrozen,
      logged,
      page,
      auth,
      credentials,
      registration,
      authError,
      showPassword,
      activeAccount,
      balanceVisible,
      homeTab,
      toast,
      modal,
      search,
      filter,
      transferStep,
      transfer,
      transferError,
      busy,
      paymentKind,
      receipt,
      unread,
      supportSent,
      passwords,
      passwordErrors,
      profileErrors,
      fieldErrors,
      supportError,
      setAuth,
      aliasChoice,
      aliasOptions,
      aliasTarget,
      aliasError,
      abrirAlias,
      cerrarAlias,
      guardarAlias,
      nuevaCuenta,
      nuevaCuentaError,
      abrirNuevaCuenta,
      crearCuenta,
      tiposCuenta,
      tipoLabel,
      estadoLabel,
      formatearAlias,
      usernameEdited,
      sugerirUsuario,
      account,
      initials,
      firstName,
      money,
      persist,
      notify,
      go,
      logout,
      authenticate,
      startTransfer,
      advanceTransfer,
      openReceipt,
      filteredTransactions,
      copy,
      shareAccount,
      downloadReceipt,
      exportHistory,
      saveProfile,
      changePassword,
      contacts,
      menu,
      notices,
      faqs,
      qrPath,
      qrSize,
      paymentCode,
      paymentError,
      readPaymentCode,
      support,
      saveSupport,
    };
  },
});
app.component("icon", {
  props: ["name"],
  template:
    '<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path :d="d"/></svg>',
  computed: {
    d() {
      return paths[this.name] || paths.grid;
    },
  },
});
app.component("page-title", {
  props: ["title", "subtitle"],
  emits: ["back"],
  template:
    '<div class="page-title"><button @click="$emit(\'back\')">← Volver</button><h2>{{title}}</h2><p v-if="subtitle">{{subtitle}}</p></div>',
});
app.mount("#app");