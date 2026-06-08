<script lang="ts">
  /** Square toolbar button (header): height follows .self-card via flex stretch. */
  export let title = "";
  export let ariaLabel = "";

  /** Width = laid-out height so the button stays square in flex row layout. */
  function squareFromHeight(node: HTMLButtonElement) {
    const apply = () => {
      const h = node.getBoundingClientRect().height;
      if (h > 0) node.style.width = `${h}px`;
    };
    const ro = new ResizeObserver(apply);
    ro.observe(node);
    apply();
    return { destroy: () => ro.disconnect() };
  }
</script>

<button
  type="button"
  class="btn-toolbar btn-surface"
  {title}
  aria-label={ariaLabel}
  use:squareFromHeight
  on:click
>
  <slot />
</button>

<style>
  .btn-toolbar {
    display: flex;
    align-items: center;
    justify-content: center;
    box-sizing: border-box;
    flex-shrink: 0;
    align-self: stretch;
    width: auto;
    min-width: 0;
    padding: 0;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-surface);
    color: var(--color-text-secondary);
    cursor: pointer;
    transition: border-color var(--transition-fast), color var(--transition-fast), background var(--transition-fast);
  }
  .btn-toolbar:hover {
    border-color: var(--color-accent);
    color: var(--color-accent);
    background: var(--color-surface-raised);
  }
  .btn-toolbar :global(svg),
  .btn-toolbar :global(.globe-icon),
  .btn-toolbar :global(.qr-glyph) {
    width: 55%;
    height: 55%;
    max-width: 36px;
    max-height: 36px;
    display: block;
  }
</style>
