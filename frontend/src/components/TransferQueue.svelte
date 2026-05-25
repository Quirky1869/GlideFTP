<script>
  import { t } from '../i18n/index.js';
  import { transfers, cancelTransfer, retryTransfer, clearTransfers, removeTransfer, formatBytes, progressPct, queueVisible } from '../stores/transfers.js';

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

  $: pending = $transfers.filter(j => j.status === 'pending' || j.status === 'running');
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
</script>

<div class="queue-panel" style="height: {queueHeight}px;">
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
      {#if activeTab !== 'pending' && currentList.length > 0}
        <button class="small-btn" on:click={() => clearTransfers(activeTab === 'failed' ? 'failed' : 'done')}>
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
          {#if job.status === 'running'}
            <div class="progress-bar">
              <div class="progress-fill" style="width: {progressPct(job)}%"></div>
            </div>
            <span class="progress-label">
              {progressPct(job)}% — {formatBytes(job.bytesDone)} / {formatBytes(job.size)}
              {#if speeds[job.id] > 0}<span class="speed-label"> • {formatSpeed(speeds[job.id])}</span>{/if}
            </span>
          {/if}
          {#if job.error}
            <div class="job-error">{job.error}</div>
          {/if}
          <div class="job-actions">
            {#if job.status === 'pending' || job.status === 'running'}
              <button class="small-btn danger" on:click={() => cancelTransfer(job.id)}>{$t('cancel')}</button>
            {/if}
            {#if job.status === 'failed'}
              <button class="small-btn" on:click={() => retryTransfer(job.id)}>{$t('retry')}</button>
            {/if}
            {#if job.status === 'done' || job.status === 'failed' || job.status === 'cancelled'}
              <button class="remove-btn" on:click={() => removeTransfer(job.id)} title="Supprimer">×</button>
            {/if}
          </div>
        </div>
      {/each}
    {/if}
  </div>
</div>

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
  background: var(--accent);
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
  background: var(--accent);
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

.progress-fill {
  height: 100%;
  background: var(--accent);
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
</style>
