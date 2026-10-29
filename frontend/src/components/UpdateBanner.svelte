<script>
  // v1.8.0 #125 - "new version available" bar. Mounted in App.svelte's
  // #app-root flex column between .main-content and TransferQueue: it sits at
  // the bottom of the window, right above the queue when that is open, and
  // the Settings / Sites overlays (position: fixed) cover it.
  import { t } from '../i18n/index.js';
  import { updateInfo, skipUpdateVersion, openUpdateLink } from '../stores/update.js';
  import { notify } from '../stores/notify.js';

  let copied = false;
  let copiedTimer = null;

  // Download button label for each install kind (see installKind in app_update.go)
  const DOWNLOAD_LABELS = {
    exe: 'updateDownloadExe',
    appimage: 'updateDownloadAppImage',
    deb: 'updateDownloadDeb',
    rpm: 'updateDownloadRpm',
    linux: 'updateDownloadArchive',
  };

  async function open(url) {
    try {
      await openUpdateLink(url);
    } catch (e) {
      notify(e?.toString() || $t('unknownError'), 'error');
    }
  }

  async function copyCommand() {
    try {
      await navigator.clipboard.writeText($updateInfo.command);
      copied = true;
      clearTimeout(copiedTimer);
      copiedTimer = setTimeout(() => copied = false, 2000);
    } catch (e) {
      notify(e?.toString() || $t('unknownError'), 'error');
    }
  }
</script>

{#if $updateInfo}
  <div class="update-banner" role="status">
    <svg class="update-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
    <span class="update-text">
      {#if $updateInfo.preview}<span class="update-preview">{$t('updatePreviewTag')}</span>{/if}
      {@html $t('updateAvailable')
        .replace('{latest}', `<strong>v${$updateInfo.latest}</strong>`)
        .replace('{current}', `v${$updateInfo.current}`)}
    </span>

    <div class="update-actions">
      {#if $updateInfo.kind === 'aur'}
        <code class="update-cmd">{$updateInfo.command}</code>
        <button class="update-btn primary" on:click={copyCommand}>
          {copied ? $t('updateCopied') + ' ✓' : $t('updateCopyCommand')}
        </button>
      {:else if $updateInfo.downloadUrl}
        <button class="update-btn primary" on:click={() => open($updateInfo.downloadUrl)} title={$updateInfo.downloadName}>
          {$t(DOWNLOAD_LABELS[$updateInfo.kind] || 'updateDownloadArchive')}
        </button>
      {/if}
      <button class="update-btn" on:click={() => open($updateInfo.releaseUrl)}>{$t('updateSeeRelease')}</button>
      <button class="update-btn" on:click={() => skipUpdateVersion($updateInfo.latest)}>{$t('updateSkip')}</button>
      <button class="update-close" on:click={() => updateInfo.set(null)} title={$t('updateLater')} aria-label={$t('updateLater')}>✕</button>
    </div>
  </div>
{/if}

<style>
.update-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  flex-shrink: 0;
  padding: 10px 16px;
  background: linear-gradient(var(--bg-secondary), var(--bg-secondary)) padding-box,
              var(--accent-gradient) border-box;
  border-top: 2px solid transparent;
  color: var(--text-primary);
  font-size: 13px;
  animation: update-in 0.25s ease;
}
@keyframes update-in {
  from { transform: translateY(100%); opacity: 0; }
  to   { transform: translateY(0);    opacity: 1; }
}
.update-icon {
  width: 18px;
  height: 18px;
  color: var(--accent);
  flex-shrink: 0;
}
.update-text { flex: 1; min-width: 200px; }
.update-text :global(strong) { color: var(--accent); }
.update-preview {
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 1px 6px;
  margin-right: 8px;
  font-size: 11px;
  color: var(--text-muted);
  text-transform: uppercase;
}
.update-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.update-cmd {
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 4px 8px;
  font-size: 12px;
  user-select: all;
}
.update-btn {
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-secondary);
  padding: 5px 12px;
  font-size: 13px;
  cursor: pointer;
  white-space: nowrap;
}
.update-btn:hover { background: var(--bg-button-hover); color: var(--text-primary); }
.update-btn.primary {
  background: var(--accent-bg) border-box;
  border-color: transparent;
  color: white;
}
.update-btn.primary:hover { background: var(--accent-hover-bg) border-box; }
.update-close {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 15px;
  line-height: 1;
  padding: 0 4px;
}
.update-close:hover { color: var(--text-primary); }

:global(html[data-accent-gradient]) .update-text :global(strong) {
  background: var(--accent-gradient);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
</style>
