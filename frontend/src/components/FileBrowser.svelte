<script>
  import { t } from '../i18n/index.js';
  import { formatBytes } from '../stores/transfers.js';
  import { QueueUpload, QueueDownload } from '../../wailsjs/go/main/App.js';
  import { queueVisible } from '../stores/transfers.js';

  export let side = 'local'; // 'local' | 'remote'
  export let path = '';
  export let entries = [];
  export let selected = [];
  export let onNavigate = async (path) => {};
  export let onNavigateUp = async () => {};
  export let onRefresh = async () => {};
  export let onMkDir = async (path) => {};
  export let onDelete = async (path) => {};
  export let onRename = async (oldPath, newPath) => {};

  // For transfer
  export let otherPath = '';

  let renamingEntry = null;
  let renameValue = '';
  let newFolderMode = false;
  let newFolderName = '';
  let contextMenu = null;
  let contextEntry = null;

  function formatDate(dateStr) {
    if (!dateStr) return '';
    const d = new Date(dateStr);
    return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }

  function handleClick(entry) {
    selected = [entry];
  }

  function handleDblClick(entry) {
    if (entry.isDir) {
      onNavigate(entry.path);
    }
  }

  function handleContextMenu(e, entry) {
    e.preventDefault();
    contextEntry = entry;
    contextMenu = { x: e.clientX, y: e.clientY };
  }

  function closeContext() {
    contextMenu = null;
    contextEntry = null;
  }

  function startRename(entry) {
    renamingEntry = entry;
    renameValue = entry.name;
    closeContext();
  }

  async function doRename(entry) {
    if (!renameValue || renameValue === entry.name) {
      renamingEntry = null;
      return;
    }
    const dir = entry.path.substring(0, entry.path.lastIndexOf('/') + 1) ||
                 entry.path.substring(0, entry.path.lastIndexOf('\\') + 1);
    const newPath = dir + renameValue;
    await onRename(entry.path, newPath);
    renamingEntry = null;
    await onRefresh();
  }

  async function handleDelete(entry) {
    await onDelete(entry.path);
    closeContext();
    await onRefresh();
  }

  async function handleNewFolder() {
    if (!newFolderName) { newFolderMode = false; return; }
    const newPath = path.endsWith('/') ? path + newFolderName : path + '/' + newFolderName;
    await onMkDir(newPath);
    newFolderName = '';
    newFolderMode = false;
    await onRefresh();
  }

  async function transferSelected() {
    if (selected.length === 0) return;
    for (const entry of selected) {
      if (side === 'local') {
        const remoteDest = otherPath.endsWith('/') ? otherPath + entry.name : otherPath + '/' + entry.name;
        QueueUpload(entry.path, remoteDest);
      } else {
        const localDest = otherPath.endsWith('/') ? otherPath + entry.name : otherPath + '/' + entry.name;
        QueueDownload(entry.path, localDest);
      }
    }
    queueVisible.set(true);
  }
</script>

<svelte:window on:click={closeContext} />

