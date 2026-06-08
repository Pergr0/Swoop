<script lang="ts">
  import { t } from "../i18n";
  import InviteQrCode from "./InviteQrCode.svelte";

  export interface InviteBundle {
    blob: string;
    shortCode: string;
    expiresAt: number;
    deviceName: string;
  }

  export let open = false;
  export let bundle: InviteBundle | null = null;
  export let loading = false;
  export let onClose: () => void = () => {};
  export let onDownload: () => void = () => {};

  function backdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) onClose();
  }
</script>

{#if open}
  <div class="modal-overlay" role="presentation" on:click={backdropClick}>
    <div class="modal modal-invite" role="dialog" aria-labelledby="invite-title" aria-modal="true">
      <button type="button" class="modal-close" aria-label={t("invite.closeAria")} on:click={onClose}>
        <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
          <path fill="currentColor" d="M18.3 5.7 12 12l6.3 6.3-1.4 1.4L10.6 13.4 4.3 19.7 2.9 18.3 9.2 12 2.9 5.7 4.3 4.3l6.3 6.3 6.3-6.3z"/>
        </svg>
      </button>
      <h3 id="invite-title">{t("invite.modalTitle")}</h3>
      <p class="modal-sub">{t("invite.modalHint")}</p>
      <div class="invite-qr-wrap">
        {#if loading}
          <div class="invite-qr-loading" aria-busy="true" aria-label={t("invite.loading")}></div>
        {:else if bundle?.blob}
          <InviteQrCode blob={bundle.blob} alt={t("invite.qrAlt")} />
        {/if}
      </div>
      {#if bundle?.blob && !loading}
        <button type="button" class="btn-download btn-secondary" on:click={onDownload}>
          {t("invite.download")}
        </button>
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
    margin: 0 0 var(--space-4);
    line-height: 1.45;
    max-width: 32ch;
    margin-left: auto;
    margin-right: auto;
  }
  .modal-invite {
    width: min(520px, 96vw);
  }
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
  .invite-qr-wrap {
    display: flex;
    justify-content: center;
    min-height: 480px;
    margin-bottom: var(--space-4);
  }
  .invite-qr-loading {
    width: 480px;
    height: 480px;
    max-width: 100%;
    border-radius: var(--radius-md);
    background: var(--color-surface-inset);
    animation: pulse 1.2s ease-in-out infinite;
  }
  @keyframes pulse {
    0%, 100% { opacity: 0.55; }
    50% { opacity: 1; }
  }
  .btn-download {
    align-self: center;
    min-width: 140px;
    padding: 12px 24px;
    font-size: var(--text-base);
    font-weight: 600;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-surface);
    color: var(--color-text-secondary);
    cursor: pointer;
    transition: border-color var(--transition-fast), color var(--transition-fast), background var(--transition-fast);
  }
  .btn-download:hover {
    border-color: var(--color-accent);
    color: var(--color-accent);
    background: var(--color-surface-raised);
  }
</style>
