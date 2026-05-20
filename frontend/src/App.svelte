<script>
  import { onMount } from 'svelte';
  import { t } from './i18n/index.js';
  import { loadSettings } from './stores/settings.js';
  import {
    connectionStatus,
    localPath, localEntries, localSelected,
    remotePath, remoteEntries, remoteSelected,
    initLocalDir, refreshLocal, navigateLocalUp,
    refreshRemote,
    localMkDir, localDelete, localRename,
    remoteMkDir, remoteDelete, remoteRename,
  } from './stores/connection.js';
  import { transfers, queueVisible, initTransfers } from './stores/transfers.js';
  import ConnectionBar from './components/ConnectionBar.svelte';
  import FileBrowser from './components/FileBrowser.svelte';
  import TransferQueue from './components/TransferQueue.svelte';
  import SettingsPanel from './components/SettingsPanel.svelte';
  import SiteManager from './components/SiteManager.svelte';

  let showSettings = false;
  let showSiteManager = false;

  $: isConnected = $connectionStatus === 'connected';
  $: pendingCount = $transfers.filter(j => j.status === 'pending' || j.status === 'running').length;

  onMount(async () => {
    await loadSettings();
    await initLocalDir();
    await initTransfers();
  });

  function handleKeydown(e) {
    if (e.key === 'Escape') {
      showSettings = false;
      showSiteManager = false;
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<div id="app-root">
  <!-- ── Topbar ──────────────────────────────────────────────────── -->
  <div class="topbar">
    <div class="topbar-left">
      <span class="app-logo">⚡ GlideFTP</span>
    </div>
    <div class="topbar-center">
      {#if isConnected}
        <ConnectionBar />
      {/if}
    </div>
    <div class="topbar-right">
      <button
        class="topbar-btn"
        class:active={showSiteManager}
        on:click={() => { showSiteManager = !showSiteManager; showSettings = false; }}
        title={$t('manageSites')}
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg>
        <span class="topbar-btn-label">{$t('manageSites')}</span>
      </button>
      <button
        class="topbar-btn queue-btn"
        class:has-activity={pendingCount > 0}
        on:click={() => queueVisible.update(v => !v)}
        title={$t('queue')}
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/></svg>
        <span class="topbar-btn-label">{$t('queue')}</span>
        {#if pendingCount > 0}
          <span class="topbar-badge">{pendingCount}</span>
        {/if}
      </button>
      <button
        class="topbar-btn"
        class:active={showSettings}
        on:click={() => { showSettings = !showSettings; showSiteManager = false; }}
        title={$t('settings')}
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
        <span class="topbar-btn-label">{$t('settings')}</span>
      </button>
    </div>
  </div>

  <!-- ── Main content ───────────────────────────────────────────── -->
  <div class="main-content">

    {#if !isConnected}
      <!-- ── Disconnected: centered connection form ─────────────── -->
      <div class="connect-center">
        <div class="connect-card">
          <div class="connect-logo">⚡</div>
          <h1 class="connect-title">GlideFTP</h1>
          <p class="connect-subtitle">{$t('connectFirst')}</p>
          <ConnectionBar />
        </div>
      </div>

    {:else}
      <!-- ── Connected: dual file browser ──────────────────────── -->
      <div class="dual-browser">
        <FileBrowser
          side="local"
          path={$localPath}
          entries={$localEntries}
          selected={$localSelected}
          otherPath={$remotePath}
          onNavigate={refreshLocal}
          onNavigateUp={() => navigateLocalUp($localPath)}
          onRefresh={() => refreshLocal($localPath)}
          onMkDir={localMkDir}
          onDelete={localDelete}
          onRename={localRename}
        />
        <div class="browser-divider"></div>
        <FileBrowser
          side="remote"
          path={$remotePath}
          entries={$remoteEntries}
          selected={$remoteSelected}
          otherPath={$localPath}
          onNavigate={refreshRemote}
          onNavigateUp={async () => {
            const parts = $remotePath.split('/').filter(Boolean);
            parts.pop();
            const parent = '/' + parts.join('/');
            await refreshRemote(parent || '/');
          }}
          onRefresh={() => refreshRemote($remotePath)}
          onMkDir={remoteMkDir}
          onDelete={remoteDelete}
          onRename={remoteRename}
        />
      </div>
    {/if}

  </div>

  <!-- ── Transfer queue ─────────────────────────────────────────── -->
  {#if $queueVisible}
    <TransferQueue />
  {/if}

  <!-- ── Overlays ───────────────────────────────────────────────── -->
  {#if showSettings}
    <SettingsPanel onClose={() => showSettings = false} />
  {/if}

  {#if showSiteManager}
    <SiteManager onClose={() => showSiteManager = false} />
  {/if}
</div>

<style>
#app-root {
  height: 100vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* ── Topbar ──────────────────────────────────────────────────────── */
.topbar {
  display: flex;
  align-items: center;
  height: 44px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border);
  padding: 0 12px;
  gap: 8px;
  flex-shrink: 0;
  z-index: 10;
}

.topbar-left {
  display: flex;
  align-items: center;
  flex-shrink: 0;
}

.topbar-center {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.app-logo {
  font-size: 15px;
  font-weight: 700;
  color: var(--accent);
  letter-spacing: -0.02em;
  white-space: nowrap;
}

.topbar-btn {
  display: flex;
  align-items: center;
  gap: 5px;
  background: transparent;
  border: none;
  border-radius: 5px;
  color: var(--text-muted);
  padding: 5px 8px;
  font-size: 12px;
  cursor: pointer;
  transition: background 0.12s, color 0.12s;
  position: relative;
}

.topbar-btn:hover, .topbar-btn.active {
  background: var(--bg-button-hover);
  color: var(--text-primary);
}

.topbar-btn svg {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
}

.topbar-btn-label {
  font-size: 12px;
  white-space: nowrap;
}

.topbar-badge {
  background: var(--accent);
  color: white;
  border-radius: 10px;
  font-size: 10px;
  font-weight: 700;
  padding: 1px 5px;
}

.queue-btn.has-activity {
  color: var(--accent);
}

/* ── Main content ────────────────────────────────────────────────── */
.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 0;
}

/* ── Centered connection screen ──────────────────────────────────── */
.connect-center {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.connect-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 32px;
  width: 100%;
  max-width: 680px;
  box-shadow: 0 8px 32px rgba(0,0,0,0.3);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.connect-logo {
  font-size: 48px;
  line-height: 1;
}

.connect-title {
  font-size: 24px;
  font-weight: 700;
  color: var(--text-primary);
}

.connect-subtitle {
  font-size: 13px;
  color: var(--text-muted);
  margin-bottom: 8px;
}

/* ── Dual file browser ───────────────────────────────────────────── */
.dual-browser {
  flex: 1;
  display: flex;
  overflow: hidden;
  min-height: 0;
}

.browser-divider {
  width: 1px;
  background: var(--border);
  flex-shrink: 0;
}
</style>