<div class="browser">
  <div class="browser-header">
    <span class="side-label">{side === 'local' ? $t('local') : $t('remote')}</span>
    <div class="path-nav">
      <button class="icon-btn" on:click={onNavigateUp} title="..">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 18 9 12 15 6"/></svg>
      </button>
      <div class="path-display" title={path}>{path}</div>
    </div>
    <div class="browser-actions">
      <button class="icon-btn" on:click={onRefresh} title={$t('refresh')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>
      </button>
      <button class="icon-btn" on:click={() => { newFolderMode = true; }} title={$t('newFolder')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/><line x1="12" y1="11" x2="12" y2="17"/><line x1="9" y1="14" x2="15" y2="14"/></svg>
      </button>
      {#if selected.length > 0}
        <button class="icon-btn transfer-btn" on:click={transferSelected} title={$t('transfer')}>
          {#if side === 'local'}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/></svg>
          {:else}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/></svg>
          {/if}
        </button>
      {/if}
    </div>
  </div>

  {#if newFolderMode}
    <div class="new-folder-row">
      <input
        type="text"
        bind:value={newFolderName}
        placeholder={$t('newFolder')}
        autofocus
        on:keydown={(e) => { if (e.key === 'Enter') handleNewFolder(); if (e.key === 'Escape') { newFolderMode = false; } }}
      />
      <button on:click={handleNewFolder}>{$t('save')}</button>
      <button on:click={() => { newFolderMode = false; newFolderName = ''; }}>{$t('cancel')}</button>
    </div>
  {/if}

  <div class="file-list">
    <div class="file-list-header">
      <span class="col-name">{$t('name')}</span>
      <span class="col-size">{$t('size')}</span>
      <span class="col-date">{$t('date')}</span>
    </div>

    {#if entries.length === 0}
      <div class="empty">{$t('emptyFolder')}</div>
    {:else}
      {#each entries as entry (entry.path)}
        <div
          class="file-row"
          class:selected={selected.some(s => s.path === entry.path)}
          class:is-dir={entry.isDir}
          on:click={() => handleClick(entry)}
          on:dblclick={() => handleDblClick(entry)}
          on:contextmenu={(e) => handleContextMenu(e, entry)}
        >
          <span class="col-name">
            <span class="file-icon">
              {#if entry.isDir}📁{:else}📄{/if}
            </span>
            {#if renamingEntry?.path === entry.path}
              <input
                class="rename-input"
                type="text"
                bind:value={renameValue}
                on:click|stopPropagation
                on:keydown={(e) => { if (e.key === 'Enter') doRename(entry); if (e.key === 'Escape') renamingEntry = null; }}
                autofocus
              />
            {:else}
              <span class="file-name">{entry.name}</span>
            {/if}
          </span>
          <span class="col-size">{entry.isDir ? '' : formatBytes(entry.size)}</span>
          <span class="col-date">{formatDate(entry.modTime)}</span>
        </div>
      {/each}
    {/if}
  </div>
</div>

{#if contextMenu && contextEntry}
  <div
    class="context-menu"
    style="left: {contextMenu.x}px; top: {contextMenu.y}px"
    on:click|stopPropagation
  >
    <button on:click={() => startRename(contextEntry)}>{$t('rename')}</button>
    <button class="danger" on:click={() => handleDelete(contextEntry)}>{$t('delete')}</button>
  </div>
{/if}

<style>
.browser {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-width: 0;
  background: var(--bg-primary);
}

.browser-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.side-label {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  color: var(--accent);
  letter-spacing: 0.05em;
  white-space: nowrap;
}

.path-nav {
  display: flex;
  align-items: center;
  gap: 4px;
  flex: 1;
  min-width: 0;
}

.path-display {
  font-size: 12px;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.browser-actions {
  display: flex;
  gap: 2px;
  flex-shrink: 0;
}

.icon-btn {
  width: 26px;
  height: 26px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.12s, color 0.12s;
  padding: 0;
}

.icon-btn:hover {
  background: var(--bg-button-hover);
  color: var(--text-primary);
}

.icon-btn svg {
  width: 15px;
  height: 15px;
}

.transfer-btn {
  color: var(--accent);
}

.transfer-btn:hover {
  background: var(--accent-subtle);
  color: var(--accent);
}

.new-folder-row {
  display: flex;
  gap: 6px;
  padding: 6px 10px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border);
}

.new-folder-row input {
  flex: 1;
  background: var(--bg-input);
  border: 1px solid var(--accent);
  border-radius: 4px;
  color: var(--text-primary);
  padding: 3px 8px;
  font-size: 12px;
  outline: none;
}

.new-folder-row button {
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-secondary);
  padding: 3px 10px;
  font-size: 12px;
  cursor: pointer;
}

.file-list {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
}

.file-list-header {
  display: flex;
  gap: 8px;
  padding: 4px 10px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border);
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  position: sticky;
  top: 0;
  z-index: 1;
}

.col-name { flex: 1; min-width: 0; }
.col-size { width: 80px; text-align: right; flex-shrink: 0; }
.col-date { width: 130px; text-align: right; flex-shrink: 0; }

.file-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 10px;
  font-size: 13px;
  cursor: pointer;
  border-bottom: 1px solid var(--border-subtle);
  user-select: none;
  transition: background 0.08s;
}

.file-row:hover {
  background: var(--bg-hover);
}

.file-row.selected {
  background: var(--accent-subtle);
}

.file-icon {
  font-size: 14px;
  margin-right: 4px;
  flex-shrink: 0;
}

.file-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.is-dir .file-name {
  font-weight: 500;
}

.rename-input {
  flex: 1;
  background: var(--bg-input);
  border: 1px solid var(--accent);
  border-radius: 3px;
  color: var(--text-primary);
  padding: 1px 6px;
  font-size: 13px;
  outline: none;
}

.empty {
  padding: 40px;
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
}

.context-menu {
  position: fixed;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 6px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.3);
  z-index: 1000;
  overflow: hidden;
  min-width: 140px;
}

.context-menu button {
  display: block;
  width: 100%;
  background: none;
  border: none;
  color: var(--text-primary);
  padding: 8px 14px;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  transition: background 0.1s;
}

.context-menu button:hover {
  background: var(--bg-hover);
}

.context-menu button.danger {
  color: var(--danger);
}
</style>
