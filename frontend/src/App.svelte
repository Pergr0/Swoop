<script lang="ts">
  import { onMount, onDestroy, tick } from "svelte";
  import {
    Peers,
    SelfInfo,
    ScanPaths,
    OpenFilePicker,
    OpenFolderPicker,
    SendTo,
    CancelOutgoing,
    CancelIncoming,
    RespondIncoming,
    DownloadsPath,
    RevealDownloads,
    Interfaces,
    StartEngine,
    SendMessage,
    ChatHistory,
    MarkRead,
    GenerateInvite,
    SaveInviteBundle,
    ImportInviteFile,
  } from "../wailsjs/go/main/App.js";
  import { EventsOn, OnFileDrop, OnFileDropOff } from "../wailsjs/runtime/runtime.js";
  import SwoopLogo from "./components/SwoopLogo.svelte";
  import DiscoveryIcon from "./components/DiscoveryIcon.svelte";
  import PlatformIcon from "./components/PlatformIcon.svelte";
  import NetIcon from "./components/NetIcon.svelte";
  import TransferPanel from "./components/TransferPanel.svelte";
  import ChevronIcon from "./components/ChevronIcon.svelte";
  import QrModal from "./components/QrModal.svelte";
  import InviteModal from "./components/InviteModal.svelte";
  import QrCodeImage from "./components/QrCodeImage.svelte";
  import IconButton from "./components/IconButton.svelte";
  import GlobeIcon from "./components/GlobeIcon.svelte";
  import ImportIcon from "./components/ImportIcon.svelte";
  import { t, localizeError, discoveryLabelFor, folderCountLabel } from "./i18n";

  interface DeviceInfo {
    id: string; name: string; host: string; address: string;
    platform: string; controlPort: number; fingerprint: string; version: number;
    browser?: string;
  }
  interface NetInterface {
    name: string;
    displayName?: string;
    ssid?: string;
    addresses: string[];
    kind: string;
    up: boolean;
    speedMbps: number;
    recommended?: boolean;
  }
  interface StagingEntry {
    path: string; name: string; kind: string; relPath: string;
    size: number; fileCount: number; children?: StagingEntry[];
  }
  interface RootDirInfo { name: string; size: number; fileCount: number; }
  interface FileMeta { name: string; relPath: string; size: number; }
  interface SendItem { path: string; relPath: string; }
  interface Offer {
    sender: DeviceInfo; totalSize: number; count: number;
    rootDirs: RootDirInfo[]; looseFiles: number; files?: FileMeta[];
  }
  interface TreeRow { entry: StagingEntry; depth: number; }
  interface Progress {
    direction: string; bytes: number; total: number; speed: number;
    etaSeconds: number; streams: number; fileIndex: number; fileName: string; peer: string;
  }
  interface TState { direction: string; state: string; message: string; peer: string; }
  interface ChatMsg { ts: number; peerId: string; peerName: string; dir: string; text: string; read?: boolean; }

  let self: DeviceInfo | null = null;
  let peers: DeviceInfo[] = [];
  let downloadsPath = "";

  let started = false;
  let starting = false;
  let interfaces: NetInterface[] = [];
  let chosenIface = ""; // "" = auto
  let startError = "";

  let view: "grid" | "device" = "grid";
  let selected: DeviceInfo | null = null;

  let stagingRoots: StagingEntry[] = [];
  let expandedDirs: Record<string, boolean> = {};
  let selectedFiles: Record<string, boolean> = {};
  let stagingError = "";
  let showAdvanced = false;

  let sendState: TState | null = null;
  let sendProgress: Progress | null = null;

  let incoming: Offer | null = null;
  let recvState: TState | null = null;
  let recvProgress: Progress | null = null;

  let chatMessages: ChatMsg[] = [];
  let chatInput = "";
  let chatSending = false;
  let chatError = "";
  let chatListEl: HTMLDivElement | null = null;
  let chatExpanded = false;
  let dropHover = false;
  let dragDepth = 0;
  let gridDropTarget: string | null = null;
  let tileDragDepth: Record<string, number> = {};
  let suppressDeviceClick = false;

  function canWindowDrop(): boolean {
    return view === "device" && !sending;
  }
  function canGridDrop(): boolean {
    return started && view === "grid" && peers.length > 0 && !sending;
  }
  function onDragEnter(e: DragEvent) {
    if (!canWindowDrop()) return;
    e.preventDefault();
    dragDepth++;
    dropHover = true;
  }
  function onDragLeave(e: DragEvent) {
    if (!canWindowDrop()) return;
    e.preventDefault();
    dragDepth = Math.max(0, dragDepth - 1);
    if (dragDepth === 0) dropHover = false;
  }
  function onDragOver(e: DragEvent) {
    if (!canWindowDrop()) return;
    e.preventDefault();
  }
  function onDragDrop(e: DragEvent) {
    if (!canWindowDrop()) return;
    e.preventDefault();
    dragDepth = 0;
    dropHover = false;
  }
  let unread: Record<string, number> = {};
  let copyFeedback = "";
  let copyTimer: ReturnType<typeof setTimeout> | null = null;
  let showQr = false;
  let internetInvite: { blob: string; shortCode: string; expiresAt: number; deviceName: string } | null = null;
  let showInviteModal = false;
  let inviteModalLoading = false;
  let inviteFeedback = "";

  const IFACE_KEY = "swoop-iface";

  let unsub: Array<() => void> = [];
  let poll: ReturnType<typeof setInterval> | null = null;

  const shortFp = (fp: string) => fp.replace(/^sha256:/, "").slice(0, 8).toUpperCase();
  function peerEndpoint(p: DeviceInfo): string {
    const addr = p.address || p.host;
    return p.controlPort > 0 ? `${addr}:${p.controlPort}` : addr;
  }
  function peerSubtitle(p: DeviceInfo): string {
    const ep = peerEndpoint(p);
    if (p.platform === "web" && p.browser) return `${ep} · ${p.browser}`;
    return ep;
  }
  function isWebPeer(p: DeviceInfo): boolean {
    return p.platform === "web";
  }
  function webUploadURL(s: DeviceInfo): string {
    const addr = s.address || s.host;
    if (!addr || !s.controlPort) return "";
    return `https://${addr}:${s.controlPort}/`;
  }
  function ifaceLabel(it: NetInterface): string {
    return it.displayName || it.name;
  }
  function defaultIfaceChoice(list: NetInterface[]): string {
    const rec = list.find((i) => i.recommended);
    return rec ? rec.name : "";
  }
  function fmtSpeedMbps(mbps: number): string {
    if (!mbps || mbps <= 0) return "";
    if (mbps >= 1000) return `${(mbps / 1000).toFixed(mbps % 1000 === 0 ? 0 : 1)} Gbps`;
    return `${mbps} Mbps`;
  }

  function fmtBytes(n: number): string {
    if (!n) return "0 B";
    const u = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(n) / Math.log(1024));
    return `${(n / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1)} ${u[i]}`;
  }
  const fmtSpeed = (bps: number) => `${fmtBytes(bps)}/s`;
  function fmtETA(sec: number): string {
    if (!sec || !isFinite(sec)) return "\u2014";
    const s = Math.round(sec);
    const m = Math.floor(s / 60);
    return m > 0 ? `${m}m ${s % 60}s` : `${s}s`;
  }

  function walkFiles(entries: StagingEntry[], fn: (e: StagingEntry) => void) {
    for (const e of entries) {
      if (e.kind === "file") fn(e);
      else if (e.children) walkFiles(e.children, fn);
    }
  }
  function visibleRows(entries: StagingEntry[], depth = 0): TreeRow[] {
    const rows: TreeRow[] = [];
    for (const e of entries) {
      rows.push({ entry: e, depth });
      if (e.kind === "dir" && expandedDirs[e.path] && e.children?.length) {
        rows.push(...visibleRows(e.children, depth + 1));
      }
    }
    return rows;
  }
  function dirCheckState(e: StagingEntry): "on" | "off" | "some" {
    let total = 0;
    let on = 0;
    walkFiles([e], (f) => { total++; if (selectedFiles[f.path]) on++; });
    if (on === 0) return "off";
    if (on === total) return "on";
    return "some";
  }
  function toggleDir(e: StagingEntry) {
    const turnOn = dirCheckState(e) !== "on";
    walkFiles([e], (f) => { selectedFiles[f.path] = turnOn; });
    selectedFiles = selectedFiles;
  }
  function toggleFile(path: string) {
    selectedFiles[path] = !selectedFiles[path];
    selectedFiles = selectedFiles;
  }
  function setCheckboxIndeterminate(node: HTMLInputElement, state: "on" | "off" | "some") {
    const apply = (s: "on" | "off" | "some") => { node.indeterminate = s === "some"; };
    apply(state);
    return { update: apply };
  }
  function rootDirsSize(dirs: RootDirInfo[]): number {
    return (dirs ?? []).reduce((a, d) => a + d.size, 0);
  }
  function toggleExpanded(path: string) {
    expandedDirs = { ...expandedDirs, [path]: !expandedDirs[path] };
  }

  $: treeRows = (expandedDirs, visibleRows(stagingRoots));
  $: selectedList = (() => {
    const out: SendItem[] = [];
    walkFiles(stagingRoots, (f) => {
      if (selectedFiles[f.path]) out.push({ path: f.path, relPath: f.relPath });
    });
    return out;
  })();
  $: selectedCount = selectedList.length;
  $: totalFileCount = (() => {
    let n = 0;
    walkFiles(stagingRoots, () => n++);
    return n;
  })();
  $: selectedTotal = (() => {
    let n = 0;
    walkFiles(stagingRoots, (f) => { if (selectedFiles[f.path]) n += f.size; });
    return n;
  })();
  $: sending = !!sendState && (sendState.state === "waiting" || sendState.state === "transferring");
  $: sendPct = sendProgress && sendProgress.total > 0 ? (sendProgress.bytes / sendProgress.total) * 100 : 0;
  $: recvPct = recvProgress && recvProgress.total > 0 ? (recvProgress.bytes / recvProgress.total) * 100 : 0;
  $: discoveryLabel = discoveryLabelFor(peers.length);
  $: mobileQrUrl = started && self ? webUploadURL(self) : "";
  $: chatBadge =
    selected && unread[selected.id]
      ? unread[selected.id]
      : 0;
  $: incomingPreview = incoming?.files?.length
    ? incoming.files.slice(0, 5)
    : [];
  $: incomingMore = incoming?.files?.length
    ? Math.max(0, (incoming.files?.length ?? 0) - 5)
    : 0;

  async function refresh() { peers = ((await Peers()) as DeviceInfo[]) ?? []; }

  async function selectDevice(p: DeviceInfo) {
    selected = p;
    view = "device";
    sendState = null;
    sendProgress = null;
    chatError = "";
    chatInput = "";
    if (unread[p.id]) { delete unread[p.id]; unread = unread; }
    chatMessages = ((await ChatHistory(p.id)) as ChatMsg[]) ?? [];
    scrollChatSoon();
    markReadSoon(p.id);
  }
  function onDeviceClick(p: DeviceInfo) {
    if (suppressDeviceClick) {
      suppressDeviceClick = false;
      return;
    }
    selectDevice(p);
  }
  function clearGridDropHover() {
    gridDropTarget = null;
    tileDragDepth = {};
  }
  function peerAtDropPoint(x: number, y: number): DeviceInfo | null {
    const el = document.elementFromPoint(x, y);
    const tile = el?.closest("[data-device-id]") as HTMLElement | null;
    if (!tile?.dataset.deviceId) return null;
    return peers.find((p) => p.id === tile.dataset.deviceId) ?? null;
  }
  function onTileDragEnter(e: DragEvent, peerId: string) {
    if (!canGridDrop()) return;
    e.preventDefault();
    e.stopPropagation();
    tileDragDepth[peerId] = (tileDragDepth[peerId] ?? 0) + 1;
    tileDragDepth = tileDragDepth;
    gridDropTarget = peerId;
  }
  function onTileDragLeave(e: DragEvent, peerId: string) {
    if (!canGridDrop()) return;
    e.preventDefault();
    e.stopPropagation();
    const next = Math.max(0, (tileDragDepth[peerId] ?? 0) - 1);
    if (next === 0) {
      delete tileDragDepth[peerId];
      if (gridDropTarget === peerId) gridDropTarget = null;
    } else {
      tileDragDepth[peerId] = next;
    }
    tileDragDepth = tileDragDepth;
  }
  function onTileDragOver(e: DragEvent, peerId: string) {
    if (!canGridDrop()) return;
    e.preventDefault();
    e.stopPropagation();
    gridDropTarget = peerId;
  }
  async function sendDroppedToDevice(p: DeviceInfo, paths: string[]) {
    if (!paths?.length || sending) return;
    suppressDeviceClick = true;
    clearGridDropHover();
    clearStaged();
    try {
      await selectDevice(p);
      await addPaths(paths);
      await tick();
      if (selectedCount === 0) return;
      await doSend();
    } finally {
      setTimeout(() => { suppressDeviceClick = false; }, 0);
    }
  }
  type DropPayload = string[] | { paths: string[]; x?: number; y?: number };
  function parseDropPayload(payload: DropPayload): { paths: string[]; x: number; y: number } {
    if (Array.isArray(payload)) return { paths: payload, x: 0, y: 0 };
    return { paths: payload.paths ?? [], x: payload.x ?? 0, y: payload.y ?? 0 };
  }
  function handleFileDrop(x: number, y: number, paths: string[]) {
    if (!started || !paths?.length || sending) return;
    if (view === "device") {
      addPaths(paths);
      return;
    }
    if (view === "grid" && peers.length > 0) {
      const peer =
        peerAtDropPoint(x, y) ??
        (gridDropTarget ? peers.find((p) => p.id === gridDropTarget) ?? null : null);
      if (peer) sendDroppedToDevice(peer, paths);
    }
  }

  function markReadSoon(peerId: string) {
    if (chatMessages.some((m) => m.dir === "in")) {
      MarkRead(peerId).catch(() => {});
    }
  }
  function back() {
    view = "grid";
    chatMessages = [];
    dragDepth = 0;
    dropHover = false;
    clearGridDropHover();
  }

  function scrollChatSoon() {
    setTimeout(() => { if (chatListEl) chatListEl.scrollTop = chatListEl.scrollHeight; }, 0);
  }
  function fmtTime(ms: number): string {
    return new Date(ms).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  }
  function copyText(t: string) {
    if (navigator.clipboard) {
      navigator.clipboard.writeText(t);
      copyFeedback = t("chat.copied");
      if (copyTimer) clearTimeout(copyTimer);
      copyTimer = setTimeout(() => (copyFeedback = ""), 1600);
    }
  }
  async function prepareInternetInvite() {
    if (!started) return;
    inviteFeedback = "";
    showInviteModal = true;
    inviteModalLoading = true;
    internetInvite = null;
    try {
      internetInvite = (await GenerateInvite()) as typeof internetInvite;
    } catch (e) {
      internetInvite = null;
      showInviteModal = false;
      inviteFeedback = localizeError(String(e));
    } finally {
      inviteModalLoading = false;
    }
  }
  function closeInviteModal() {
    showInviteModal = false;
    internetInvite = null;
  }
  async function downloadInviteFile() {
    if (!internetInvite) return;
    try {
      await SaveInviteBundle(internetInvite);
    } catch (e) {
      const msg = String(e);
      if (msg) inviteFeedback = localizeError(msg);
    }
  }
  async function importInvite() {
    if (!started) return;
    inviteFeedback = "";
    try {
      const res = (await ImportInviteFile()) as { device: DeviceInfo; shortCode: string };
      if (!res?.device) return;
      inviteFeedback = t("invite.importOk", { name: res.device.name, code: res.shortCode });
    } catch (e) {
      const msg = String(e);
      if (msg.includes("expired") || msg.includes("истёк")) {
        inviteFeedback = t("invite.expired");
      } else if (msg) {
        inviteFeedback = localizeError(msg);
      }
    }
  }
  async function revealDownloads() {
    try {
      await RevealDownloads();
    } catch (e) {
      stagingError = localizeError(String(e));
    }
  }
  function previewName(f: FileMeta): string {
    return f.relPath || f.name;
  }
  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      if (showQr) showQr = false;
      else if (incoming && recvState?.state !== "transferring") dismissIncoming();
      else if (view === "device") back();
    }
  }
  function saveIfaceChoice(iface: string) {
    chosenIface = iface;
    try {
      localStorage.setItem(IFACE_KEY, iface);
    } catch {
      /* ignore */
    }
  }
  async function sendMsg() {
    const text = chatInput.trim();
    if (!selected || !text || chatSending) return;
    chatSending = true;
    chatError = "";
    try {
      await SendMessage(selected.id, text);
      chatInput = "";
    } catch (e) {
      chatError = localizeError(String(e));
    } finally {
      chatSending = false;
    }
  }

  async function addPaths(paths: string[]) {
    if (!paths || paths.length === 0) return;
    stagingError = "";
    try {
      const added = ((await ScanPaths(paths)) as StagingEntry[]) ?? [];
      const known = new Set(stagingRoots.map((r) => r.path));
      for (const e of added) {
        if (!known.has(e.path)) {
          stagingRoots = [...stagingRoots, e];
          known.add(e.path);
        }
      }
      walkFiles(added, (f) => {
        if (selectedFiles[f.path] === undefined) selectedFiles[f.path] = true;
      });
      selectedFiles = selectedFiles;
    } catch (e) {
      stagingError = localizeError(String(e));
    }
  }
  function clearStaged() {
    stagingRoots = [];
    expandedDirs = {};
    selectedFiles = {};
    stagingError = "";
  }
  async function pickFiles() { await addPaths((await OpenFilePicker()) as string[]); }
  async function pickFolder() {
    const path = (await OpenFolderPicker()) as string;
    if (path) await addPaths([path]);
  }

  async function doSend() {
    if (!selected || selectedCount === 0 || sending) return;
    sendState = { direction: "send", state: "waiting", message: t("transfer.waitingAccept"), peer: selected.name };
    sendProgress = null;
    try {
      await SendTo(selected.id, selectedList);
    } catch (e) {
      sendState = { direction: "send", state: "failed", message: localizeError(String(e)), peer: selected.name };
    }
  }
  function cancelSend() { CancelOutgoing(); }
  function cancelRecv() { CancelIncoming(); }

  function accept() { RespondIncoming(true); }
  function decline() { RespondIncoming(false); incoming = null; recvState = null; recvProgress = null; }
  function dismissIncoming() { incoming = null; recvState = null; recvProgress = null; }

  async function startWith(iface: string) {
    if (starting || started) return;
    starting = true;
    startError = "";
    saveIfaceChoice(iface);
    try {
      await StartEngine(iface);
    } catch (e) {
      startError = localizeError(String(e));
      starting = false;
      return;
    }
    self = (await SelfInfo()) as DeviceInfo;
    downloadsPath = (await DownloadsPath()) as string;
    await tick();
    nudgeLayout();
    await refresh();
    unsub.push(EventsOn("peers:changed", (d: DeviceInfo[]) => (peers = d ?? [])));
    unsub.push(EventsOn("files:dropped", (payload: DropPayload) => {
      const { paths, x, y } = parseDropPayload(payload);
      handleFileDrop(x, y, paths);
    }));
    unsub.push(EventsOn("transfer:offer", (o: Offer) => { incoming = o; recvState = null; recvProgress = null; }));
    unsub.push(EventsOn("transfer:progress", (p: Progress) => {
      if (p.direction === "send") sendProgress = p; else recvProgress = p;
    }));
    unsub.push(EventsOn("transfer:state", (s: TState) => {
      if (s.direction === "send") sendState = s;
      else recvState = s;
    }));
    unsub.push(EventsOn("chat:message", (m: ChatMsg) => {
      if (selected && view === "device" && m.peerId === selected.id) {
        chatMessages = [...chatMessages, m];
        scrollChatSoon();
        if (m.dir === "in") MarkRead(m.peerId).catch(() => {});
      } else if (m.dir === "in") {
        unread[m.peerId] = (unread[m.peerId] ?? 0) + 1;
        unread = unread;
      }
    }));
    unsub.push(EventsOn("chat:read", (peerId: string, upToTs: number) => {
      if (selected && view === "device" && peerId === selected.id) {
        chatMessages = chatMessages.map((m) =>
          m.dir === "out" && m.ts <= upToTs ? { ...m, read: true } : m,
        );
      }
    }));
    poll = setInterval(refresh, 4000);
    started = true;
    starting = false;
  }

  function nudgeLayout() {
    requestAnimationFrame(() => window.dispatchEvent(new Event("resize")));
  }

  onMount(async () => {
    OnFileDrop(() => {}, false);
    unsub.push(() => OnFileDropOff());
    nudgeLayout();

    interfaces = ((await Interfaces()) as NetInterface[]) ?? [];
    try {
      const saved = localStorage.getItem(IFACE_KEY);
      if (saved !== null) {
        chosenIface = saved;
      } else {
        chosenIface = defaultIfaceChoice(interfaces);
      }
    } catch {
      chosenIface = defaultIfaceChoice(interfaces);
    }
  });
  onDestroy(() => {
    unsub.forEach((u) => u());
    if (poll) clearInterval(poll);
    if (copyTimer) clearTimeout(copyTimer);
  });
