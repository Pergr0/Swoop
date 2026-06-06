<script lang="ts">
  import QRCode from "qrcode";

  export let open = false;
  export let url = "";
  export let onClose: () => void = () => {};

  let dataUrl = "";

  $: if (open && url) {
    QRCode.toDataURL(url, {
      width: 280,
      margin: 2,
      errorCorrectionLevel: "M",
      color: { dark: "#e8edf5ff", light: "#171d27ff" },
    })
      .then((d) => (dataUrl = d))
      .catch(() => (dataUrl = ""));
  } else if (!open) {
    dataUrl = "";
  }

  function backdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) onClose();
  }
</script>

{#if open}
  <div class="modal-overlay" role="presentation" on:click={backdropClick}>
    <div class="modal modal-qr" role="dialog" aria-labelledby="qr-title" aria-modal="true">
      <button type="button" class="modal-close" aria-label="Закрыть" on:click={onClose}>
        <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
          <path fill="currentColor" d="M18.3 5.7 12 12l6.3 6.3-1.4 1.4L10.6 13.4 4.3 19.7 2.9 18.3 9.2 12 2.9 5.7 4.3 4.3l6.3 6.3 6.3-6.3z"/>
        </svg>
      </button>
      <h3 id="qr-title">Отправка с телефона</h3>
      <p class="modal-sub">Отсканируйте QR-код камерой телефона (та же Wi‑Fi сеть)</p>
      {#if dataUrl}
        <img class="qr-image" src={dataUrl} width="280" height="280" alt="QR-код со ссылкой на отправку файлов" />
      {:else}
        <div class="qr-placeholder" aria-hidden="true"></div>
      {/if}
      {#if url}
        <p class="qr-url">{url}</p>
      {/if}
    </div>
  </div>
{/if}

<style>
  .modal-overlay {
    position: fixed;
    inset: 0;
    background: rgba(8, 11, 16, 0.72);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
    animation: overlay-in var(--transition-normal);
  }
  @keyframes overlay-in {
    from { opacity: 0; }
    to { opacity: 1; }
  }
  .modal {
    position: relative;
    background: var(--color-surface);
    border: 1px solid var(--color-border-strong);
    border-radius: var(--radius-xl);
    padding: var(--space-6);
    max-height: min(90vh, 720px);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    text-align: center;
    animation: modal-in var(--transition-normal);
  }
  @keyframes modal-in {
    from { opacity: 0; transform: scale(0.97); }
    to { opacity: 1; transform: scale(1); }
  }
  .modal h3 {
    margin: var(--space-3) 0 var(--space-2);
    font-size: var(--text-lg);
  }
  .modal-sub {
    font-size: var(--text-sm);
    color: var(--color-text-muted);
    margin: var(--space-1) 0;
  }
  .modal-qr { width: min(360px, 92vw); }
  .modal-close {
    position: absolute;
    top: 12px;
    right: 12px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    padding: 0;
    border: none;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--color-text-muted);
    cursor: pointer;
  }
  .modal-close:hover {
    background: var(--color-surface-inset);
    color: var(--color-text);
  }
  .qr-image {
    display: block;
    margin: var(--space-4) auto 0;
    border-radius: var(--radius-md);
    background: var(--color-surface-inset);
  }
  .qr-placeholder {
    width: 280px;
    height: 280px;
    margin: var(--space-4) auto 0;
    border-radius: var(--radius-md);
    background: var(--color-surface-inset);
  }
  .qr-url {
    margin: var(--space-3) 0 0;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--color-text-muted);
    word-break: break-all;
    line-height: 1.45;
  }
</style>
