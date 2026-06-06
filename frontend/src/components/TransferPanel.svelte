<script lang="ts">
  import ProgressBar from "./ProgressBar.svelte";

  export let title = "";
  export let indeterminate = false;
  export let percent = 0;
  export let bytesLabel = "";
  export let speedLabel = "";
  export let etaLabel = "";
  export let showEta = true;
  export let showAdvanced = false;
  export let advancedText = "";
  export let largePercent = false;
  export let compact = false;
</script>

<div class="transfer-panel" class:compact>
  {#if title}<div class="transfer-title">{title}</div>{/if}
  {#if largePercent && !indeterminate}
    <div class="transfer-pct">{percent.toFixed(0)}%</div>
  {/if}
  <ProgressBar value={percent} {indeterminate} />
  <div class="transfer-stats">
    <span>{bytesLabel}</span>
    {#if !compact}
      <span>{speedLabel}</span>
      {#if showEta}<span>{etaLabel}</span>{/if}
    {:else}
      <span>{speedLabel}</span>
    {/if}
  </div>
  {#if showAdvanced && advancedText}
    <div class="transfer-advanced">{advancedText}</div>
  {/if}
  <slot name="actions" />
</div>

<style>
  .transfer-panel {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: var(--space-4);
  }
  .transfer-panel.compact {
    padding: var(--space-3) var(--space-4);
    border: none;
    background: transparent;
  }
  .transfer-title {
    font-size: var(--text-md);
    font-weight: 600;
    margin-bottom: var(--space-3);
    color: var(--color-text-secondary);
  }
  .compact .transfer-title {
    font-size: var(--text-sm);
    margin-bottom: var(--space-2);
  }
  .transfer-pct {
    font-size: var(--text-2xl);
    font-weight: 700;
    text-align: center;
    margin-bottom: var(--space-2);
    color: var(--color-text);
    font-variant-numeric: tabular-nums;
  }
  .transfer-stats {
    display: flex;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-2);
    font-size: var(--text-sm);
    color: var(--color-text-secondary);
    margin-top: var(--space-3);
    font-family: var(--font-mono);
  }
  .compact .transfer-stats {
    margin-top: var(--space-2);
    font-size: var(--text-xs);
  }
  .transfer-advanced {
    font-size: var(--text-xs);
    color: var(--color-text-faint);
    margin-top: var(--space-2);
  }
</style>
