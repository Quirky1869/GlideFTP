<script>
  // Shows the window size ("1400 × 900") in the middle of the window while it
  // is being resized, then fades out - handy to pick the values for the
  // Settings > Window width/height fields. The size comes from Wails'
  // WindowGetSize(), i.e. the same units main.go uses for options.Width/Height,
  // not the webview's innerWidth/innerHeight.
  import { onMount, onDestroy } from 'svelte';
  import { WindowGetSize } from '../../wailsjs/runtime/runtime.js';
  import { settings } from '../stores/settings.js';

  const HIDE_DELAY_MS = 1200;
  // Startup (window creation, "Open maximized") fires resize events too -
  // ignore them so the badge doesn't flash when the app opens.
  const STARTUP_GRACE_MS = 1500;

  let size = null;   // { w, h }
  let visible = false;
  let ready = false;
  let hideTimer = null;
  let readyTimer = null;
  let pending = false;

  onMount(() => {
    readyTimer = setTimeout(() => ready = true, STARTUP_GRACE_MS);
  });

  onDestroy(() => {
    clearTimeout(hideTimer);
    clearTimeout(readyTimer);
  });

  // Resize events fire many times per second: one WindowGetSize call per frame max.
  function onResize() {
    if (!ready || pending) return;
    pending = true;
    requestAnimationFrame(async () => {
      try {
        const s = await WindowGetSize();
        size = { w: s.w, h: s.h };
        visible = true;
        clearTimeout(hideTimer);
        hideTimer = setTimeout(() => visible = false, HIDE_DELAY_MS);
      } catch {}
      pending = false;
    });
  }
</script>

<svelte:window on:resize={onResize} />

{#if size}
  <!-- Accent border always; accent glow only with Settings > "Ombre
       d'accentuation" (connectCardShadow), like the connect card -->
  <div class="size-badge" class:visible class:glow={$settings?.connectCardShadow} aria-hidden="true">
    {size.w} <span class="times">×</span> {size.h}
  </div>
{/if}

<style>
.size-badge {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%) scale(0.96);
  z-index: 4999;
  pointer-events: none;
  padding: 12px 22px;
  border-radius: 10px;
  border: 2px solid var(--accent);
  background: var(--bg-secondary);
  box-shadow: 0 8px 32px rgba(0,0,0,0.35);
  color: var(--text-primary);
  font-family: monospace;
  font-size: 26px;
  font-weight: 700;
  letter-spacing: 0.02em;
  opacity: 0;
  transition: opacity 0.25s ease, transform 0.25s ease;
}
.size-badge.visible {
  opacity: 1;
  transform: translate(-50%, -50%) scale(1);
  transition: opacity 0.08s ease, transform 0.08s ease;
}
.times { color: var(--accent); }

/* ── Accent gradient mode ── */
:global(html[data-accent-gradient]) .times {
  background: var(--accent-gradient);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
.size-badge.glow {
  box-shadow: 0 0 24px var(--accent-glow, rgba(0,0,0,0.3)), 0 8px 32px rgba(0,0,0,0.35);
}

:global(html[data-accent-gradient]) .size-badge {
  border-color: transparent;
  background: linear-gradient(var(--bg-secondary), var(--bg-secondary)) padding-box, var(--accent-gradient) border-box;
}
:global(html[data-accent-gradient]) .size-badge.glow {
  box-shadow: -10px 0 24px var(--accent-glow), 10px 0 24px var(--accent-glow-2), 0 8px 32px rgba(0,0,0,0.35);
}
</style>
