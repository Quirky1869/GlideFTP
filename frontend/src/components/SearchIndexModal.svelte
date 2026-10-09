<script>
  // Explains the search index option (internal/searchindex).
  // mode 'intro': shown once on the first launch on this device - Enable / No thanks.
  // mode 'info': re-opened from the "?" next to the option in Settings > Recherche - Close.
  import { onMount } from 'svelte';
  import { t } from '../i18n/index.js';
  import { trapFocus } from '../utils/focusTrap.js';
  import { GetSearchIndexInfo } from '../../wailsjs/go/main/App.js';

  export let mode = 'intro';
  export let onEnable = () => {};
  export let onDecline = () => {};
  export let onClose = () => {};

  let dir = '';
  onMount(async () => {
    try { dir = (await GetSearchIndexInfo())?.dir || ''; } catch {}
  });

  function onKeydown(e) {
    if (e.key === 'Escape') (mode === 'intro' ? onDecline : onClose)();
  }
</script>

<div class="si-overlay">
  <div class="si-box" use:trapFocus on:keydown={onKeydown} role="dialog">
    <div class="si-title">{$t('searchIndexIntroTitle')}</div>
    <ul class="si-points">
      <li>{$t('searchIndexIntroWhat')}</li>
      <li>{$t('searchIndexIntroFaster')}</li>
      <li>{$t('searchIndexIntroVerified')}</li>
      <li>{$t('searchIndexIntroPrivacy')}</li>
      <li>{$t('searchIndexIntroWhere')}<br /><code class="si-path">{dir}</code></li>
      <li class="si-strong">{$t('searchIndexIntroSettings')}</li>
    </ul>
    <div class="si-actions">
      {#if mode === 'intro'}
        <button class="btn-primary" on:click={onEnable} autofocus>{$t('searchIndexEnable')}</button>
        <button class="btn-secondary" on:click={onDecline}>{$t('searchIndexDecline')}</button>
      {:else}
        <button class="btn-primary" on:click={onClose} autofocus>{$t('close')}</button>
      {/if}
    </div>
  </div>
</div>

<style>
.si-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.65);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 4900; /* above Settings (400) and ColorPicker (500), under NotifyModal (5000) */
}

.si-box {
  background: var(--bg-secondary);
  border: 2px solid var(--accent);
  border-radius: 12px;
  padding: 24px 26px 20px;
  width: 520px;
  max-width: 90vw;
  max-height: 85vh;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 14px;
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.5), 0 0 0 4px var(--accent-subtle);
}

.si-title { font-size: 17px; font-weight: 700; color: var(--text-primary); text-align: center; }

.si-points {
  margin: 0;
  padding-left: 20px;
  display: flex;
  flex-direction: column;
  gap: 9px;
  font-size: 13px;
  line-height: 1.45;
  color: var(--text-secondary);
}
/* "You can turn it off / clear the cache in Settings" - the point to remember */
.si-strong { font-weight: 700; color: var(--text-primary); }
.si-path {
  display: inline-block;
  margin-top: 3px;
  font-family: monospace;
  font-size: 12px;
  color: var(--text-primary);
  word-break: break-all;
}

.si-actions { display: flex; justify-content: center; gap: 8px; margin-top: 4px; }

.btn-primary, .btn-secondary {
  border-radius: 6px;
  padding: 8px 18px;
  font-size: 13px;
  cursor: pointer;
}
.btn-primary { background: var(--accent-bg); border: none; color: white; font-weight: 500; }
.btn-primary:hover { background: var(--accent-hover-bg); }
.btn-secondary { background: var(--bg-button); border: 1px solid var(--border); color: var(--text-secondary); }
.btn-secondary:hover { background: var(--bg-button-hover); color: var(--text-primary); }

/* ── Accent gradient mode (CLAUDE.md accent rules) ── */
:global(html[data-accent-gradient]) .si-box {
  border-color: transparent;
  background: linear-gradient(var(--bg-secondary), var(--bg-secondary)) padding-box, var(--accent-gradient) border-box;
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.5), -3px 0 0 4px var(--accent-subtle), 3px 0 0 4px var(--accent-subtle-2);
}
</style>
