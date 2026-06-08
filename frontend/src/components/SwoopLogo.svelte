<script lang="ts">
  import appIcon from "../../../build/appicon.png";

  /** Fixed px size; omit to fill parent (e.g. header brand-logo stretch). */
  export let size: number | undefined = undefined;
</script>

<img
  class="swoop-logo"
  src={appIcon}
  alt=""
  width={size}
  height={size}
  aria-hidden="true"
/>

<style>
  .swoop-logo {
    display: block;
    flex-shrink: 0;
    border-radius: 22%;
    object-fit: contain;
  }
</style>
