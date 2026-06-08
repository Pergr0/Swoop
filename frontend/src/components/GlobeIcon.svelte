<script lang="ts">
  /** Globe glyph — same design as net/other.svg, uses currentColor for toolbar buttons. */
  /** Omit to size via parent CSS (e.g. header toolbar). */
  export let size: number | undefined = undefined;
</script>

<svg class="globe-icon" viewBox="0 0 32 32" width={size} height={size} aria-hidden="true">
  <circle cx="16" cy="16" r="11" fill="none" stroke="currentColor" stroke-width="2" />
  <ellipse cx="16" cy="16" rx="5" ry="11" fill="none" stroke="currentColor" stroke-width="1.6" />
  <path fill="none" stroke="currentColor" stroke-width="1.6" d="M5 16h22M7.5 11h17M7.5 21h17" />
</svg>

<style>
  .globe-icon {
    display: block;
    flex-shrink: 0;
  }
</style>
