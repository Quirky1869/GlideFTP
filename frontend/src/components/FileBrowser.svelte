<script>
  import { t } from '../i18n/index.js';
  import { formatBytes } from '../stores/transfers.js';
  import { QueueUpload, QueueDownload } from '../../wailsjs/go/main/App.js';
  import { queueVisible } from '../stores/transfers.js';

  export let side = 'local'; // 'local' | 'remote'
  export let path = '';
  export let entries = [];
  export let selected = [];
  export let onNavigate = async (_path) => {};
  export let onNavigateUp = async () => {};
  export let onRefresh = async () => {};
  export let onMkDir = async (_path) => {};
  export let onDelete = async (_path) => {};
  export let onRename = async (_old, _new) => {};
  export let otherPath = '';

  let renamingEntry = null;
  let renameValue = '';
  let newFolderMode = false;
  let newFolderName = '';
  let contextMenu = null;
  let contextEntry = null;
  let contextIsEmpty = false;
  let dragOver = false;
  let panelEl;

  // Editable path bar
  let editingPath = false;
  let editPathValue = '';

  function startEditPath() {
    editPathValue = path;
    editingPath = true;
  }

  async function confirmEditPath() {
    editingPath = false;
    if (editPathValue && editPathValue !== path) {
      await onNavigate(editPathValue);
    }
  }

  function handlePathKeydown(e) {
    if (e.key === 'Enter') { e.preventDefault(); confirmEditPath(); }
    if (e.key === 'Escape') { editingPath = false; }
  }

  // Sort state
  let sortKey = 'name';
  let sortDir = 'asc';

  $: sortedEntries = (() => {
    const dirs = entries.filter(e => e.isDir);
    const files = entries.filter(e => !e.isDir);
    const cmp = (a, b) => {
      let v = 0;
      if (sortKey === 'name') {
        v = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
      } else if (sortKey === 'size') {
        v = (a.size || 0) - (b.size || 0);
      } else if (sortKey === 'date') {
        v = (new Date(a.modTime || 0)) - (new Date(b.modTime || 0));
      }
      return sortDir === 'asc' ? v : -v;
    };
    return [...dirs.sort(cmp), ...files.sort(cmp)];
  })();

  function toggleSort(key) {
    if (sortKey === key) sortDir = sortDir === 'asc' ? 'desc' : 'asc';
    else { sortKey = key; sortDir = 'asc'; }
  }

  function formatDate(dateStr) {
    if (!dateStr) return '';
    const d = new Date(dateStr);
    return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }

  // ── Selection ─────────────────────────────────────────────────────────────

  function handleClick(e, entry) {
    if (e.ctrlKey || e.metaKey) {
      selected = selected.some(s => s.path === entry.path)
        ? selected.filter(s => s.path !== entry.path)
        : [...selected, entry];
    } else if (e.shiftKey && selected.length > 0) {
      const all = sortedEntries;
      const lastPath = selected[selected.length - 1].path;
      const li = all.findIndex(e => e.path === lastPath);
      const ci = all.findIndex(e => e.path === entry.path);
      const [s, en_] = [Math.min(li, ci), Math.max(li, ci)];
      selected = all.slice(s, en_ + 1);
    } else {
      selected = [entry];
    }
    panelEl?.focus({ preventScroll: true });
  }

  function handleDblClick(entry) {
    if (entry.isDir) onNavigate(entry.path);
  }

  // ── Context menu ──────────────────────────────────────────────────────────

  function handleFileContextMenu(e, entry) {
    e.preventDefault();
    contextEntry = entry;
    contextIsEmpty = false;
    contextMenu = { x: e.clientX, y: e.clientY };
    if (!selected.some(s => s.path === entry.path)) selected = [entry];
  }

  function handleBrowserContextMenu(e) {
    e.preventDefault();
    contextEntry = null;
    contextIsEmpty = true;
    contextMenu = { x: e.clientX, y: e.clientY };
  }

  function closeContext() {
    contextMenu = null;
    contextEntry = null;
    contextIsEmpty = false;
  }

  // ── Rename ────────────────────────────────────────────────────────────────

  function startRename(entry) {
    renamingEntry = entry;
    renameValue = entry.name;
    closeContext();
  }

  async function doRename(entry) {
    if (!renameValue || renameValue === entry.name) { renamingEntry = null; return; }
    const sep = entry.path.includes('/') ? '/' : '\\';
    const dir = entry.path.substring(0, entry.path.lastIndexOf(sep) + 1);
    await onRename(entry.path, dir + renameValue);
    renamingEntry = null;
    await onRefresh();
  }

  // ── Delete ────────────────────────────────────────────────────────────────

  async function handleDelete(entry) {
    await onDelete(entry.path);
    closeContext();
    await onRefresh();
  }

  // ── New folder ────────────────────────────────────────────────────────────

  async function handleNewFolder() {
    if (!newFolderName) { newFolderMode = false; return; }
    const base = path.replace(/[/\\]?$/, '/');
    await onMkDir(base + newFolderName);
    newFolderName = '';
    newFolderMode = false;
    await onRefresh();
  }

  // ── Transfer ──────────────────────────────────────────────────────────────

  function queueTransfer(entry) {
    const dest = otherPath.replace(/[/\\]?$/, '/') + entry.name;
    if (side === 'local') QueueUpload(entry.path, dest);
    else QueueDownload(entry.path, dest);
  }

  async function transferSelected() {
    if (!selected.length) return;
    for (const entry of selected) queueTransfer(entry);
    queueVisible.set(true);
  }

  async function transferEntry(entry) {
    queueTransfer(entry);
    queueVisible.set(true);
    closeContext();
  }

  // ── Keyboard (F2 rename) ──────────────────────────────────────────────────

  function handlePanelKeydown(e) {
    if (e.key === 'F2' && selected.length === 1 && !renamingEntry) {
      e.preventDefault();
      startRename(selected[0]);
    }
  }

  // ── Drag and drop ─────────────────────────────────────────────────────────

  function handleDragStart(e, entry) {
    e.dataTransfer.effectAllowed = 'copy';
    e.dataTransfer.setData('application/glideftp', JSON.stringify({
      path: entry.path,
      name: entry.name,
      fromSide: side,
    }));
  }

  function handleDragOver(e) {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'copy';
    dragOver = true;
  }

  function handleDragLeave(e) {
    if (!e.currentTarget.contains(e.relatedTarget)) dragOver = false;
  }

  async function handleDrop(e) {
    e.preventDefault();
    dragOver = false;
    const raw = e.dataTransfer.getData('application/glideftp');
    if (!raw) return;
    try {
      const { path: srcPath, name, fromSide } = JSON.parse(raw);
      if (fromSide === side) return;
      const dest = path.replace(/[/\\]?$/, '/') + name;
      if (side === 'remote') QueueUpload(srcPath, dest);
      else QueueDownload(srcPath, dest);
      queueVisible.set(true);
    } catch {}
  }