</script>

<svelte:window on:keydown={onKeydown} />

<main
  class="app-main"
  class:window-drop={view === "device" && !sending}
  class:drop-active={dropHover}
  on:dragenter={onDragEnter}
  on:dragleave={onDragLeave}
  on:dragover={onDragOver}
  on:drop={onDragDrop}
>
  {#if view === "device" && dropHover && !sending}
    <div class="drop-overlay" aria-hidden="true">
      <span class="drop-overlay-text">{t("drop.overlay")}</span>
    </div>
  {/if}
  <header class="app-header">
    <div class="brand">
      <SwoopLogo size={34} />
      <div class="brand-text">
        <h1>Swoop</h1>
        {#if started}
          <p class="brand-status">{discoveryLabel}</p>
        {/if}
      </div>
    </div>
    <div class="header-end">
      {#if self}
        <div class="header-self">
          {#if started}
            <IconButton
              title={t("invite.internetBtnTitle")}
              ariaLabel={t("invite.internetBtnAria")}
              on:click={prepareInternetInvite}
            >
              <GlobeIcon />
            </IconButton>
          {/if}
          {#if started && webUploadURL(self)}
            <IconButton
              title={t("qr.btnTitle")}
              ariaLabel={t("qr.btnAria")}
              on:click={() => (showQr = true)}
            >
              <svg class="qr-glyph" viewBox="0 0 24 24" aria-hidden="true">
                <path fill="currentColor" d="M3 3h8v8H3V3zm2 2v4h4V5H5zm8-2h8v8h-8V3zm2 2v4h4V5h-4zM3 13h8v8H3v-8zm2 2v4h4v-4H5zm13-2h2v2h-2v-2zm-2 2h2v2h-2v-2zm2 2h2v2h-2v-2zm-2 2h2v2h-2v-2zm2 2h2v2h-2v-2zM13 17h2v2h-2v-2zm2-4h2v2h-2v-2z"/>
              </svg>
            </IconButton>
          {/if}
          <div class="self-card">
            <PlatformIcon platform={self.platform} size={40} />
            <div class="self-meta">
              <div class="self-name">{self.name}</div>
              <div class="self-sub">{t("devices.thisDevice")} · {peerEndpoint(self)}</div>
            </div>
          </div>
        </div>
      {/if}
    </div>
  </header>

  <QrModal
    open={showQr}
    url={self ? webUploadURL(self) : ""}
    onClose={() => (showQr = false)}
  />

  <InviteModal
    open={showInviteModal}
    bundle={internetInvite}
    loading={inviteModalLoading}
    onClose={closeInviteModal}
    onDownload={downloadInviteFile}
  />

  {#if sending && view === "grid"}
    <div class="transfer-float">
      <TransferPanel
        compact
        title="{sendState && sendState.state === 'waiting' ? t('transfer.waitingAccept') : t('transfer.sending')}{sendState ? ` · ${sendState.peer}` : ''}"
        indeterminate={sendState?.state === "waiting"}
        percent={sendPct}
        bytesLabel="{sendProgress ? fmtBytes(sendProgress.bytes) : '0 B'} / {fmtBytes(selectedTotal)}"
        speedLabel={sendProgress ? fmtSpeed(sendProgress.speed) : "\u2014"}
        etaLabel=""
        showEta={false}
      />
      <button class="btn-danger" on:click={cancelSend}>{t("transfer.cancel")}</button>
    </div>
  {/if}

  {#if view === "grid"}
    <section class="devices scroll-y view-panel">
      <div class="devices-header">
        {#if started}
          <button
            type="button"
            class="btn-secondary btn-with-icon"
            title={t("invite.importBtnTitle")}
            aria-label={t("invite.importBtnAria")}
            on:click={importInvite}
          >
            <ImportIcon size={32} />
            <span>{t("invite.importBtn")}</span>
          </button>
        {/if}
        <h2 class="section-title">{t("devices.title")} <span class="count">{peers.length}</span></h2>
      </div>
      {#if inviteFeedback}
        <p class="invite-feedback">{inviteFeedback}</p>
      {/if}
      {#if peers.length === 0}
        <div class="empty">
          <div class="empty-foreground">
            <div class="empty-art" aria-hidden="true">
              <DiscoveryIcon size={56} />
            </div>
            <p>{t("discovery.emptyTitle")}</p>
            <small>{t("discovery.emptyHint")}</small>
          </div>
          {#if mobileQrUrl}
            <div class="empty-qr-panel">
              <QrCodeImage url={mobileQrUrl} size={200} alt={t("qr.alt")} muted />
            </div>
          {/if}
        </div>
      {:else}
        <div class="grid">
          {#each peers as p (p.id)}
            <button
              class="device"
              class:device-drop-target={gridDropTarget === p.id}
              data-device-id={p.id}
              title="{isWebPeer(p) ? `${p.name} · ${p.browser || t('devices.browser')}` : `${p.name} · ${p.fingerprint}`}"
              on:click={() => onDeviceClick(p)}
              on:dragenter={(e) => onTileDragEnter(e, p.id)}
              on:dragleave={(e) => onTileDragLeave(e, p.id)}
              on:dragover={(e) => onTileDragOver(e, p.id)}
            >
              {#if gridDropTarget === p.id}
                <span class="device-drop-hint" aria-hidden="true">{t("drop.tileHint")}</span>
              {/if}
              {#if incoming?.sender.id === p.id}
                <span class="badge badge-transfer" title={t("transfer.incomingBadge")} aria-label={t("transfer.incomingBadge")}>↓</span>
              {:else if unread[p.id] && !isWebPeer(p)}
                <span class="badge">{unread[p.id]}</span>
              {/if}
              <PlatformIcon platform={p.platform} size={48} />
              <div class="device-name">
                <span class="device-online" title={t("devices.online")} aria-hidden="true"></span>
                <span>{p.name}</span>
              </div>
              <div class="device-host">{peerSubtitle(p)}</div>
            </button>
          {/each}
        </div>
      {/if}
    </section>
  {:else if view === "device" && selected}
    <section class="card-view view-panel">
      <div class="device-top">
        <button type="button" class="btn-back btn-surface" on:click={back}>
          <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><path fill="currentColor" d="M15.4 5.4 9.8 11l5.6 5.6-1.8 1.8L6.2 11l7.4-7.4 1.8 1.8z"/></svg>
          <span>{t("devices.back")}</span>
        </button>
        <div class="device-top-meta">
          <PlatformIcon platform={selected.platform} size={36} />
          <div>
            <div class="device-top-name">{selected.name}</div>
            <div class="device-top-sub">
              {#if isWebPeer(selected)}
                {selected.browser || t("devices.browser")} · {peerEndpoint(selected)}
              {:else}
                {t("devices.fingerprint")} {shortFp(selected.fingerprint)}
              {/if}
            </div>
          </div>
        </div>
      </div>

      <div class="transfer-stack">
      <div class="transfer-area">
      {#if sending || (sendProgress && sendState && sendState.state === "transferring")}
        <TransferPanel
          title={sendState && sendState.state === "waiting" ? t("transfer.waitingPeer") : t("transfer.sending")}
          indeterminate={sendState?.state === "waiting"}
          percent={sendPct}
          largePercent={sendState?.state === "transferring"}
          bytesLabel="{sendProgress ? fmtBytes(sendProgress.bytes) : '0 B'} / {fmtBytes(selectedTotal)}"
          speedLabel={sendProgress ? fmtSpeed(sendProgress.speed) : "\u2014"}
          etaLabel={t("transfer.etaRemaining", { eta: sendProgress ? fmtETA(sendProgress.etaSeconds) : "\u2014" })}
          showAdvanced={showAdvanced && !!sendProgress}
          advancedText={sendProgress ? t("transfer.streams", { streams: sendProgress.streams, file: sendProgress.fileName || "\u2014", pct: sendPct.toFixed(1) }) : ""}
        >
          <div slot="actions" class="panel-actions">
            <button class="link" on:click={() => (showAdvanced = !showAdvanced)}>
              {showAdvanced ? t("transfer.hide") : t("transfer.details")}
            </button>
            <button class="btn-danger" on:click={cancelSend}>{t("transfer.cancel")}</button>
          </div>
        </TransferPanel>
      {:else}
        {#if stagingError}
          <div class="banner banner-failed">{stagingError}</div>
        {/if}

        <div class="transfer-scroll scroll-y" class:transfer-scroll-empty={stagingRoots.length === 0}>
          {#if stagingRoots.length === 0}
            <div class="files-empty">
              <p>{t("drop.emptyTitle")}</p>
              <p class="files-empty-hint">{t("drop.emptyHint")}</p>
            </div>
          {:else}
            <div class="staged">
              <div class="staged-head">
                {t("files.stagedSummary", { items: stagingRoots.length, selected: selectedCount, total: totalFileCount, size: fmtBytes(selectedTotal) })}
              </div>
              <div class="staged-list">
                {#each treeRows as row (row.entry.path)}
                  {@const e = row.entry}
                  {@const check = e.kind === "dir" ? dirCheckState(e) : (selectedFiles[e.path] ? "on" : "off")}
                  <div class="staged-item" style="padding-left:{12 + row.depth * 18}px">
                    {#if e.kind === "dir"}
                      <button class="tree-toggle" on:click={() => toggleExpanded(e.path)} aria-label={expandedDirs[e.path] ? t("files.collapse") : t("files.expand")}>
                        <ChevronIcon expanded={!!expandedDirs[e.path]} size={22} />
                      </button>
                      <input
                        type="checkbox"
                        class="tree-check"
                        checked={check === "on"}
                        use:setCheckboxIndeterminate={check}
                        on:change={() => toggleDir(e)}
                      />
                      <span class="tree-icon">{"\u{1F4C1}"}</span>
                      <span class="staged-name" title={e.path}>{e.name}</span>
                      <span class="staged-size">{t("files.dirFiles", { count: e.fileCount, size: fmtBytes(e.size) })}</span>
                    {:else}
                      <span class="tree-spacer"></span>
                      <input
                        type="checkbox"
                        class="tree-check"
                        checked={!!selectedFiles[e.path]}
                        on:change={() => toggleFile(e.path)}
                      />
                      <span class="tree-icon">{"\u{1F4C4}"}</span>
                      <span class="staged-name" title={e.path}>{e.name}</span>
                      <span class="staged-size">{fmtBytes(e.size)}</span>
                    {/if}
                  </div>
                {/each}
              </div>
            </div>
          {/if}
        </div>

        {#if sendState && ["completed", "declined", "failed", "canceled"].includes(sendState.state)}
          <div class="banner banner-{sendState.state}">
            {#if sendState.state === "completed"}{t("transfer.completed")}
            {:else if sendState.state === "declined"}{t("transfer.declined")}
            {:else if sendState.state === "canceled"}{t("transfer.canceled")}
            {:else}{t("transfer.failed")}: {localizeError(sendState.message)}{/if}
            <span class="banner-hint">{t("transfer.listKept")}</span>
          </div>
        {/if}
      {/if}
      </div>

      {#if !sending && !(sendProgress && sendState && sendState.state === "transferring")}
        <div class="send-bar">
          <div class="send-bar-summary">
            {#if selectedCount > 0}
              {t("files.selectedSummary", { count: selectedCount, size: fmtBytes(selectedTotal) })}
            {:else}
              <span class="send-bar-empty">{t("files.noneSelected")}</span>
            {/if}
          </div>
          <div class="send-bar-actions">
            <button class="btn-secondary" on:click={pickFiles}>{t("files.pick")}</button>
            <button class="btn-secondary" on:click={pickFolder}>{t("files.folder")}</button>
            <button
              class="btn-secondary btn-clear"
              disabled={stagingRoots.length === 0}
              on:click={clearStaged}
            >
              {t("files.clear")}
            </button>
            <button class="btn-primary btn-send" disabled={selectedCount === 0} on:click={doSend}>
              {t("files.send")}{#if selectedCount > 0} ({selectedCount}){/if}
            </button>
          </div>
        </div>
      {/if}
      </div>

      <div class="chat" class:chat-expanded={chatExpanded}>
        <button
          type="button"
          class="chat-head"
          on:click={() => {
            chatExpanded = !chatExpanded;
            if (chatExpanded) scrollChatSoon();
          }}
        >
          <span>{t("chat.title")}</span>
          {#if chatBadge > 0}<span class="chat-count">{chatBadge}</span>{/if}
          <span class="chat-chevron"><ChevronIcon expanded={chatExpanded} size={22} /></span>
        </button>
        {#if chatExpanded}
          <div class="chat-list scroll-y" bind:this={chatListEl}>
            {#if chatMessages.length === 0}
              <div class="chat-empty">{t("chat.empty")}</div>
            {:else}
              {#each chatMessages as m, i (i)}
                <div class="chat-msg {m.dir === 'out' ? 'chat-out' : 'chat-in'}">
                  <div class="chat-bubble">
                    <span class="chat-text">{m.text}</span>
                    <button class="chat-copy" title={t("chat.copy")} aria-label={t("chat.copy")} on:click={() => copyText(m.text)}>
                      <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path fill="currentColor" d="M8 2h10a2 2 0 012 2v14h-2V4H8V2zm-4 4h10a2 2 0 012 2v14a2 2 0 01-2 2H6a2 2 0 01-2-2V8a2 2 0 012-2z"/></svg>
                    </button>
                  </div>
                  <div class="chat-time">
                    {fmtTime(m.ts)}{#if m.dir === "out"} <span class="chat-tick {m.read ? 'read' : ''}" title={m.read ? t("chat.read") : t("chat.delivered")}>{"\u2713\u2713"}</span>{/if}
                  </div>
                </div>
              {/each}
            {/if}
          </div>
          {#if chatError}<div class="chat-error">{chatError}</div>{/if}
          {#if copyFeedback}<div class="chat-toast">{copyFeedback}</div>{/if}
          <form class="chat-input" on:submit|preventDefault={sendMsg}>
            <input type="text" maxlength="8192" placeholder={t("chat.placeholder")} bind:value={chatInput} aria-label={t("chat.messageAria")} />
            <button class="btn-primary chat-send" type="submit" disabled={chatSending || !chatInput.trim()} aria-label={t("chat.sendAria")}>
              <svg class="chat-send-icon" viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
                <path fill="currentColor" d="M5 4.8v14.4L20.2 12z" />
              </svg>
            </button>
          </form>
        {/if}
      </div>
    </section>
  {/if}

  {#if started && downloadsPath}
    <footer class="save-footer" title={downloadsPath}>
      <span class="save-label">{t("footer.downloads")}</span>
      <button class="save-path-btn" on:click={revealDownloads}>{downloadsPath}</button>
    </footer>
  {/if}
</main>

{#if !started}
  <div class="modal-overlay">
    <div class="modal modal-wide">
      <h3>{t("iface.title")}</h3>
      <p class="modal-sub">{t("iface.sub")}</p>
      <div class="iface-list scroll-y">
        <button class="iface {chosenIface === '' ? 'iface-active' : ''}" on:click={() => saveIfaceChoice("")}>
          <span class="iface-radio" aria-hidden="true"></span>
          <NetIcon kind="auto" />
          <div class="iface-meta">
            <div class="iface-name">{t("iface.auto")}</div>
            <div class="iface-sub">{t("iface.autoSub")}</div>
          </div>
        </button>
        {#each interfaces as it (it.name)}
          <button class="iface {chosenIface === it.name ? 'iface-active' : ''}" on:click={() => saveIfaceChoice(it.name)}>
            <span class="iface-radio" aria-hidden="true"></span>
            <NetIcon kind={it.kind} />
            <div class="iface-meta">
              <div class="iface-name-row">
                <div class="iface-name">{ifaceLabel(it)}</div>
                {#if it.recommended}
                  <span class="iface-rec">{t("iface.recommended")}</span>
                {/if}
              </div>
              <div class="iface-sub">
                {#if it.displayName && it.displayName !== it.name}
                  <span class="iface-device">{it.name}</span>
                {/if}
                <span class="iface-ip">{it.addresses.join(", ")}</span>
                {#if fmtSpeedMbps(it.speedMbps)}<span class="iface-speed">{fmtSpeedMbps(it.speedMbps)}</span>{/if}
              </div>
            </div>
          </button>
        {/each}
      </div>
      {#if startError}
        <div class="banner banner-failed">{startError}</div>
      {/if}
      <div class="modal-actions">
        <button class="btn-primary" disabled={starting} on:click={() => startWith(chosenIface)}>
          {starting ? t("iface.starting") : t("iface.continue")}
        </button>
      </div>
    </div>
  </div>
{/if}

{#if incoming}
  <div class="modal-overlay" role="presentation">
    <div class="modal modal-incoming" role="dialog">
      {#if !recvState || recvState.state === "waiting"}
        <div class="modal-device-icon">
          <PlatformIcon platform={incoming.sender.platform} size={56} />
        </div>
        <h3>{t("transfer.incomingTitle", { name: incoming.sender.name })}</h3>
        <p class="modal-sub">{peerEndpoint(incoming.sender)}</p>
        <p class="modal-info">
          {t("transfer.incomingSummary", { count: incoming.count, size: fmtBytes(incoming.totalSize) })}
          {#if incoming.looseFiles > 0}{t("transfer.looseFiles", { loose: incoming.looseFiles })}{/if}
        </p>
        {#if incoming.rootDirs && incoming.rootDirs.length > 0}
          <p class="modal-sub root-names">
            {folderCountLabel(incoming.rootDirs.length)}
            {incoming.rootDirs.map((d) => d.name).join(", ")}
          </p>
        {/if}
        {#if incomingPreview.length > 0}
          <ul class="file-preview scroll-y">
            {#each incomingPreview as f (f.relPath + f.name)}
              <li><span class="file-preview-name">{previewName(f)}</span><span class="file-preview-size">{fmtBytes(f.size)}</span></li>
            {/each}
            {#if incomingMore > 0}
              <li class="file-preview-more">{t("transfer.andMore", { n: incomingMore })}</li>
            {/if}
          </ul>
        {/if}
        <div class="modal-actions">
          <button class="btn-ghost" on:click={decline}>{t("transfer.decline")}</button>
          <button class="btn-primary" on:click={accept}>{t("transfer.accept")}</button>
        </div>
      {:else if recvState.state === "transferring"}
        <h3>{t("transfer.incomingFrom", { name: incoming.sender.name })}</h3>
        <TransferPanel
          title=""
          indeterminate={false}
          percent={recvPct}
          largePercent
          bytesLabel="{recvProgress ? fmtBytes(recvProgress.bytes) : '0 B'} / {fmtBytes(incoming.totalSize)}"
          speedLabel={recvProgress ? fmtSpeed(recvProgress.speed) : "\u2014"}
          etaLabel={t("transfer.etaRemaining", { eta: recvProgress ? fmtETA(recvProgress.etaSeconds) : "\u2014" })}
          showAdvanced={!!recvProgress}
          advancedText={recvProgress ? t("transfer.streams", { streams: recvProgress.streams, file: "\u2014", pct: recvPct.toFixed(1) }) : ""}
        />
        <div class="modal-actions">
          <button class="btn-danger" on:click={cancelRecv}>{t("transfer.cancel")}</button>
        </div>
      {:else}
        <div class="modal-result" class:success={recvState.state === "completed"} class:warn={recvState.state !== "completed" && recvState.state !== "canceled"}>
          <svg viewBox="0 0 24 24" width="40" height="40" aria-hidden="true">
            {#if recvState.state === "completed"}
              <path fill="currentColor" d="M12 2a10 10 0 100 20 10 10 0 000-20zm-1.2 14.2l-4.2-4.2 1.4-1.4 2.8 2.8 5.8-5.8 1.4 1.4-7.2 7.2z"/>
            {:else}
              <path fill="currentColor" d="M12 2a10 10 0 100 20 10 10 0 000-20zm1 5v6h-2V7h2zm0 8v2h-2v-2h2z"/>
            {/if}
          </svg>
        </div>
        <h3>
          {#if recvState.state === "completed"}{t("transfer.filesReceived")}
          {:else if recvState.state === "canceled"}{t("transfer.canceled")}
          {:else}{t("transfer.incomplete")}{/if}
        </h3>
        <p class="modal-sub">{localizeError(recvState.message)}</p>
        <div class="modal-actions">
          {#if recvState.state === "completed"}
            <button class="btn-secondary" on:click={revealDownloads}>{t("transfer.openFolder")}</button>
          {/if}
          <button class="btn-primary" on:click={dismissIncoming}>{t("common.close")}</button>
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .app-main {
    position: relative;
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100%;
    max-width: 100%;
    min-width: 0;
    padding: var(--space-5) var(--space-6) var(--space-3);
    overflow: hidden;
  }
  .app-main.window-drop {
    --wails-drop-target: drop;
  }
  .app-main.window-drop.drop-active,
  .app-main.window-drop:global(.wails-drop-target-active) {
    outline: 2px dashed var(--color-accent);
    outline-offset: -4px;
  }
  .drop-overlay {
    position: absolute;
    inset: 0;
    z-index: 50;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(15, 20, 27, 0.55);
    pointer-events: none;
  }
  .drop-overlay-text {
    font-size: var(--text-lg);
    font-weight: 600;
    color: var(--color-accent-muted);
    padding: var(--space-4) var(--space-6);
    border: 2px dashed var(--color-accent);
    border-radius: var(--radius-lg);
    background: var(--color-surface-raised);
  }

  @keyframes view-in {
    from { opacity: 0; transform: translateY(6px); }
    to { opacity: 1; transform: translateY(0); }
  }

  .app-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-5);
    gap: var(--space-4);
    width: 100%;
    min-width: 0;
    max-width: 100%;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-width: 0;
    flex: 1 1 auto;
  }
  .brand-text { min-width: 0; }
  .brand h1 { font-size: var(--text-xl); margin: 0; font-weight: 700; line-height: 1.2; }
  .brand-status { margin: 2px 0 0; font-size: var(--text-sm); color: var(--color-text-muted); }
  .header-end {
    display: flex;
    align-items: center;
    flex: 0 1 auto;
    min-width: 0;
    max-width: 100%;
  }
  .header-self {
    display: flex;
    align-items: stretch;
    gap: var(--space-2);
    min-width: 0;
    max-width: 100%;
  }
  .btn-surface {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-surface);
    color: var(--color-text-secondary);
    cursor: pointer;
    transition: border-color var(--transition-fast), color var(--transition-fast), background var(--transition-fast);
  }
  .btn-surface:hover {
    border-color: var(--color-accent);
    color: var(--color-accent);
    background: var(--color-surface-raised);
  }
  .devices-header {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .devices-header .section-title {
    margin: 0;
  }
  .invite-feedback {
    margin: var(--space-2) 0 var(--space-3);
    font-size: var(--text-sm);
    color: var(--color-success-text);
  }
  .btn-back,
  .btn-with-icon {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: 12px 18px;
    font-size: var(--text-base);
    font-weight: 600;
    flex-shrink: 0;
  }
  .self-card {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 10px 16px;
    min-width: 0;
    max-width: 100%;
  }
  .self-meta { min-width: 0; }
  .self-name {
    font-size: var(--text-base);
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .self-sub {
    font-size: var(--text-sm);
    color: var(--color-text-muted);
    margin-top: 2px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .devices { flex: 1; min-height: 0; }
  .view-panel {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    margin-bottom: var(--space-3);
    animation: view-in var(--transition-normal);
  }
  .section-title {
    font-size: var(--text-md); font-weight: 600; color: var(--color-text-secondary);
    display: flex; align-items: center; gap: var(--space-3); margin: 0;
  }
  .count {
    background: var(--color-surface-inset); color: var(--color-accent-muted);
    border-radius: 999px; padding: 2px 12px; font-size: var(--text-sm); font-weight: 600;
  }
  .empty {
    width: 100%;
    text-align: center;
    margin-top: var(--space-6);
    color: var(--color-text-faint);
  }
  .empty-foreground {
    width: 100%;
    padding-top: var(--space-5);
  }
  .empty-art {
    display: flex;
    justify-content: center;
    align-items: center;
    width: 100%;
    margin-bottom: var(--space-4);
  }
  .empty p {
    margin: 0 auto var(--space-2);
    max-width: 420px;
    font-size: var(--text-lg);
    color: var(--color-text-secondary);
  }
  .empty small {
    display: block;
    margin-inline: auto;
    max-width: 420px;
    font-size: var(--text-sm);
    line-height: 1.5;
  }
  .empty-qr-panel {
    display: flex;
    flex-direction: column;
    align-items: center;
    margin-top: var(--space-7);
    padding-bottom: var(--space-4);
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 200px));
    justify-content: start;
    gap: var(--space-4);
    margin-top: var(--space-5);
  }
  .device {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-btn);
    padding: var(--space-5) var(--space-4);
    text-align: center;
    cursor: pointer;
    color: inherit;
    transition: border-color var(--transition-fast), background var(--transition-fast), box-shadow var(--transition-fast);
    overflow: hidden;
  }
  .device > :not(.device-drop-hint) {
    position: relative;
    z-index: 1;
  }
  .device.device-drop-target {
    border-color: var(--color-accent);
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--color-accent) 45%, transparent), 0 8px 28px rgba(0, 0, 0, 0.22);
  }
  .device.device-drop-target::before {
    content: "";
    position: absolute;
    inset: 0;
    background: color-mix(in srgb, var(--color-accent) 14%, var(--color-surface));
    backdrop-filter: blur(10px);
    -webkit-backdrop-filter: blur(10px);
    border-radius: inherit;
    pointer-events: none;
    z-index: 0;
  }
  .device-drop-hint {
    position: absolute;
    left: 50%;
    bottom: 10px;
    transform: translateX(-50%);
    z-index: 2;
    max-width: calc(100% - 16px);
    padding: 2px 8px;
    border-radius: 999px;
    background: color-mix(in srgb, var(--color-accent) 88%, #000);
    color: #fff;
    font-size: var(--text-xs);
    font-weight: 600;
    line-height: 1.35;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    pointer-events: none;
  }
  .device.device-drop-target .device-host {
    opacity: 0.55;
  }
  .badge {
    position: absolute; top: 10px; right: 10px; min-width: 22px; height: 22px; padding: 0 6px;
    border-radius: 11px; background: var(--color-accent); color: #fff;
    font-size: var(--text-xs); font-weight: 700; line-height: 22px;
    z-index: 2;
  }
  .badge-transfer {
    background: var(--color-success);
    font-size: 14px;
    line-height: 22px;
    animation: badge-pulse 1.4s ease-in-out infinite;
  }
  @keyframes badge-pulse {
    0%, 100% { transform: scale(1); }
    50% { transform: scale(1.08); }
  }
  .device:hover { border-color: var(--color-accent); background: var(--color-surface-raised); box-shadow: 0 4px 16px rgba(0,0,0,.18); }
  .device.device-drop-target:hover { background: var(--color-surface); }
  .device-name {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    font-size: var(--text-md);
    font-weight: 600;
    margin-top: var(--space-3);
  }
  .device-online {
    width: 9px;
    height: 9px;
    border-radius: 50%;
    background: var(--color-success);
    box-shadow: 0 0 8px rgba(61, 220, 132, 0.55);
    flex-shrink: 0;
  }
  .device-host {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    color: var(--color-text-muted);
    margin-top: var(--space-1);
  }

  .save-footer {
    flex-shrink: 0;
    margin-top: auto;
    padding: var(--space-2) 0 var(--space-1);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-1);
    text-align: center;
    font-size: var(--text-sm);
    color: var(--color-text-faint);
  }
  .save-path-btn {
    background: none; border: none; padding: 0; cursor: pointer;
    font-family: var(--font-mono); font-size: var(--text-sm); color: var(--color-accent-muted);
    max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }
  .save-path-btn:hover { text-decoration: underline; }

  .card-view {
    flex: 1; overflow: hidden; min-height: 0;
    display: flex; flex-direction: column; gap: var(--space-3);
  }
  .device-top {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-shrink: 0;
  }
  .device-top-meta { display: flex; align-items: center; gap: var(--space-3); min-width: 0; flex: 1; }
  .device-top-name { font-size: var(--text-md); font-weight: 600; }
  .device-top-sub { font-size: var(--text-sm); color: var(--color-text-faint); }

  .transfer-stack {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .transfer-area {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    align-content: start;
  }
  .transfer-scroll {
    flex: 1;
    min-height: 0;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    background: var(--color-surface);
  }
  .transfer-scroll-empty {
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .files-empty {
    text-align: center;
    padding: var(--space-6);
    color: var(--color-text-faint);
  }
  .files-empty p {
    margin: 0;
    font-size: var(--text-md);
    color: var(--color-text-secondary);
  }
  .files-empty-hint {
    margin-top: var(--space-2) !important;
    font-size: var(--text-sm) !important;
    color: var(--color-text-faint) !important;
  }
  .send-bar {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    padding: var(--space-3) var(--space-4);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
  }
  .send-bar-summary { flex: 1; min-width: 0; font-size: var(--text-base); color: var(--color-text-secondary); }
  .send-bar-empty { color: var(--color-text-faint); }

  .send-bar-actions {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-shrink: 0;
  }
  .btn-clear { min-width: 112px; padding: 12px 18px; }
  .btn-send { min-width: 168px; padding: 12px 22px; }

  .staged { padding: var(--space-3) var(--space-4); }
  .staged-head { font-size: var(--text-sm); color: var(--color-text-secondary); margin-bottom: var(--space-2); }
  .staged-list { display: flex; flex-direction: column; gap: var(--space-1); }
  .staged-item {
    display: flex; align-items: center; gap: var(--space-2);
    background: var(--color-surface-inset); border-radius: var(--radius-sm);
    padding: 8px 12px; min-height: 40px;
  }
  .staged-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: var(--text-sm); }
  .staged-size { font-size: var(--text-xs); color: var(--color-text-muted); font-family: var(--font-mono); flex-shrink: 0; }
  .tree-toggle {
    display: inline-flex; align-items: center; justify-content: center;
    background: none; border: none; color: var(--color-accent-muted); cursor: pointer;
    width: 28px; height: 28px; padding: 0; flex-shrink: 0;
  }
  .tree-toggle:hover { color: var(--color-text); }
  .tree-spacer { width: 28px; flex-shrink: 0; }
  .tree-check { flex-shrink: 0; accent-color: var(--color-accent); width: 16px; height: 16px; }
  .tree-icon { font-size: 16px; flex-shrink: 0; }
  .root-names { word-break: break-word; }

  .btn-primary,
  .btn-secondary,
  .btn-danger,
  .btn-ghost {
    border-radius: var(--radius-btn);
    padding: 12px 18px;
    font-size: var(--text-base);
    font-weight: 600;
    cursor: pointer;
    transition: background var(--transition-fast), border-color var(--transition-fast), color var(--transition-fast);
  }
  .btn-primary {
    background: var(--color-accent); color: #fff; border: none;
  }
  .btn-primary:hover:not(:disabled) { background: var(--color-accent-hover); }
  .btn-primary:disabled { background: var(--color-surface-inset); color: var(--color-text-faint); cursor: not-allowed; }
  .btn-secondary {
    background: var(--color-surface-inset); color: var(--color-text-secondary);
    border: 1px solid var(--color-border); font-weight: 600;
  }
  .btn-secondary:hover:not(:disabled) { border-color: var(--color-accent); color: var(--color-text); }
  .btn-secondary:disabled {
    color: var(--color-text-faint);
    border-color: var(--color-border);
    cursor: not-allowed;
  }
  .btn-danger {
    background: var(--color-danger-bg); color: var(--color-danger);
    border: 1px solid var(--color-danger-border); font-weight: 600;
  }
  .btn-ghost {
    background: transparent; color: var(--color-text-secondary);
    border: 1px solid var(--color-border); font-weight: 600;
  }
  .btn-ghost:hover { background: var(--color-surface-inset); }
  .link { background: none; border: none; color: var(--color-accent-muted); cursor: pointer; font-size: var(--text-sm); padding: var(--space-2); }
  .panel-actions { display: flex; justify-content: space-between; align-items: center; margin-top: var(--space-4); }

  .transfer-float {
    display: flex; align-items: center; gap: var(--space-4);
    background: var(--color-surface); border: 1px solid var(--color-border);
    border-radius: var(--radius-md); padding: var(--space-3) var(--space-4); margin-bottom: var(--space-4);
  }
  .transfer-float :global(.transfer-panel) { flex: 1; min-width: 0; }

  .banner {
    width: 100%;
    margin-top: var(--space-2);
    padding: 12px 16px;
    border-radius: var(--radius-md);
    font-size: var(--text-sm);
  }
  .banner-completed { background: var(--color-success-bg); color: var(--color-success-text); }
  .banner-declined, .banner-failed { background: var(--color-danger-bg); color: var(--color-danger); }
  .banner-canceled { background: var(--color-warn-bg); color: var(--color-text-secondary); }
  .banner-hint { color: var(--color-text-faint); }

  .chat {
    flex-shrink: 0; margin-top: auto; display: flex; flex-direction: column;
    background: var(--color-surface); border: 1px solid var(--color-border);
    border-radius: var(--radius-lg); padding: var(--space-3) var(--space-4); gap: var(--space-3);
  }
  .chat-head {
    display: flex; align-items: center; gap: var(--space-2); width: 100%;
    background: none; border: none; padding: var(--space-1) 0; cursor: pointer;
    font-size: var(--text-md); font-weight: 600; color: var(--color-text-secondary); text-align: left;
  }
  .chat-head:hover { color: var(--color-text); }
  .chat-count { background: var(--color-accent); color: #fff; border-radius: 10px; padding: 0 8px; font-size: var(--text-xs); line-height: 22px; font-weight: 700; }
  .chat-chevron {
    margin-left: auto;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: var(--color-accent-muted);
  }
  .chat-expanded { padding-bottom: var(--space-3); }
  .chat-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    max-height: min(300px, 36vh);
    min-height: 0;
  }
  .chat-empty { color: var(--color-text-faint); font-size: var(--text-sm); text-align: center; padding: var(--space-4) 0; }
  .chat-msg { display: flex; flex-direction: column; max-width: 85%; }
  .chat-in { align-self: flex-start; align-items: flex-start; }
  .chat-out { align-self: flex-end; align-items: flex-end; }
  .chat-bubble {
    display: flex; align-items: flex-start; gap: var(--space-2);
    background: var(--color-surface-inset); border: 1px solid var(--color-border);
    border-radius: var(--radius-md); padding: 10px 12px;
  }
  .chat-out .chat-bubble { background: #1d2c44; border-color: #2a3f5f; }
  .chat-text { white-space: pre-wrap; word-break: break-word; overflow-wrap: anywhere; font-size: var(--text-base); }
  .chat-copy { background: none; border: none; color: var(--color-text-faint); cursor: pointer; padding: 0; flex-shrink: 0; }
  .chat-copy:hover { color: var(--color-accent-muted); }
  .chat-time { font-size: var(--text-xs); color: var(--color-text-faint); margin-top: var(--space-1); }
  .chat-tick { color: var(--color-text-faint); font-size: var(--text-xs); letter-spacing: -1px; }
  .chat-tick.read { color: var(--color-accent); }
  .chat-error { font-size: var(--text-sm); color: var(--color-danger); }
  .chat-toast { font-size: var(--text-sm); color: var(--color-success-text); text-align: center; }
  .chat-input { display: flex; gap: var(--space-2); }
  .chat-input input {
    flex: 1; background: var(--color-surface-inset); border: 1px solid var(--color-border);
    border-radius: var(--radius-btn); padding: 11px 14px; color: var(--color-text);
    font-size: var(--text-base); font-family: inherit;
  }
  .chat-send {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 0 16px;
    min-width: 52px;
    min-height: 44px;
  }
  .chat-send-icon {
    display: block;
    margin-left: 2px;
  }

  .modal-overlay {
    position: fixed; inset: 0; background: rgba(8, 11, 16, 0.72);
    display: flex; align-items: center; justify-content: center; z-index: 100;
    animation: overlay-in var(--transition-normal);
  }
  @keyframes overlay-in { from { opacity: 0; } to { opacity: 1; } }
  .modal {
    background: var(--color-surface); border: 1px solid var(--color-border-strong);
    border-radius: var(--radius-xl); padding: var(--space-6); width: min(400px, 92vw);
    max-height: min(90vh, 720px);
    display: flex; flex-direction: column; overflow: hidden;
    text-align: center; animation: modal-in var(--transition-normal);
  }
  @keyframes modal-in { from { opacity: 0; transform: scale(0.97); } to { opacity: 1; transform: scale(1); } }
  .modal h3 { margin: var(--space-3) 0 var(--space-2); font-size: var(--text-lg); }
  .modal-sub { font-size: var(--text-sm); color: var(--color-text-muted); margin: var(--space-1) 0; }
  .modal-info { font-size: var(--text-base); color: var(--color-accent-muted); margin: var(--space-3) 0; }
  .modal-actions { display: flex; gap: var(--space-3); justify-content: center; margin-top: var(--space-5); flex-shrink: 0; }
  .modal-actions button { flex: 1; font-size: var(--text-base); }
  .modal-wide { width: min(480px, 94vw); text-align: left; }
  .modal-wide .iface-list { flex: 1 1 auto; min-height: 0; }
  .modal-wide h3 { text-align: left; margin-top: 0; font-size: var(--text-xl); }
  .modal-incoming { width: min(440px, 94vw); }
  .modal-device-icon { display: flex; justify-content: center; margin-bottom: var(--space-2); }
  .modal-result { display: flex; justify-content: center; margin-bottom: var(--space-2); color: var(--color-text-muted); }
  .modal-result.success { color: var(--color-success); }
  .modal-result.warn { color: var(--color-danger); }
  .file-preview {
    list-style: none; margin: var(--space-3) 0; padding: 0; max-height: 160px;
    min-height: 0; flex-shrink: 1;
    text-align: left; border: 1px solid var(--color-border); border-radius: var(--radius-md);
    background: var(--color-surface-inset);
  }
  .file-preview li {
    display: flex; justify-content: space-between; gap: var(--space-3);
    padding: 10px 14px; font-size: var(--text-sm); border-bottom: 1px solid var(--color-border);
  }
  .file-preview li:last-child { border-bottom: none; }
  .file-preview-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .file-preview-size { font-family: var(--font-mono); color: var(--color-text-muted); flex-shrink: 0; }
  .file-preview-more { color: var(--color-text-faint); justify-content: center; }

  .settings-path {
    display: block; width: 100%; text-align: left; background: var(--color-surface-inset);
    border: 1px solid var(--color-border); border-radius: var(--radius-btn);
    padding: 12px 14px; font-family: var(--font-mono); font-size: var(--text-sm);
    color: var(--color-accent-muted); cursor: pointer; word-break: break-all;
  }
  .settings-path:hover { border-color: var(--color-accent); }

  .iface-list {
    display: flex; flex-direction: column; gap: var(--space-2);
    margin: var(--space-4) 0 var(--space-2); max-height: 320px; min-height: 0;
  }
  .iface {
    display: flex; align-items: center; gap: var(--space-3);
    background: var(--color-surface-inset); border: 1px solid var(--color-border);
    border-radius: var(--radius-btn); padding: 12px 14px; cursor: pointer; color: inherit; text-align: left;
    transition: border-color var(--transition-fast), background var(--transition-fast);
  }
  .iface:hover { border-color: var(--color-accent); }
  .iface-active { border-color: var(--color-accent); background: var(--color-surface-raised); }
  .iface-radio {
    width: 18px; height: 18px; border-radius: 50%; border: 2px solid var(--color-text-muted); flex-shrink: 0;
  }
  .iface-active .iface-radio {
    border-color: var(--color-accent); box-shadow: inset 0 0 0 4px var(--color-accent);
  }
  .iface-meta { flex: 1; min-width: 0; }
  .iface-name-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }
  .iface-name { font-size: var(--text-base); font-weight: 600; }
  .iface-rec {
    font-size: var(--text-xs);
    font-weight: 600;
    color: var(--color-success-text);
    background: var(--color-success-bg);
    border-radius: 999px;
    padding: 2px 8px;
    line-height: 1.35;
  }
  .iface-sub { font-size: var(--text-sm); color: var(--color-text-muted); margin-top: 2px; display: flex; gap: var(--space-2); flex-wrap: wrap; }
  .iface-device { font-family: var(--font-mono); color: var(--color-text-faint); }
  .iface-ip { font-family: var(--font-mono); color: var(--color-accent-muted); }
  .iface-speed { color: var(--color-success-text); }
</style>
