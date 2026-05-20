<script>
  import { t } from '../i18n/index.js';
  import { formatBytes } from '../stores/transfers.js';
  import { settings } from '../stores/settings.js';
  import { QueueUpload, QueueDownload, LocalListDir, RemoteListDir } from '../../wailsjs/go/main/App.js';
  import { queueVisible } from '../stores/transfers.js';

  export let side = 'local';
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
  let fileListEl;

  // ── Delete confirmation ───────────────────────────────────────────────────

  let confirmDeleteEntry = null;
  let deleteError = '';

  function handleDelete(entry) {
    closeContext();
    if ($settings?.confirmOnDelete) {
      confirmDeleteEntry = entry;
    } else {
      doDelete(entry);
    }
  }

  async function doDelete(entry) {
    deleteError = '';
    try {
      await onDelete(entry.path);
      await onRefresh();
    } catch (e) {
      deleteError = e?.message || e?.toString() || 'Delete failed';
      setTimeout(() => { deleteError = ''; }, 4000);
      await onRefresh(); // refresh anyway so UI reflects actual state
    }
    confirmDeleteEntry = null;
  }

  // ── Editable path bar ─────────────────────────────────────────────────────

  let editingPath = false;
  let editPathValue = '';
  let pathSuggestions = [];
  let highlightedSugg = -1;
  let suggDebounce = null;

  function startEditPath() {
    editPathValue = path;
    editingPath = true;
    pathSuggestions = [];
    highlightedSugg = -1;
  }

  async function confirmEditPath() {
    editingPath = false;
    pathSuggestions = [];
    if (editPathValue && editPathValue !== path) {
      try { await onNavigate(editPathValue); } catch {}
    }
  }

  function handlePathKeydown(e) {
    if (e.key === 'ArrowDown') {
      highlightedSugg = Math.min(highlightedSugg + 1, pathSuggestions.length - 1);
      e.preventDefault();
    } else if (e.key === 'ArrowUp') {
      highlightedSugg = Math.max(highlightedSugg - 1, -1);
      e.preventDefault();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (highlightedSugg >= 0 && pathSuggestions[highlightedSugg]) {
        editPathValue = pathSuggestions[highlightedSugg];
        pathSuggestions = [];
        highlightedSugg = -1;
      } else {
        confirmEditPath();
      }
    } else if (e.key === 'Escape') {
      editingPath = false;
      pathSuggestions = [];
    }
  }

  function onPathInput(value) {
    clearTimeout(suggDebounce);
    highlightedSugg = -1;
    suggDebounce = setTimeout(() => fetchSuggestions(value), 350);
  }

  async function fetchSuggestions(value) {
    if (!value || value.length < 2) { pathSuggestions = []; return; }
    const sep = '/';
    const lastSepIdx = value.lastIndexOf(sep);
    if (lastSepIdx < 0) { pathSuggestions = []; return; }
    const parentDir = lastSepIdx === 0 ? '/' : value.substring(0, lastSepIdx);
    const prefix = value.substring(lastSepIdx + 1).toLowerCase();
    try {
      const list = side === 'local' ? await LocalListDir(parentDir) : await RemoteListDir(parentDir);
      pathSuggestions = (list || [])
        .filter(e => e.isDir && e.name.toLowerCase().startsWith(prefix))
        .map(e => e.path)
        .slice(0, 8);
    } catch {
      pathSuggestions = [];
    }
  }

  function selectSuggestion(s) {
    editPathValue = s;
    pathSuggestions = [];
    highlightedSugg = -1;
  }

  // ── Sort ──────────────────────────────────────────────────────────────────

  let sortKey = 'name';
  let sortDir = 'asc';

  $: sortedEntries = (() => {
    const dirs = entries.filter(e => e.isDir);
    const files = entries.filter(e => !e.isDir);
    const cmp = (a, b) => {
      let v = 0;
      if (sortKey === 'name') v = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
      else if (sortKey === 'size') v = (a.size || 0) - (b.size || 0);
      else if (sortKey === 'date') v = (new Date(a.modTime || 0)) - (new Date(b.modTime || 0));
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

  // ── Rubber-band selection ─────────────────────────────────────────────────

  let rubberBand = null;

  function handleListMouseDown(e) {
    if (e.button !== 0) return;
    if (e.target.closest('.file-row')) return;
    if (e.target.closest('.file-list-header')) return;

    const startX = e.clientX;
    const startY = e.clientY;
    let dragging = false;

    const onMove = (ev) => {
      if (!dragging) {
        const dx = ev.clientX - startX;
        const dy = ev.clientY - startY;
        if (Math.sqrt(dx * dx + dy * dy) > 5) {
          dragging = true;
          selected = [];
        }
      }
      if (dragging) {
        rubberBand = { x1: startX, y1: startY, x2: ev.clientX, y2: ev.clientY };
        updateRubberBandSelection();
      }
    };
    const onUp = () => {
      if (!dragging) selected = [];
      rubberBand = null;
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
    e.preventDefault();
  }

  function updateRubberBandSelection() {
    if (!rubberBand || !fileListEl) return;
    const { x1, y1, x2, y2 } = rubberBand;
    const left = Math.min(x1, x2);
    const right = Math.max(x1, x2);
    const top = Math.min(y1, y2);
    const bottom = Math.max(y1, y2);
    const rows = fileListEl.querySelectorAll('.file-row:not(.parent-row)');
    const newSelected = [];
    rows.forEach((row, idx) => {
      const rect = row.getBoundingClientRect();
      if (rect.left < right && rect.right > left && rect.top < bottom && rect.bottom > top) {
        if (sortedEntries[idx]) newSelected.push(sortedEntries[idx]);
      }
    });
    selected = newSelected;
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
    try {
      await onRename(entry.path, dir + renameValue);
      await onRefresh();
    } catch {}
    renamingEntry = null;
  }

  // ── New folder ────────────────────────────────────────────────────────────

  async function handleNewFolder() {
    if (!newFolderName) { newFolderMode = false; return; }
    const base = path.replace(/[/\\]?$/, '/');
    try {
      await onMkDir(base + newFolderName);
      await onRefresh();
    } catch {}
    newFolderName = '';
    newFolderMode = false;
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
    if (e.key === 'Delete' && selected.length > 0 && !renamingEntry) {
      e.preventDefault();
      handleDelete(selected[0]);
    }
  }

  // ── Drag and drop ─────────────────────────────────────────────────────────

  function handleDragStart(e, entry) {
    e.dataTransfer.effectAllowed = 'copy';
    const isInSelection = selected.some(s => s.path === entry.path);
    const dragEntries = (isInSelection && selected.length > 1) ? selected : [entry];
    e.dataTransfer.setData('application/glideftp', JSON.stringify({
      entries: dragEntries.map(en => ({ path: en.path, name: en.name })),
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
      const { entries: dragEntries, fromSide } = JSON.parse(raw);
      if (fromSide === side) return;
      for (const { path: srcPath, name } of dragEntries) {
        const dest = path.replace(/[/\\]?$/, '/') + name;
        if (side === 'remote') QueueUpload(srcPath, dest);
        else QueueDownload(srcPath, dest);
      }
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
      <div class="path-edit-wrap">
        {#if editingPath}
          <input
            class="path-edit"
            type="text"
            bind:value={editPathValue}
            on:keydown={handlePathKeydown}
            on:input={(e) => onPathInput(e.target.value)}
            on:blur={() => { if (pathSuggestions.length === 0) confirmEditPath(); }}
            autofocus
          />
          {#if pathSuggestions.length > 0}
            <div class="path-autocomplete">
              {#each pathSuggestions as s, i}
                <div
                  class="path-sugg"
                  class:highlighted={i === highlightedSugg}
                  on:mousedown|preventDefault={() => selectSuggestion(s)}
                >
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="sugg-icon"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>
                  {s}
                </div>
              {/each}
            </div>
          {/if}
        {:else}
          <div class="path-display" title={path} on:click={startEditPath}>{path}</div>
        {/if}
      </div>
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

  {#if deleteError}
    <div class="error-bar">{deleteError}</div>
  {/if}

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

  <div
    class="file-list"
    bind:this={fileListEl}
    on:mousedown={handleListMouseDown}
  >
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

    <!-- ".." parent row -->
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

<!-- Context menu -->
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

<!-- Delete confirmation dialog -->
{#if confirmDeleteEntry}
  <div class="confirm-overlay" on:click|self={() => confirmDeleteEntry = null}>
    <div class="confirm-box" on:click|stopPropagation>
      <div class="confirm-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/></svg>
      </div>
      <div class="confirm-msg">{$t('confirmDeleteFile')}</div>
      <div class="confirm-name">{confirmDeleteEntry.name}</div>
      <div class="confirm-actions">
        <button class="confirm-del-btn" on:click={() => doDelete(confirmDeleteEntry)}>{$t('deleteConfirm')}</button>
        <button class="confirm-cancel-btn" on:click={() => confirmDeleteEntry = null}>{$t('cancel')}</button>
      </div>
    </div>
  </div>
{/if}

<!-- Rubber-band selection rectangle -->
{#if rubberBand}
  <div class="rubber-band" style="
    left: {Math.min(rubberBand.x1, rubberBand.x2)}px;
    top: {Math.min(rubberBand.y1, rubberBand.y2)}px;
    width: {Math.abs(rubberBand.x2 - rubberBand.x1)}px;
    height: {Math.abs(rubberBand.y2 - rubberBand.y1)}px;
  "></div>
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

.path-edit-wrap {
  flex: 1;
  min-width: 0;
  position: relative;
}

.path-display {
  font-size: 12px;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: text;
  border-radius: 3px;
  padding: 2px 4px;
}
.path-display:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.path-edit {
  width: 100%;
  background: var(--bg-input);
  border: 1px solid var(--accent);
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 12px;
  padding: 2px 6px;
  outline: none;
  height: 24px;
}

.path-autocomplete {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 4px;
  box-shadow: 0 4px 16px rgba(0,0,0,0.3);
  z-index: 200;
  max-height: 200px;
  overflow-y: auto;
  margin-top: 2px;
}

.path-sugg {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  font-size: 12px;
  color: var(--text-primary);
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.path-sugg:hover, .path-sugg.highlighted {
  background: var(--accent-subtle);
  color: var(--accent);
}
.sugg-icon {
  width: 12px;
  height: 12px;
  flex-shrink: 0;
  color: var(--accent);
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
.icon-btn:hover { background: var(--bg-button-hover); color: var(--text-primary); }
.icon-btn svg { width: 15px; height: 15px; }

.transfer-btn { color: var(--accent); }
.transfer-btn:hover { background: var(--accent-subtle); color: var(--accent); }

.error-bar {
  background: var(--danger);
  color: white;
  font-size: 12px;
  padding: 4px 10px;
  flex-shrink: 0;
}

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
  user-select: none;
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

.col-sortable { cursor: pointer; user-select: none; }
.col-sortable:hover { color: var(--text-primary); }
.sort-arr { color: var(--accent); font-size: 10px; }

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

.file-icon { font-size: 14px; margin-right: 4px; flex-shrink: 0; }
.file-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
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

/* ── Context menu ── */
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
  display: flex; align-items: center; gap: 8px;
  width: 100%; background: none; border: none;
  color: var(--text-primary); padding: 8px 14px;
  font-size: 13px; text-align: left; cursor: pointer;
  transition: background 0.1s;
}
.context-menu button:hover { background: var(--bg-hover); }
.context-menu button svg { width: 14px; height: 14px; flex-shrink: 0; }
.context-menu button.danger { color: var(--danger); }
.menu-sep { border: none; border-top: 1px solid var(--border); margin: 3px 0; }

/* ── Delete confirmation ── */
.confirm-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1100;
}
.confirm-box {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 24px 28px;
  min-width: 280px;
  max-width: 90vw;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  box-shadow: 0 16px 48px rgba(0,0,0,0.4);
}
.confirm-icon svg { width: 36px; height: 36px; color: var(--danger); }
.confirm-msg { font-size: 15px; font-weight: 600; color: var(--text-primary); }
.confirm-name {
  font-size: 12px;
  color: var(--text-muted);
  background: var(--bg-hover);
  border-radius: 4px;
  padding: 3px 8px;
  max-width: 240px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.confirm-actions { display: flex; gap: 8px; margin-top: 4px; }
.confirm-del-btn {
  background: var(--danger); border: none; border-radius: 5px;
  color: white; padding: 7px 18px; font-size: 13px; font-weight: 500; cursor: pointer;
}
.confirm-del-btn:hover { background: var(--danger-hover); }
.confirm-cancel-btn {
  background: var(--bg-button); border: 1px solid var(--border); border-radius: 5px;
  color: var(--text-secondary); padding: 7px 18px; font-size: 13px; cursor: pointer;
}
.confirm-cancel-btn:hover { background: var(--bg-button-hover); }

/* ── Rubber band ── */
.rubber-band {
  position: fixed;
  border: 1px solid var(--accent);
  background: var(--accent-subtle);
  pointer-events: none;
  z-index: 200;
}
</style>