</script>

<svelte:window on:click={closeContext} />

<div
  class="browser"
  class:drag-over={dragOver}
  bind:this={panelEl}
  tabindex="-1"
  on:keydown={handlePanelKeydown}
  on:contextmenu={handleBrowserContextMenu}
  on:dragover={handleDragOver}
  on:dragleave={handleDragLeave}
  on:drop={handleDrop}
  role="region"
>
  <div class="browser-header" on:contextmenu|stopPropagation>
    <span class="side-label">{side === 'local' ? $t('local') : $t('remote')}</span>
    <div class="path-nav">
      <button class="icon-btn" on:click={onNavigateUp} title="Parent folder">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 18 9 12 15 6"/></svg>
      </button>
      {#if editingPath}
        <input
          class="path-edit"
          type="text"
          bind:value={editPathValue}
          on:keydown={handlePathKeydown}
          on:blur={confirmEditPath}
          autofocus
        />
      {:else}
        <div class="path-display" title={path} on:click={startEditPath}>{path}</div>
      {/if}
    </div>
    <div class="browser-actions">
      <button class="icon-btn" on:click={onRefresh} title={$t('refresh')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>
      </button>
      <button class="icon-btn" on:click={() => { newFolderMode = true; closeContext(); }} title={$t('newFolder')}>
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
    <div class="new-folder-row" on:contextmenu|stopPropagation>
      <input
        type="text"
        bind:value={newFolderName}
        placeholder={$t('newFolder')}
        autofocus
        on:keydown={(e) => { if (e.key === 'Enter') handleNewFolder(); if (e.key === 'Escape') { newFolderMode = false; newFolderName = ''; } }}
      />
      <button on:click={handleNewFolder}>{$t('save')}</button>
      <button on:click={() => { newFolderMode = false; newFolderName = ''; }}>{$t('cancel')}</button>
    </div>
  {/if}

  <div class="file-list">
    <div class="file-list-header" on:contextmenu|stopPropagation>
      <span class="col-name col-sortable" on:click={() => toggleSort('name')}>
        {$t('name')}{#if sortKey === 'name'} <span class="sort-arr">{sortDir === 'asc' ? '↑' : '↓'}</span>{/if}
      </span>
      <span class="col-size col-sortable" on:click={() => toggleSort('size')}>
        {$t('size')}{#if sortKey === 'size'} <span class="sort-arr">{sortDir === 'asc' ? '↑' : '↓'}</span>{/if}
      </span>
      <span class="col-date col-sortable" on:click={() => toggleSort('date')}>
        {$t('date')}{#if sortKey === 'date'} <span class="sort-arr">{sortDir === 'asc' ? '↑' : '↓'}</span>{/if}
      </span>
    </div>

    <!-- ".." parent folder row -->
    <div
      class="file-row is-dir parent-row"
      on:click={onNavigateUp}
      on:dblclick={onNavigateUp}
      on:contextmenu|stopPropagation
    >
      <span class="col-name">
        <span class="file-icon">📁</span>
        <span class="file-name">..</span>
      </span>
      <span class="col-size"></span>
      <span class="col-date"></span>
    </div>

    {#if entries.length === 0}
      <div class="empty">{$t('emptyFolder')}</div>
    {:else}
      {#each sortedEntries as entry (entry.path)}
        <div
          class="file-row"
          class:selected={selected.some(s => s.path === entry.path)}
          class:is-dir={entry.isDir}
          draggable={true}
          on:click={(e) => handleClick(e, entry)}
          on:dblclick={() => handleDblClick(entry)}
          on:contextmenu|stopPropagation={(e) => handleFileContextMenu(e, entry)}
          on:dragstart={(e) => handleDragStart(e, entry)}
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

{#if contextMenu}
  <div
    class="context-menu"
    style="left: {contextMenu.x}px; top: {contextMenu.y}px"
    on:click|stopPropagation
  >
    {#if contextIsEmpty}
      <button on:click={() => { newFolderMode = true; closeContext(); }}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/><line x1="12" y1="11" x2="12" y2="17"/><line x1="9" y1="14" x2="15" y2="14"/></svg>
        {$t('newFolder')}
      </button>
    {:else}
      <button on:click={() => startRename(contextEntry)}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
        {$t('rename')}
      </button>
      <button on:click={() => transferEntry(contextEntry)}>
        {#if side === 'local'}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/></svg>
        {:else}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/></svg>
        {/if}
        {$t('transfer')}
      </button>
      <hr class="menu-sep" />
      <button class="danger" on:click={() => handleDelete(contextEntry)}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/></svg>
        {$t('delete')}
      </button>
    {/if}
  </div>
{/if}

<style>
.browser {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  min-width: 0;
  background: var(--bg-primary);
  outline: none;
}

.browser.drag-over {
  box-shadow: inset 0 0 0 2px var(--accent);
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
  cursor: text;
  border-radius: 3px;
  padding: 2px 4px;
}

.path-display:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.path-edit {
  flex: 1;
  min-width: 0;
  background: var(--bg-input);
  border: 1px solid var(--accent);
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 12px;
  padding: 2px 6px;
  outline: none;
  height: 24px;
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

.transfer-btn { color: var(--accent); }
.transfer-btn:hover { background: var(--accent-subtle); color: var(--accent); }

.new-folder-row {
  display: flex;
  gap: 6px;
  padding: 6px 10px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
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

.col-sortable {
  cursor: pointer;
  user-select: none;
}

.col-sortable:hover {
  color: var(--text-primary);
}

.sort-arr {
  color: var(--accent);
  font-size: 10px;
}

.col-name { flex: 1; min-width: 0; }
.col-size { width: 80px; text-align: right; flex-shrink: 0; }
.col-date { width: 130px; text-align: right; flex-shrink: 0; }

.file-row .col-name { display: flex; align-items: center; }
.file-list-header .col-name { display: flex; align-items: center; gap: 4px; }

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

.file-row:hover { background: var(--bg-hover); }
.file-row.selected { background: var(--accent-subtle); }

.parent-row { color: var(--text-muted); }
.parent-row:hover { color: var(--text-primary); }

.file-icon {
  font-size: 14px;
  margin-right: 4px;
  flex-shrink: 0;
}

.file-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.is-dir .file-name { font-weight: 500; }

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
  min-width: 160px;
}

.context-menu button {
  display: flex;
  align-items: center;
  gap: 8px;
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

.context-menu button:hover { background: var(--bg-hover); }
.context-menu button svg { width: 14px; height: 14px; flex-shrink: 0; }
.context-menu button.danger { color: var(--danger); }

.menu-sep {
  border: none;
  border-top: 1px solid var(--border);
  margin: 3px 0;
}
</style>
