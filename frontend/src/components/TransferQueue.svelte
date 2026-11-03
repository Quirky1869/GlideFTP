<script>
  import { t } from '../i18n/index.js';
  import { transfers, cancelTransfer, cancelAllTransfers, retryTransfer, abandonTransfer, clearTransfers, removeTransfer, formatBytes, progressPct, queueVisible, accountKey, resumeBannerDismissed } from '../stores/transfers.js';
  import { connections } from '../stores/connection.js';
  import { notify } from '../stores/notify.js';
  import { settings, saveSettings } from '../stores/settings.js';
  import { queueDoneAction, queueDoneOnce } from '../stores/queueDone.js';
  import { trapFocus } from '../utils/focusTrap.js';

  let activeTab = 'pending';
  let queueHeight = 220;
  let resizing = false;
  let startY = 0;
  let startHeight = 0;

  // ── Speed tracking ────────────────────────────────────────────────────────
  let prevBytes = {}; // id → { bytes, time }
  let speeds = {};    // id → bytes/sec

  $: {
    const now = Date.now();
    $transfers.forEach(job => {
      if (job.status === 'running') {
        const prev = prevBytes[job.id];
        if (prev && (now - prev.time) > 250) {
          const dt = (now - prev.time) / 1000;
          const db = job.bytesDone - prev.bytes;
          if (db >= 0 && dt > 0) speeds[job.id] = db / dt;
          prevBytes[job.id] = { bytes: job.bytesDone, time: now };
        } else if (!prev) {
          prevBytes[job.id] = { bytes: job.bytesDone, time: now };
          speeds[job.id] = speeds[job.id] || 0;
        }
      } else {
        delete prevBytes[job.id];
      }
    });
    speeds = speeds;
  }

  function formatSpeed(bps) {
    if (!bps || bps <= 0) return '';
    if (bps >= 1024 * 1024) return (bps / (1024 * 1024)).toFixed(1) + ' MB/s';
    if (bps >= 1024) return Math.round(bps / 1024) + ' KB/s';
    return Math.round(bps) + ' B/s';
  }

  function avgSpeed(job) {
    if (!job.finishedAt || !job.createdAt || !job.size) return '';
    const dur = (new Date(job.finishedAt) - new Date(job.createdAt)) / 1000;
    if (dur <= 0) return '';
    return formatSpeed(job.size / dur);
  }

  function dirLabel(job) {
    const host = job.remoteHost || '?';
    return job.direction === 'upload' ? `local → ${host} ` : `${host} → local `;
  }

  // connection_closed comes from the Go queue (Queue.ConnectionClosed).
  function jobError(job, t) {
    if (job.error === 'connection_closed') return t('transferConnClosed').replace('{host}', job.remoteHost || '?');
    return job.error;
  }

  // Accounts with an open connection: a paused / failed transfer can only be
  // resumed on its own server (it is bound again to that connection).
  $: openAccounts = new Set($connections.map(accountKey));
  const canRun = (job, accounts) => !job.account || accounts.has(job.account);

  async function runAgain(job, resume) {
    try {
      const wasPaused = job.status === 'paused';
      await retryTransfer(job.id, resume);
      // Resuming one paused transfer brings back the "Resume all" bar for the
      // others, even after "Later".
      if (wasPaused) resumeBannerDismissed.set(new Set());
    } catch (e) {
      const msg = e?.toString() || '';
      notify(msg.includes('not_connected') ? $t('transferNotConnected').replace('{host}', job.remoteHost || '?') : msg, 'error');
    }
  }

  async function abandon(job) {
    try {
      const note = await abandonTransfer(job.id);
      if (note === 'remote_kept') notify($t('transferRemoteKept').replace('{path}', job.remotePath));
    } catch (e) {
      notify(e?.toString() || $t('unknownError'), 'error');
    }
  }

  function startResize(e) {
    resizing = true;
    startY = e.clientY;
    startHeight = queueHeight;
    e.preventDefault();
    const onMove = (ev) => {
      queueHeight = Math.max(80, Math.min(500, startHeight - (ev.clientY - startY)));
    };
    const onUp = () => {
      resizing = false;
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
  }

  // Paused transfers (resume later, lost connection, app closed) stay in "Pending".
  $: pending = $transfers.filter(j => j.status === 'pending' || j.status === 'running' || j.status === 'paused');
  $: failed = $transfers.filter(j => j.status === 'failed' || j.status === 'cancelled');
  $: done = $transfers.filter(j => j.status === 'done');

  $: tabs = [
    { key: 'pending', label: $t('pending'), count: pending.length },
    { key: 'failed', label: $t('failed'), count: failed.length },
    { key: 'done', label: $t('success'), count: done.length },
  ];

  $: currentList = activeTab === 'pending' ? pending : activeTab === 'failed' ? failed : done;

  function statusClass(job) {
    return job.status;
  }

  // ── After the queue (right-click anywhere in the queue) ─────────────────────
  // One flat menu: bubble / sound (for this queue only, locked on when the
  // Settings turn them on for every queue), then one action.
  let qdMenu = null; // { x, y, up, left } while shown

  function openQdMenu(e) {
    e.preventDefault();
    const left = e.clientX > window.innerWidth - 330;
    const up = e.clientY > window.innerHeight / 2;
    qdMenu = {
      x: left ? window.innerWidth - e.clientX : e.clientX,
      y: up ? window.innerHeight - e.clientY : e.clientY,
      up, left,
    };
  }

  function closeQdMenu() {
    qdMenu = null;
  }

  function toggleOnce(key) {
    queueDoneOnce.update(o => ({ ...o, [key]: !o[key] }));
    closeQdMenu();
  }

  function chooseAction(action) {
    closeQdMenu();
    if (action === 'command') {
      cmdPrompt = { value: $settings?.queueDoneCommand || '' };
      return;
    }
    queueDoneAction.set(action);
  }

  $: bubbleLocked = !!$settings?.queueDoneBubble;
  $: soundLocked = !!$settings?.notificationSoundEnabled;
  $: actions = [
    { key: 'none', label: $t('qdActionNone') },
    { key: 'quit', label: $t('qdActionQuit') },
    { key: 'shutdown', label: $t('qdActionShutdown') },
    { key: 'command', label: $t('qdActionCommand') },
  ];

  // "Run a command...": the command line, pre-filled with the last one.
  let cmdPrompt = null; // { value }
  const isWindows = navigator.userAgent.includes('Windows');

  async function confirmCommand() {
    const value = cmdPrompt.value.trim();
    if (!value) return;
    cmdPrompt = null;
    if (value !== $settings?.queueDoneCommand) {
      try {
        await saveSettings({ ...$settings, queueDoneCommand: value });
      } catch (e) {
        notify(e?.toString() || $t('unknownError'), 'error');
        return;
      }
    }
    queueDoneAction.set('command');
  }

  function focusOnOpen(node) {
    requestAnimationFrame(() => { node.focus(); node.select?.(); });
  }

  $: badgeLabel = { quit: $t('qdBadgeQuit'), shutdown: $t('qdBadgeShutdown'), command: $t('qdBadgeCommand') }[$queueDoneAction];
</script>

<svelte:window on:click={closeQdMenu} on:keydown={(e) => { if (qdMenu && e.key === 'Escape') closeQdMenu(); }} />

<div class="queue-panel" style="height: {queueHeight}px;" on:contextmenu={openQdMenu}>
  <div class="queue-resize-handle" class:active={resizing} on:mousedown={startResize}></div>
  <div class="queue-header">
    <span class="queue-title">{$t('transferQueue')}</span>
    <div class="tabs">
      {#each tabs as tab}
        <button
          class="tab"
          class:active={activeTab === tab.key}
          on:click={() => activeTab = tab.key}
        >
          {tab.label}
          {#if tab.count > 0}<span class="badge">{tab.count}</span>{/if}
        </button>
      {/each}
    </div>
    <div class="queue-actions">
      {#if badgeLabel}
        <span class="qd-badge" class:danger={$queueDoneAction === 'shutdown'} title={$queueDoneAction === 'command' ? $settings?.queueDoneCommand : null}>
          {#if $queueDoneAction === 'shutdown'}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M18.36 6.64a9 9 0 1 1-12.73 0"/><line x1="12" y1="2" x2="12" y2="12"/></svg>
          {:else if $queueDoneAction === 'quit'}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>
          {:else}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>
          {/if}
          {badgeLabel}
          <button class="qd-badge-x" on:click={() => queueDoneAction.set('none')} title={$t('qdBadgeCancel')}>✕</button>
        </span>
      {/if}
      {#if activeTab === 'pending' && pending.length > 0}
        <button class="small-btn danger cancel-all" on:click={cancelAllTransfers} title={$t('cancelAllTransfersHint')}>
          {$t('cancelAllTransfers')}
        </button>
      {/if}
      {#if activeTab !== 'pending' && currentList.length > 0}
        <button class="small-btn" on:click={() => {
          if (activeTab === 'failed') { clearTransfers('failed'); clearTransfers('cancelled'); }
          else clearTransfers('done');
        }}>
          {$t('clear')}
        </button>
      {/if}
      <button class="close-btn" on:click={() => queueVisible.set(false)}>✕</button>
    </div>
  </div>

  <div class="queue-body">
    {#if currentList.length === 0}
      <div class="empty">{$t('noTransfers')}</div>
    {:else}
      {#each currentList as job (job.id)}
        <div class="job-row" class:running={job.status === 'running'}>
          <div class="job-info">
            <span class="job-direction">{job.direction === 'upload' ? '↑' : '↓'}</span>
            <span class="job-name" title="{job.localPath}">{job.name}</span>
            <span class="job-size">{formatBytes(job.size)}</span>
            <span class="job-status {statusClass(job)}">{job.status}</span>
          </div>
          <div class="job-route">{dirLabel(job)}{#if activeTab === 'done' && avgSpeed(job)}<span class="avg-speed">• {avgSpeed(job)} {$t('avgSuffix')}</span>{/if}</div>
          {#if job.status === 'running' || ((job.status === 'paused' || job.status === 'failed') && job.bytesDone > 0)}
            <div class="progress-bar" class:stopped={job.status !== 'running'}>
              <div class="progress-fill" style="width: {progressPct(job)}%"></div>
            </div>
            <span class="progress-label">
              {progressPct(job)}% - {formatBytes(job.bytesDone)} / {formatBytes(job.size)}
              {#if job.status === 'running' && speeds[job.id] > 0}<span class="speed-label"> • {formatSpeed(speeds[job.id])}</span>{/if}
            </span>
          {/if}
          {#if job.error}
            <div class="job-error">{jobError(job, $t)}</div>
          {/if}
          <div class="job-actions">
            {#if job.status === 'pending' || job.status === 'running'}
              <button class="small-btn danger" on:click={() => cancelTransfer(job.id)}>{$t('cancel')}</button>
            {/if}
            {#if job.status === 'paused' || job.status === 'failed'}
              {@const runnable = canRun(job, openAccounts)}
              {@const notHere = runnable ? '' : $t('transferNotConnected').replace('{host}', job.remoteHost || '?')}
              {#if job.status === 'paused' || job.bytesDone > 0}
                <button class="small-btn primary" disabled={!runnable} title={notHere || $t('resumeTransferHint')} on:click={() => runAgain(job, true)}>{$t('resumeTransfer')}</button>
              {/if}
              {#if job.status === 'failed'}
                <button class="small-btn" disabled={!runnable} title={notHere || $t('restartTransferHint')} on:click={() => runAgain(job, false)}>{$t('restartTransfer')}</button>
              {/if}
              <button class="small-btn danger" title={$t('abandonTransferHint')} on:click={() => abandon(job)}>{$t('abandonTransfer')}</button>
            {/if}
            {#if job.status === 'done' || job.status === 'cancelled'}
              <button class="remove-btn" on:click={() => removeTransfer(job.id)} title="Supprimer">×</button>
            {/if}
          </div>
        </div>
      {/each}
    {/if}
  </div>
</div>

{#if qdMenu}
  <div
    class="qd-menu"
    style="{qdMenu.left ? 'right' : 'left'}: {qdMenu.x}px; {qdMenu.up ? 'bottom' : 'top'}: {qdMenu.y}px"
    on:click|stopPropagation
    on:contextmenu|preventDefault
  >
    <div class="qd-menu-title">{$t('qdMenuTitle')}</div>
    <button class="qd-item" disabled={bubbleLocked} on:click={() => toggleOnce('bubble')}>
      <span class="qd-check" class:on={bubbleLocked || $queueDoneOnce.bubble}></span>
      <span class="qd-text">
        {$t('qdBubble')}
        <small>{bubbleLocked ? $t('qdOnInSettings') : $t('qdThisQueueOnly')}</small>
      </span>
    </button>
    <button class="qd-item" disabled={soundLocked} on:click={() => toggleOnce('sound')}>
      <span class="qd-check" class:on={soundLocked || $queueDoneOnce.sound}></span>
      <span class="qd-text">
        {$t('qdSound')}
        <small>{soundLocked ? $t('qdOnInSettings') : $t('qdThisQueueOnly')}</small>
      </span>
    </button>
    <hr class="qd-sep" />
    {#each actions as a (a.key)}
      <button class="qd-item" class:danger={a.key === 'shutdown'} on:click={() => chooseAction(a.key)}>
        <span class="qd-radio" class:on={$queueDoneAction === a.key}></span>
        <span class="qd-text">
          {a.label}
          {#if a.key === 'command' && $settings?.queueDoneCommand}
            <small class="qd-cmd">{$settings.queueDoneCommand}</small>
          {/if}
        </span>
      </button>
    {/each}
  </div>
{/if}

{#if cmdPrompt}
  <div class="qd-overlay" on:click|self={() => cmdPrompt = null}>
    <div
      class="qd-box"
      role="dialog"
      aria-modal="true"
      use:trapFocus
      on:keydown|stopPropagation={(e) => { if (e.key === 'Escape') cmdPrompt = null; }}
    >
      <div class="qd-box-title">{$t('qdCommandTitle')}</div>
      <input
        class="qd-input"
        type="text"
        spellcheck="false"
        placeholder={isWindows ? 'echo %GLIDEFTP_DONE% > C:\\fin.txt' : 'notify-send GlideFTP "$GLIDEFTP_DONE OK, $GLIDEFTP_FAILED KO"'}
        bind:value={cmdPrompt.value}
        use:focusOnOpen
        on:keydown={(e) => { if (e.key === 'Enter') confirmCommand(); }}
      />
      <div class="qd-help">{(isWindows ? $t('qdCommandHelpWin') : $t('qdCommandHelp'))}</div>
      <div class="qd-box-actions">
        <button class="qd-cancel" on:click={() => cmdPrompt = null}>{$t('cancel')}</button>
        <button class="qd-ok" disabled={!cmdPrompt.value.trim()} on:click={confirmCommand}>{$t('validate')}</button>
      </div>
    </div>
  </div>
{/if}

<style>
.queue-panel {
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--border);
  background: var(--bg-secondary);
  flex-shrink: 0;
  position: relative;
}

.queue-resize-handle {
  height: 4px;
  cursor: row-resize;
  background: var(--border);
  flex-shrink: 0;
  transition: background 0.15s;
}

.queue-resize-handle:hover, .queue-resize-handle.active {
  background: var(--accent-bg);
}

.queue-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 12px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.queue-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  white-space: nowrap;
}

.tabs {
  display: flex;
  gap: 2px;
  flex: 1;
}

.tab {
  display: flex;
  align-items: center;
  gap: 5px;
  background: transparent;
  border: none;
  border-bottom: 2px solid transparent;
  color: var(--text-muted);
  padding: 4px 10px;
  font-size: 12px;
  cursor: pointer;
  transition: color 0.12s;
  border-radius: 0;
}

.tab:hover {
  color: var(--text-primary);
}

.tab.active {
  color: var(--accent);
  border-bottom-color: var(--accent);
}

.badge {
  background: var(--accent-bg);
  color: white;
  border-radius: 10px;
  font-size: 10px;
  padding: 1px 5px;
  font-weight: 700;
}

.queue-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.small-btn {
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 3px;
  color: var(--text-secondary);
  padding: 2px 8px;
  font-size: 11px;
  cursor: pointer;
}

.small-btn:hover {
  background: var(--bg-button-hover);
}

.small-btn.danger {
  color: var(--danger);
  border-color: var(--danger);
}
.small-btn.primary {
  background: var(--accent-bg) border-box;
  border-color: transparent;
  color: white;
}
.small-btn.primary:hover:not(:disabled) { background: var(--accent-hover-bg) border-box; }
.small-btn:disabled { opacity: 0.45; cursor: default; }
.small-btn.cancel-all:hover {
  background: var(--danger);
  color: white;
}

.close-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 14px;
  padding: 0 4px;
}

.close-btn:hover {
  color: var(--text-primary);
}

.queue-body {
  overflow-y: auto;
  flex: 1;
}

.empty {
  text-align: center;
  padding: 20px;
  color: var(--text-muted);
  font-size: 12px;
}

.job-row {
  padding: 6px 12px;
  border-bottom: 1px solid var(--border-subtle);
  font-size: 12px;
}

.job-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.job-direction {
  font-size: 14px;
  font-weight: bold;
  color: var(--accent);
  width: 16px;
  text-align: center;
}

.job-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-primary);
}

.job-size {
  color: var(--text-muted);
  width: 70px;
  text-align: right;
}

.job-status {
  width: 70px;
  text-align: right;
  font-size: 11px;
  font-weight: 500;
}

.job-status.pending { color: var(--text-muted); }
.job-status.running { color: var(--accent); }
.job-status.done { color: var(--success); }
.job-status.failed, .job-status.cancelled { color: var(--danger); }

.progress-bar {
  height: 3px;
  background: var(--border);
  border-radius: 2px;
  margin-top: 4px;
  overflow: hidden;
}

/* Paused / failed: how far it got, greyed */
.progress-bar.stopped .progress-fill { background: var(--text-muted); }

.progress-fill {
  height: 100%;
  background: var(--accent-bg);
  border-radius: 2px;
  transition: width 0.3s ease;
}

.progress-label {
  font-size: 10px;
  color: var(--text-muted);
  margin-top: 2px;
  display: block;
}

.speed-label {
  color: var(--accent);
  font-weight: 500;
}

.job-route {
  font-size: 10px;
  color: var(--text-muted);
  margin-top: 2px;
  font-style: italic;
}

.avg-speed {
  color: var(--accent);
  font-style: normal;
  font-weight: 500;
}

.job-error {
  color: var(--danger);
  font-size: 11px;
  margin-top: 2px;
}

.job-actions {
  display: flex;
  gap: 4px;
  margin-top: 4px;
  align-items: center;
}

.remove-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 16px;
  padding: 0 4px;
  line-height: 1;
  margin-left: auto;
}

.remove-btn:hover {
  color: var(--danger);
}

/* ── Accent gradient mode ── */
:global(html[data-accent-gradient]) .tab.active,
:global(html[data-accent-gradient]) .job-direction,
:global(html[data-accent-gradient]) .job-status.running,
:global(html[data-accent-gradient]) .speed-label,
:global(html[data-accent-gradient]) .avg-speed {
  background: var(--accent-gradient);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
/* text-fill-color is inherited: keep the count badge's number visible */
:global(html[data-accent-gradient]) .tab.active .badge { -webkit-text-fill-color: white; }
:global(html[data-accent-gradient]) .tab.active { border-image: var(--accent-gradient) 1; }

/* ── After the queue: badge, menu, command prompt ─────────────────────── */
.qd-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 2px 4px 2px 8px;
  border-radius: 10px;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
  color: var(--accent);
  border: 1px solid var(--accent);
  background: var(--accent-subtle-bg);
}
.qd-badge.danger {
  color: var(--danger);
  border-color: var(--danger);
  background: rgba(229, 83, 75, 0.12);
}
.qd-badge svg { width: 12px; height: 12px; flex-shrink: 0; }
.qd-badge-x {
  background: none;
  border: none;
  color: inherit;
  font-size: 10px;
  cursor: pointer;
  padding: 0 3px;
  opacity: 0.75;
}
.qd-badge-x:hover { opacity: 1; }

.qd-menu {
  position: fixed;
  z-index: 1000;
  min-width: 270px;
  max-width: 330px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 7px;
  padding: 4px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.4);
}
.qd-menu-title {
  padding: 6px 10px 5px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
}
.qd-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  text-align: left;
  background: none;
  border: none;
  border-radius: 4px;
  padding: 6px 10px;
  color: var(--text-primary);
  font-size: 13px;
  cursor: pointer;
}
.qd-item:hover:not(:disabled) { background: var(--bg-hover); }
.qd-item:disabled { cursor: default; }
.qd-item.danger { color: var(--danger); }
.qd-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.qd-text small {
  font-size: 11px;
  color: var(--text-muted);
}
.qd-cmd {
  font-family: monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.qd-check, .qd-radio {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  border: 2px solid var(--text-muted);
  box-sizing: border-box;
  position: relative;
}
.qd-check { border-radius: 3px; }
.qd-radio { border-radius: 50%; }
.qd-check.on {
  border-color: transparent;
  background: var(--accent-bg) border-box;
}
.qd-check.on::after {
  content: '';
  position: absolute;
  left: 3px;
  top: 0;
  width: 4px;
  height: 8px;
  border: solid white;
  border-width: 0 2px 2px 0;
  transform: rotate(45deg);
}
.qd-item:disabled .qd-check.on { opacity: 0.6; }
.qd-radio.on {
  border-color: var(--accent);
  background: radial-gradient(var(--accent) 0 3px, transparent 3.5px);
}
.qd-sep { border: none; border-top: 1px solid var(--border); margin: 4px 0; }

.qd-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 4800;
}
.qd-box {
  background: linear-gradient(var(--bg-secondary), var(--bg-secondary)) padding-box,
              var(--accent-gradient) border-box;
  border: 2px solid transparent;
  border-radius: 10px;
  padding: 22px 26px;
  width: 520px;
  max-width: 95vw;
  display: flex;
  flex-direction: column;
  gap: 12px;
  box-shadow: 0 16px 48px rgba(0,0,0,0.5);
}
.qd-box-title { font-size: 15px; font-weight: 600; color: var(--text-primary); }
.qd-input {
  background: var(--bg-input, var(--bg-primary));
  border: 1px solid var(--border);
  border-radius: 5px;
  padding: 8px 10px;
  color: var(--text-primary);
  font-family: monospace;
  font-size: 13px;
}
.qd-input:focus { outline: none; border-color: var(--accent); }
.qd-help {
  font-size: 12px;
  color: var(--text-secondary);
  white-space: pre-line;
  line-height: 1.5;
}
.qd-box-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
.qd-cancel, .qd-ok {
  border-radius: 5px;
  padding: 7px 16px;
  font-size: 13px;
  cursor: pointer;
}
.qd-cancel {
  background: var(--bg-button);
  border: 1px solid var(--border);
  color: var(--text-secondary);
}
.qd-cancel:hover { background: var(--bg-button-hover); }
.qd-ok {
  background: var(--accent-bg) border-box;
  border: 1px solid transparent;
  color: white;
  font-weight: 500;
}
.qd-ok:hover:not(:disabled) { background: var(--accent-hover-bg) border-box; }
.qd-ok:disabled { opacity: 0.5; cursor: default; }
</style>
