<script lang="ts">
  export let value = 0;
  export let indeterminate = false;
</script>

<div class="bar" class:bar-indeterminate={indeterminate} role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={indeterminate ? undefined : Math.round(value)}>
  <div class="bar-fill" style={indeterminate ? "" : `width:${value}%`}></div>
</div>

<style>
  .bar {
    height: 10px;
    background: var(--color-surface-inset);
    border-radius: 6px;
    overflow: hidden;
  }
  .bar-fill {
    height: 100%;
    background: linear-gradient(90deg, var(--color-accent), var(--color-success));
    transition: width 0.2s ease;
  }
  .bar-indeterminate .bar-fill {
    width: 35% !important;
    animation: indeterminate 1.2s ease-in-out infinite;
  }
  @keyframes indeterminate {
    0% {
      transform: translateX(-120%);
    }
    100% {
      transform: translateX(320%);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .bar-indeterminate .bar-fill {
      width: 50% !important;
      animation: none;
      opacity: 0.7;
    }
  }
</style>
