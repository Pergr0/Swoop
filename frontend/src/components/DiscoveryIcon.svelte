<script lang="ts">
  export let size = 56;

  const cx = 32;
  const cy = 33;
  const period = 2.8;
  const fadeSpan = 42; // % of cycle to fade out after the sweep passes

  const blips: { r: number; deg: number }[] = [
    { r: 9, deg: -38 },
    { r: 9, deg: 142 },
    { r: 18, deg: 18 },
    { r: 18, deg: -128 },
    { r: 26, deg: -62 },
    { r: 26, deg: 108 },
  ];

  const rings = [9, 18, 26];

  function blipXY(r: number, deg: number) {
    const rad = (deg * Math.PI) / 180;
    return { x: cx + r * Math.cos(rad), y: cy + r * Math.sin(rad) };
  }

  // Sweep starts at 12 o'clock; pass moment as % of one rotation.
  function passPct(deg: number) {
    return (((deg + 90) % 360) + 360) % 360 / 3.6;
  }

  function blipKeyframeRule(i: number, deg: number) {
    const pass = passPct(deg);
    const pre = Math.max(0, pass - 0.35);
    const fadeEnd = Math.min(99.9, pass + fadeSpan);
    return `@keyframes blip-${i} {
      0%, ${pre.toFixed(2)}% { opacity: 0; transform: scale(0.85); }
      ${pass.toFixed(2)}% { opacity: 1; transform: scale(1.15); }
      ${fadeEnd.toFixed(2)}% { opacity: 0; transform: scale(0.85); }
      100% { opacity: 0; transform: scale(0.85); }
    }`;
  }

  $: blipCss = blips.map((b, i) => blipKeyframeRule(i, b.deg)).join("\n");
</script>

{@html `<style>${blipCss}</style>`}

<svg
  class="discovery-icon"
  width={size}
  height={size}
  viewBox="0 0 64 64"
  aria-hidden="true"
  focusable="false"
>
  <defs>
    <linearGradient id="sweep-trail" x1="0" y1="0" x2="1" y2="0">
      <stop offset="0%" stop-color="currentColor" stop-opacity="0.45" />
      <stop offset="100%" stop-color="currentColor" stop-opacity="0" />
    </linearGradient>
  </defs>

  <circle cx={cx} cy={cy} r="28" fill="currentColor" opacity="0.08" />
  <circle cx={cx} cy={cy} r="28" fill="none" stroke="currentColor" stroke-width="1.4" opacity="0.45" />

  {#each rings as r}
    <circle cx={cx} cy={cy} {r} fill="none" stroke="currentColor" stroke-width="1" opacity="0.28" />
  {/each}

  <line x1={cx} y1={cy - 26} x2={cx} y2={cy + 26} stroke="currentColor" stroke-width="0.8" opacity="0.18" />
  <line x1={cx - 26} y1={cy} x2={cx + 26} y2={cy} stroke="currentColor" stroke-width="0.8" opacity="0.18" />

  <g transform="translate({cx} {cy})">
    <g class="sweep">
      <path d="M0 0 L0 -26 A26 26 0 0 1 6.8 -25.1 Z" fill="url(#sweep-trail)" />
      <line x1="0" y1="0" x2="0" y2="-26" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
    </g>
  </g>

  {#each blips as b, i}
    {@const p = blipXY(b.r, b.deg)}
    <circle
      class="blip"
      cx={p.x}
      cy={p.y}
      r="2.2"
      fill="currentColor"
      style="animation: blip-{i} {period}s linear infinite"
    />
  {/each}

  <circle cx={cx} cy={cy} r="2" fill="currentColor" opacity="0.7" />
</svg>

<style>
  .discovery-icon {
    display: block;
    flex-shrink: 0;
    color: var(--color-accent-muted);
  }

  .sweep {
    transform-origin: 0 0;
    animation: pelengator-sweep 2.8s linear infinite;
  }

  @keyframes pelengator-sweep {
    to {
      transform: rotate(360deg);
    }
  }

  .blip {
    opacity: 0;
    transform-box: fill-box;
    transform-origin: center;
  }

  @media (prefers-reduced-motion: reduce) {
    .sweep {
      animation: none;
      transform: rotate(-38deg);
    }
    .blip {
      animation: none !important;
      opacity: 0.5;
      transform: none;
    }
  }
</style>
