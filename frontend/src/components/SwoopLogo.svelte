<script lang="ts">
  import appIcon from "../../../build/appicon.png";

  export let size = 34;
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
