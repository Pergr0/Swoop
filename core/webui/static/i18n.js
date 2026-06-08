(function () {
  function detectLocale() {
    const langs = navigator.languages?.length ? navigator.languages : [navigator.language || "en"];
    for (const raw of langs) {
      const base = (raw || "").split("-")[0].toLowerCase();
      if (base === "ru") return "ru";
    }
    return "en";
  }

  const en = {
    pageTitle: "Swoop — share with computer",
    pageSub:
      "Share files with the computer on this network. 1:1 pairing — this page is linked only to this PC.",
    connecting: "Connecting…",
    fingerprint: "Fingerprint",
    chatTitle: "Messages",
    chatEmpty: "Short notes and links with the computer.",
    chatPlaceholder: "Message or link…",
    chatSend: "Send",
    chatRead: "Read",
    chatDelivered: "Delivered",
    pullTitle: "Receive from computer",
    pullHint:
      "Page is open — the computer can send files. A request will appear here after you press Send on the PC.",
    pullAccept: "Accept and download",
    pullDecline: "Decline",
    sendTitle: "Send to computer",
    pickFiles: "Choose files",
    noFiles: "No files selected",
    sendBtn: "Send",
    phone: "Phone",
    browser: "Browser",
    computer: "Computer",
    hostInfoError: "Could not load host info",
    filesSummary: "{n} file(s) · {size}",
    wantsToSend: "{name} wants to send files",
    willDownloadZip: " · will download as .zip",
    andMore: "… {n} more",
    downloading: "Downloading",
    archivePrefix: "Archive ",
    transferDeclined: "Transfer declined",
    declined: "Declined",
    errorHttp: "Error: HTTP {code}",
    pullDone: "Done! Files saved.",
    waitingDesktop: "Waiting for confirmation on the computer…",
    recipientDeclined: "Recipient declined the transfer",
    hostBusy: "Computer is busy with another transfer",
    timedOut: "Timed out",
    handshakeError: "Handshake error: HTTP {code}",
    noBrowserUpload: "Host does not support browser upload",
    uploading: "Uploading…",
    uploadDone: "Done! Files are on the computer.",
    downloadError: "Download error: HTTP {code}",
    uploadError: "Upload error: HTTP {code}",
    networkError: "Network unavailable",
    archiveEmpty: "Archive not received (empty response)",
    unexpectedResponse: "Unexpected server response",
    incompleteFile: "File not received completely",
  };

  const ru = {
    pageTitle: "Swoop — обмен с компьютером",
    pageSub:
      "Обмен файлами с компьютером в этой сети. Подключение 1 к 1 — эта страница связана только с этим ПК.",
    connecting: "Подключение…",
    fingerprint: "Отпечаток",
    chatTitle: "Сообщения",
    chatEmpty: "Короткие заметки и ссылки с компьютером.",
    chatPlaceholder: "Сообщение или ссылка…",
    chatSend: "Отпр.",
    chatRead: "Прочитано",
    chatDelivered: "Доставлено",
    pullTitle: "Получить с компьютера",
    pullHint:
      "Страница открыта — компьютер может отправить файлы. Запрос появится здесь после нажатия «Отправить» на ПК.",
    pullAccept: "Принять и скачать",
    pullDecline: "Отклонить",
    sendTitle: "Отправить на компьютер",
    pickFiles: "Выбрать файлы",
    noFiles: "Файлы не выбраны",
    sendBtn: "Отправить",
    phone: "Телефон",
    browser: "Браузер",
    computer: "Компьютер",
    hostInfoError: "не удалось получить информацию о хосте",
    filesSummary: "{n} файл(ов) · {size}",
    wantsToSend: "{name} хочет отправить файлы",
    willDownloadZip: " · будет скачан .zip",
    andMore: "… ещё {n}",
    downloading: "Скачивание",
    archivePrefix: "Архив ",
    transferDeclined: "Передача отклонена",
    declined: "Отклонено",
    errorHttp: "Ошибка: HTTP {code}",
    pullDone: "Готово! Файлы сохранены.",
    waitingDesktop: "Ожидание подтверждения на компьютере…",
    recipientDeclined: "Получатель отклонил передачу",
    hostBusy: "Компьютер занят другой передачей",
    timedOut: "Время ожидания истекло",
    handshakeError: "Ошибка handshake: HTTP {code}",
    noBrowserUpload: "Хост не поддерживает загрузку из браузера",
    uploading: "Загрузка…",
    uploadDone: "Готово! Файлы на компьютере.",
    downloadError: "Ошибка скачивания: HTTP {code}",
    uploadError: "Ошибка загрузки: HTTP {code}",
    networkError: "Сеть недоступна",
    archiveEmpty: "архив не получен (пустой ответ)",
    unexpectedResponse: "неожиданный ответ сервера",
    incompleteFile: "файл получен не полностью",
  };

  const locale = detectLocale();
  const dict = locale === "ru" ? ru : en;

  function t(key, params) {
    let s = dict[key] || en[key] || key;
    if (params) {
      for (const [k, v] of Object.entries(params)) {
        s = s.split("{" + k + "}").join(String(v));
      }
    }
    return s;
  }

  function applyStatic() {
    document.documentElement.lang = locale;
    document.title = t("pageTitle");
    const sub = document.querySelector(".sub");
    if (sub) sub.textContent = t("pageSub");
    const hostName = document.getElementById("hostName");
    if (hostName) hostName.textContent = t("connecting");
    const titles = document.querySelectorAll(".card-title");
    if (titles[0]) titles[0].textContent = t("chatTitle");
    if (titles[1]) titles[1].textContent = t("pullTitle");
    if (titles[2]) titles[2].textContent = t("sendTitle");
    const chatEmpty = document.getElementById("chatEmpty");
    if (chatEmpty) chatEmpty.textContent = t("chatEmpty");
    const chatInput = document.getElementById("chatInput");
    if (chatInput) chatInput.placeholder = t("chatPlaceholder");
    const chatSend = document.getElementById("chatSend");
    if (chatSend) chatSend.textContent = t("chatSend");
    const pullHint = document.getElementById("pullHint");
    if (pullHint) pullHint.textContent = t("pullHint");
    const pullAccept = document.getElementById("pullAccept");
    if (pullAccept) pullAccept.textContent = t("pullAccept");
    const pullDecline = document.getElementById("pullDecline");
    if (pullDecline) pullDecline.textContent = t("pullDecline");
    const pickLabel = document.querySelector('label[for="pick"]');
    if (pickLabel) pickLabel.textContent = t("pickFiles");
    const fileList = document.getElementById("fileList");
    if (fileList) fileList.textContent = t("noFiles");
    const send = document.getElementById("send");
    if (send) send.textContent = t("sendBtn");
  }

  window.SwoopI18n = { locale, t, applyStatic };
  applyStatic();
})();
