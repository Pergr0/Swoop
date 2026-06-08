<script lang="ts">
  import QrCodeImage from "./QrCodeImage.svelte";

  export let blob = "";
  /** Large modules help phone cameras read the dense signed invite payload. */
  export let size = 480;
  export let alt = "";
</script>

<QrCodeImage url={blob} {size} {alt} ecLevel="M" margin={4} />
