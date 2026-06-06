<script lang="ts">
  export let expanded = false;
  export let size = 22;
</script>

<svg
  class="chevron-icon"
  width={size}
  height={size}
  viewBox="0 0 24 24"
  aria-hidden="true"
  focusable="false"
>
  {#if expanded}
    <path fill="currentColor" d="M6.3 9.3 12 15l5.7-5.7 1.4 1.4L12 17.8 4.9 10.7z" />
  {:else}
    <path fill="currentColor" d="M9.3 6.3 15 12l-5.7 5.7-1.4-1.4L12 10.6 7.9 6.5z" />
  {/if}
</svg>

<style>
  .chevron-icon {
    display: block;
    flex-shrink: 0;
  }
</style>
