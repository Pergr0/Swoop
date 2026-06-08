<script lang="ts">
  import QRCode from "qrcode";

  export let url = "";
  export let size = 280;
  export let alt = "";
  /** Softer contrast for inline/ambient display; still scannable. */
  export let muted = false;

  let dataUrl = "";

  $: if (url) {
    QRCode.toDataURL(url, {
      width: size,
      margin: 2,
      errorCorrectionLevel: muted ? "H" : "M",
      color: muted
        ? { dark: "#6f8298ff", light: "#0f141bff" }
        : { dark: "#e8edf5ff", light: "#171d27ff" },
    })
      .then((d) => (dataUrl = d))
      .catch(() => (dataUrl = ""));
  } else {
    dataUrl = "";
  }
</script>

{#if dataUrl}
  <img class="qr-image" class:qr-image-muted={muted} src={dataUrl} width={size} height={size} {alt} />
{:else if url}
  <div class="qr-placeholder" style="width: {size}px; height: {size}px" aria-hidden="true"></div>
{/if}

<style>
  .qr-image {
    display: block;
    border-radius: var(--radius-md);
    background: var(--color-surface-inset);
  }
  .qr-image-muted {
    background: transparent;
  }
  .qr-placeholder {
    border-radius: var(--radius-md);
    background: var(--color-surface-inset);
  }
</style>
