<script>
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { t } from './i18n/index.js';
  import { EventsOn } from '../wailsjs/runtime/runtime.js';
  import { loadSettings, settings } from './stores/settings.js';
  import {
    connectionStatus,
    connections, activeConnectionId, switchTab, closeTab,
    localPath, localEntries, localSelected,
    remotePath, remoteEntries, remoteSelected,
    initLocalDir, refreshLocal, navigateLocalUp,
    refreshRemote,
    localMkDir, localDelete, localRename, localCopy, localSearch,
    remoteMkDir, remoteDelete, remoteRename, remoteSearch,
    disconnect,
  } from './stores/connection.js';
  import { transfers, queueVisible, initTransfers, completedTransfer } from './stores/transfers.js';
  import { trapFocus } from './utils/focusTrap.js';
  import ConnectionBar from './components/ConnectionBar.svelte';
  import FileBrowser from './components/FileBrowser.svelte';
  import TransferQueue from './components/TransferQueue.svelte';
  import SettingsPanel from './components/SettingsPanel.svelte';
  import SiteManager from './components/SiteManager.svelte';
  import NotifyModal from './components/NotifyModal.svelte';
  import { notification, closeNotify } from './stores/notify.js';

  let showSettings = false;
  let showSiteManager = false;
  let showDisconnectConfirm = false;
  let lostNotif = null; // { host } when a connection is dropped by the server
  let lastDefaultLocalDir = ''; // tracks the setting so a change can be applied live

  // Resizable split pane
  let leftWidth = 50; // percent
  let resizing = false;
  let dualBrowserEl;

  $: isConnected = $connectionStatus === 'connected';
  $: pendingCount = $transfers.filter(j => j.status === 'pending' || j.status === 'running').length;

  onMount(async () => {
    const s = await loadSettings();
    lastDefaultLocalDir = s?.defaultLocalDir || '';
    await initLocalDir(lastDefaultLocalDir);
    await initTransfers();
    EventsOn('connection:lost', ({ id, host }) => {
      closeTab(id);
      lostNotif = { host };
      setTimeout(() => { lostNotif = null; }, 10000);
    });
  });

  // Auto-refresh both panels when a transfer completes
  $: if ($completedTransfer) {
    if (get(connectionStatus) === 'connected') {
      refreshLocal(get(localPath));
      refreshRemote(get(remotePath));
    }
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') {
      if ($notification) { closeNotify(); return; }
      showSettings = false;
      showSiteManager = false;
      return;
    }
    // WebKit-GTK doesn't fire native undo in bound inputs - force it
    if ((e.ctrlKey || e.metaKey) && e.key === 'z') {
      const el = document.activeElement;
      if (el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA')) {
        e.preventDefault();
        document.execCommand('undo');
        el.dispatchEvent(new Event('input', { bubbles: true }));
      }
    }
  }

  // Settings saved - refresh file lists so showHiddenFiles takes effect
  async function handleSettingsSaved(newSettings) {
    // Default local directory changed - navigate there immediately instead of
    // waiting for the next app launch to pick it up.
    const newDefaultDir = newSettings?.defaultLocalDir || '';
    if (newDefaultDir !== lastDefaultLocalDir) {
      lastDefaultLocalDir = newDefaultDir;
      await initLocalDir(newDefaultDir);
    } else {
      refreshLocal(get(localPath));
    }
    if (isConnected) {
      refreshRemote(get(remotePath));
    }
  }

  // ── Resizable split pane ──────────────────────────────────────────────────

  function startResize(e) {
    resizing = true;
    e.preventDefault();
    const onMove = (ev) => {
      if (!dualBrowserEl) return;
      const rect = dualBrowserEl.getBoundingClientRect();
      leftWidth = Math.max(20, Math.min(80, ((ev.clientX - rect.left) / rect.width) * 100));
    };
    const onUp = () => {
      resizing = false;
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<div id="app-root">
  <!-- ── Connection lost banner ────────────────────────────────── -->
  {#if lostNotif}
    <div class="conn-lost-banner">
      <span>⚠ {$t('connectionLost')} - {$t('connectionLostDetail').replace('{host}', lostNotif.host)}</span>
      <button class="conn-lost-close" on:click={() => lostNotif = null}>✕</button>
    </div>
  {/if}
  <!-- ── Topbar ──────────────────────────────────────────────────── -->
  <div class="topbar">
    <div class="topbar-left">
      <span class="app-logo">GlideFTP</span>
    </div>
    <div class="topbar-center">
      {#if isConnected}
        <ConnectionBar onMultiDisconnect={() => showDisconnectConfirm = true} />
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

  <!-- ── Connection tabs (visible only when 2+ connections open) ── -->
  {#if $connections.length > 1}
    <div class="conn-tabs">
      {#each $connections as conn (conn.id)}
        <div
          class="conn-tab"
          class:active={conn.id === $activeConnectionId}
          on:click={() => switchTab(conn.id)}
          title={conn.host}
        >
          <span class="conn-tab-name">{conn.name || conn.host}</span>
          <button
            class="conn-tab-close"
            on:click|stopPropagation={() => closeTab(conn.id)}
            title="Fermer"
          >✕</button>
        </div>
      {/each}
    </div>
  {/if}

  <!-- ── Main content ───────────────────────────────────────────── -->
  <div class="main-content">

    {#if !isConnected}
      <!-- ── Disconnected: centered connection form ─────────────── -->
      <div class="connect-center">
        <div class="connect-card" class:shadow-accent={$settings?.connectCardShadow}>
          <h1 class="connect-title">GlideFTP</h1>
          <p class="connect-subtitle">{$t('connectFirst')}</p>
          <div class="connect-bar-wrap">
            <ConnectionBar />
          </div>
          <p class="connect-sites-hint">
            <button class="link-btn" on:click={() => showSiteManager = true}>{$t('manageSites')}</button>
          </p>
        </div>
      </div>

    {:else}
      <!-- ── Connected: dual file browser ──────────────────────── -->
      <div class="dual-browser" bind:this={dualBrowserEl}>
        <div class="browser-panel" style="width: {leftWidth}%">
          <FileBrowser
            side="local"
            path={$localPath}
            entries={$localEntries}
            selected={$localSelected}
            otherPath={$remotePath}
            otherEntries={$remoteEntries}
            onNavigate={refreshLocal}
            onNavigateUp={() => navigateLocalUp($localPath)}
            onRefresh={() => refreshLocal($localPath)}
            onMkDir={localMkDir}
            onDelete={localDelete}
            onRename={localRename}
            onSearch={localSearch}
          />
        </div>
        <div
          class="browser-splitter"
          class:active={resizing}
          on:mousedown={startResize}
          title="Drag to resize"
        ></div>
        <div class="browser-panel" style="flex: 1; min-width: 0;">
          <FileBrowser
            side="remote"
            path={$remotePath}
            entries={$remoteEntries}
            selected={$remoteSelected}
            otherPath={$localPath}
            otherEntries={$localEntries}
            onNavigate={refreshRemote}
            onNavigateUp={async () => {
              const parts = $remotePath.split('/').filter(Boolean);
              parts.pop();
              await refreshRemote('/' + parts.join('/') || '/');
            }}
            onRefresh={() => refreshRemote($remotePath)}
            onMkDir={remoteMkDir}
            onDelete={remoteDelete}
            onRename={remoteRename}
            onSearch={remoteSearch}
          />
        </div>
      </div>
    {/if}

  </div>

  <!-- ── Transfer queue ─────────────────────────────────────────── -->
  {#if $queueVisible}
    <TransferQueue />
  {/if}

  <!-- ── Overlays ───────────────────────────────────────────────── -->
  {#if showSettings}
    <SettingsPanel onClose={() => showSettings = false} onSaved={handleSettingsSaved} />
  {/if}

  {#if showSiteManager}
    <SiteManager onClose={() => showSiteManager = false} />
  {/if}

  <NotifyModal />

  <!-- ── Disconnect-all confirmation ───────────────────────────── -->
  {#if showDisconnectConfirm}
    <div class="dc-overlay" on:click|self={() => showDisconnectConfirm = false}>
      <div class="dc-box" use:trapFocus>
        <div class="dc-title">{$t('multiDisconnectTitle')}</div>
        <div class="dc-msg">{$connections.length} {$t('multiDisconnectMsg')}</div>
        <div class="dc-actions">
          <button class="btn-danger-full" on:click={async () => { showDisconnectConfirm = false; await disconnect(); }}>
            {$t('disconnectAll')}
          </button>
          <button class="btn-cancel" on:click={() => showDisconnectConfirm = false}>
            {$t('cancel')}
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
#app-root {
  height: 100vh;
  width: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* ── Connection lost banner ──────────────────────────────────────── */
.conn-lost-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 16px;
  background: #7a2020;
  color: #ffd0d0;
  font-size: 13px;
  flex-shrink: 0;
  animation: slide-down 0.2s ease;
}
@keyframes slide-down {
  from { transform: translateY(-100%); opacity: 0; }
  to   { transform: translateY(0);     opacity: 1; }
}
.conn-lost-close {
  background: none;
  border: none;
  color: #ffd0d0;
  cursor: pointer;
  font-size: 16px;
  line-height: 1;
  padding: 0 4px;
  flex-shrink: 0;
}
.conn-lost-close:hover { color: #fff; }

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
  overflow: hidden;
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
  overflow: hidden;
}

/* Override ConnectionBar styles when inside topbar */
.topbar-center :global(.conn-bar) {
  background: transparent;
  border-bottom: none;
  padding: 0;
}
/* Hide labels in topbar to keep it single-row */
.topbar-center :global(.field-group label) {
  display: none;
}
.topbar-center :global(.conn-fields) {
  align-items: center;
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
  white-space: nowrap;
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

.queue-btn.has-activity { color: var(--accent); }

/* ── Connection tabs ─────────────────────────────────────────────── */
.conn-tabs {
  display: flex;
  align-items: stretch;
  height: 36px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border);
  overflow-x: auto;
  overflow-y: hidden;
  flex-shrink: 0;
  scrollbar-width: none;
}
.conn-tabs::-webkit-scrollbar { display: none; }

.conn-tab {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 160px;
  max-width: 240px;
  padding: 0 10px 0 14px;
  border-right: 1px solid var(--border);
  cursor: pointer;
  background: var(--bg-primary);
  color: var(--text-muted);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  transition: background 0.1s, color 0.1s;
  user-select: none;
  flex-shrink: 0;
}
.conn-tab:hover { background: var(--bg-hover); color: var(--text-primary); }
.conn-tab.active {
  background: var(--bg-secondary);
  color: var(--text-primary);
  border-bottom: 2px solid var(--accent);
}

.conn-tab-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
}

.conn-tab-close {
  flex-shrink: 0;
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 10px;
  padding: 2px 3px;
  border-radius: 3px;
  line-height: 1;
  transition: background 0.1s, color 0.1s;
}
.conn-tab-close:hover { background: var(--danger); color: white; }

/* ── Disconnect-all confirmation overlay ─────────────────────────── */
.dc-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 600;
}

.dc-box {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 24px 28px;
  width: 340px;
  max-width: 95vw;
  display: flex;
  flex-direction: column;
  gap: 12px;
  box-shadow: 0 16px 48px rgba(0,0,0,0.5);
}

.dc-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.dc-msg {
  font-size: 13px;
  color: var(--text-secondary);
}

.dc-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  margin-top: 4px;
}

.btn-danger-full {
  background: var(--danger);
  border: none;
  border-radius: 5px;
  color: white;
  padding: 7px 16px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.12s;
}
.btn-danger-full:hover { filter: brightness(1.1); }

.btn-cancel {
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-secondary);
  padding: 7px 16px;
  font-size: 13px;
  cursor: pointer;
}
.btn-cancel:hover { background: var(--bg-button-hover); }

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
  padding: 32px 36px 24px;
  width: 100%;
  max-width: 860px;
  box-shadow: 0 8px 32px rgba(0,0,0,0.3);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  transition: box-shadow 0.25s ease;
}

.connect-card.shadow-accent {
  box-shadow:
    0 8px 32px rgba(0,0,0,0.3),
    4px 4px 8px var(--accent-glow),
    8px 8px 20px var(--accent-glow),
    14px 14px 45px var(--accent-subtle);
}

.connect-title {
  font-size: 24px;
  font-weight: 700;
  color: var(--text-primary);
}

.connect-subtitle {
  font-size: 13px;
  color: var(--text-muted);
  margin-bottom: 4px;
}

/* ConnectionBar inside the card: remove bar styling */
.connect-bar-wrap {
  width: 100%;
}

.connect-bar-wrap :global(.conn-bar) {
  background: transparent;
  border-bottom: none;
  padding: 0;
}

.connect-bar-wrap :global(.conn-fields) {
  flex-wrap: nowrap;
  gap: 10px;
}

.connect-sites-hint {
  font-size: 12px;
  color: var(--text-muted);
}

.link-btn {
  background: none;
  border: none;
  color: var(--accent);
  cursor: pointer;
  font-size: 12px;
  padding: 0;
  text-decoration: underline;
}

/* ── Dual file browser ───────────────────────────────────────────── */
.dual-browser {
  flex: 1;
  display: flex;
  overflow: hidden;
  min-height: 0;
}

.browser-panel {
  overflow: hidden;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.browser-splitter {
  width: 5px;
  flex-shrink: 0;
  cursor: col-resize;
  background: var(--border);
  transition: background 0.15s;
  user-select: none;
}

.browser-splitter:hover, .browser-splitter.active {
  background: var(--accent);
}
</style>
