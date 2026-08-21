<script>
  import { t } from '../i18n/index.js';
  import { formatBytes } from '../stores/transfers.js';
  import { settings } from '../stores/settings.js';
  import { QueueUpload, QueueDownload, QueueUploadDir, QueueDownloadDir, LocalListDir, RemoteListDir } from '../../wailsjs/go/main/App.js';
  import { queueVisible } from '../stores/transfers.js';
  import { trapFocus } from '../utils/focusTrap.js';
  import { clipboard, localCopy, remoteCopy, remoteCopyDir } from '../stores/connection.js';

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
  export let onSearch = async (_path, _query, _recursive) => [];
  export let otherPath = '';
  export let otherEntries = [];

  let refreshing = false;

  async function handleRefresh() {
    if (refreshing) return;
    refreshing = true;
    if (treeMode) {
      await Promise.all([enterTreeMode(), new Promise(r => setTimeout(r, 500))]);
    } else {
      await Promise.all([onRefresh(), new Promise(r => setTimeout(r, 500))]);
    }
    refreshing = false;
  }

  let renamingEntry = null;
  let renameValue = '';
  let newFolderMode = false;
  let newFolderName = '';
  let contextMenu = null;
  let contextEntry = null;
  let contextIsEmpty = false;
  let dragOver = false;
  let rowDragOverPath = null; // path of the folder row currently hovered during drag
  let pasteMsg = null; // { text, ok } — green if ok, orange if !ok
  let panelEl;
  let fileListEl;

  // ── Copy / Cut / Paste ────────────────────────────────────────────────────

  $: parentPath = (() => {
    const parts = path.replace(/\/+$/, '').split('/').filter(Boolean);
    parts.pop();
    return '/' + parts.join('/') || '/';
  })();

  function cutEntries(ents) {
    if (!ents || ents.length === 0) return;
    clipboard.set({ entries: ents.map(e => ({ path: e.path, name: e.name, isDir: e.isDir })), operation: 'cut', side });
  }

  function copyEntries(ents) {
    if (!ents || ents.length === 0) return;
    clipboard.set({ entries: ents.map(e => ({ path: e.path, name: e.name, isDir: e.isDir })), operation: 'copy', side });
  }

  function showPasteMsg(text, ok) {
    pasteMsg = { text, ok };
    setTimeout(() => { pasteMsg = null; }, 4000);
  }

  // Returns a unique name like "file (copie).txt", "file (copie 1).txt", etc.
  // usedNames is a Set of names already taken (updated in-place as names are reserved).
  function uniqueCopyName(name, usedNames) {
    const dotIdx = name.indexOf('.', name.startsWith('.') ? 1 : 0);
    const base = dotIdx > 0 ? name.slice(0, dotIdx) : name;
    const ext  = dotIdx > 0 ? name.slice(dotIdx)    : '';
    const candidates = [`${base} (copie)${ext}`,
      ...[...Array(100).keys()].map(i => `${base} (copie ${i + 1})${ext}`)];
    const chosen = candidates.find(c => !usedNames.has(c));
    usedNames.add(chosen);
    return chosen;
  }

  async function pasteToFolder(destFolder) {
    const cb = $clipboard;
    if (!cb || cb.side !== side) return;
    pasteMsg = null;

    // Track names present in current dir + names generated during this batch.
    const usedNames = new Set(entries.map(e => e.name));

    // Progressive refresh: first at 1s, then every 4s while the operation runs
    let progressTimer = setTimeout(async () => {
      await onRefresh();
      progressTimer = setInterval(() => onRefresh(), 4000);
    }, 1000);
    const stopProgress = () => { clearTimeout(progressTimer); clearInterval(progressTimer); };

    try {
      for (const entry of cb.entries) {
        const base = destFolder.replace(/\/?$/, '/');
        // Same-location paste: generate a "(copie)" name instead of skipping.
        const sameLocation = entry.path === base + entry.name;
        const destName = sameLocation ? uniqueCopyName(entry.name, usedNames) : entry.name;
        const dest = base + destName;
        if (cb.operation === 'cut') {
          await onRename(entry.path, dest);
        } else if (side === 'local') {
          await localCopy(entry.path, dest);
        } else {
          if (entry.isDir) await remoteCopyDir(entry.path, dest);
          else await remoteCopy(entry.path, dest);
        }
      }
      if (cb.operation === 'cut') clipboard.set(null);
      stopProgress();
      await onRefresh();
      if (treeMode) {
        if (cb.operation === 'cut') treeRemoveEntries(cb.entries);
        await treeRefreshDir(destFolder);
      }
      showPasteMsg($t(cb.operation === 'cut' ? 'cutDone' : 'copyDone'), true);
    } catch (e) {
      stopProgress();
      await onRefresh();
      if (treeMode) await treeRefreshDir(destFolder);
      showPasteMsg(e?.message || String(e), false);
    }
  }

  // ── Intra-panel drag-and-drop ─────────────────────────────────────────────

  function handleRowDragOver(e, folderPath) {
    e.preventDefault();
    e.stopPropagation();
    rowDragOverPath = folderPath;
  }

  function handleRowDragLeave(e) {
    if (!e.currentTarget.contains(e.relatedTarget)) rowDragOverPath = null;
  }

  async function handleDropOnFolder(e, destFolder) {
    e.preventDefault();
    e.stopPropagation();
    rowDragOverPath = null;
    dragOver = false;
    const raw = e.dataTransfer.getData('application/glideftp');
    if (!raw) return;
    try {
      const { entries: dragged, fromSide } = JSON.parse(raw);
      if (fromSide === side) {
        // Same-panel: move into destFolder
        for (const { path: srcPath, name } of dragged) {
          const dest = destFolder.replace(/\/?$/, '/') + name;
          if (srcPath !== dest) await onRename(srcPath, dest);
        }
        if (treeMode) {
          treeRemoveEntries(dragged);
          await treeRefreshDir(destFolder);
        } else {
          await onRefresh();
        }
      } else {
        // Cross-panel: transfer to this specific folder
        for (const { path: srcPath, name, isDir: entryIsDir } of dragged) {
          const dest = destFolder.replace(/\/?$/, '/') + name;
          if (entryIsDir) {
            if (side === 'remote') QueueUploadDir(srcPath, dest);
            else QueueDownloadDir(srcPath, dest);
          } else {
            if (side === 'remote') QueueUpload(srcPath, dest);
            else QueueDownload(srcPath, dest);
          }
        }
        queueVisible.set(true);
      }
    } catch {}
  }

  // ── Delete confirmation ───────────────────────────────────────────────────

  let confirmDeleteEntries = null; // array when confirmation pending
  let deleteError = '';

  function handleDelete(entries) {
    closeContext();
    if (!entries || entries.length === 0) return;
    if ($settings?.confirmOnDelete) {
      confirmDeleteEntries = entries;
    } else {
      doDeleteAll(entries);
    }
  }

  async function doDeleteAll(entries) {
    deleteError = '';
    let lastError = null;
    for (const entry of entries) {
      try {
        await onDelete(entry.path);
      } catch (e) {
        lastError = e?.message || e?.toString() || 'Delete failed';
      }
    }
    if (lastError) {
      deleteError = lastError;
      setTimeout(() => { deleteError = ''; }, 4000);
    }
    if (showSearchResults) {
      selected = [];
      await refreshAfterSearchMutation();
    } else if (treeMode) {
      treeRemoveEntries(entries);
      treeSelected = null;
      selected = [];
    } else {
      await onRefresh();
    }
    confirmDeleteEntries = null;
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
    parentFocused = false;
    if (e.ctrlKey || e.metaKey) {
      selected = selected.some(s => s.path === entry.path)
        ? selected.filter(s => s.path !== entry.path)
        : [...selected, entry];
    } else if (e.shiftKey && selected.length > 0) {
      const all = showSearchResults ? sortedSearchResults : sortedEntries;
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
    const source = showSearchResults ? sortedSearchResults : sortedEntries;
    const newSelected = [];
    rows.forEach((row, idx) => {
      const rect = row.getBoundingClientRect();
      if (rect.left < right && rect.right > left && rect.top < bottom && rect.bottom > top) {
        if (source[idx]) newSelected.push(source[idx]);
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
    const newPath = dir + renameValue;
    try {
      await onRename(entry.path, newPath);
      if (showSearchResults) {
        await refreshAfterSearchMutation();
      } else if (treeMode) {
        const oldPath = entry.path;
        treeNodes = treeNodes.map(n => {
          if (n.path === oldPath) return { ...n, name: renameValue, path: newPath };
          if (n.path.startsWith(oldPath + '/')) return { ...n, path: newPath + n.path.slice(oldPath.length) };
          return n;
        });
        if (treeSelected === oldPath) treeSelected = newPath;
        const pdir = treeParentDir(oldPath);
        treeLoaded.delete(pdir);
        delete treeChildrenMap[pdir];
        if (entry.isDir) { treeLoaded.delete(oldPath); delete treeChildrenMap[oldPath]; }
      } else {
        await onRefresh();
      }
    } catch {}
    renamingEntry = null;
  }

  // ── New folder ────────────────────────────────────────────────────────────

  let newFolderTargetPath = '';
  // { clickedPath, clickedName } while asking "current folder or the
  // right-clicked folder?" after right-clicking a directory.
  let newFolderChoice = null;

  function folderBaseName(p) {
    if (!p) return '';
    const clean = p.replace(/[/\\]+$/, '');
    const sep = clean.includes('/') ? '/' : '\\';
    const idx = clean.lastIndexOf(sep);
    return (idx >= 0 ? clean.slice(idx + 1) : clean) || clean || p;
  }

  function startNewFolder(targetPath) {
    newFolderTargetPath = targetPath;
    newFolderName = '';
    newFolderMode = true;
    closeContext();
  }

  // Right-click "New folder" on a file/folder entry: a file has no
  // ambiguity (always creates in the current folder); a folder asks
  // whether to create it in the current folder or inside that folder.
  function requestNewFolderForEntry(entry) {
    if (!entry || !entry.isDir) {
      startNewFolder(path);
      return;
    }
    newFolderChoice = { clickedPath: entry.path, clickedName: entry.name };
    closeContext();
  }

  function chooseNewFolderTarget(useClicked) {
    const targetPath = useClicked ? newFolderChoice.clickedPath : path;
    newFolderChoice = null;
    startNewFolder(targetPath);
  }

  async function handleNewFolder() {
    if (!newFolderName) { newFolderMode = false; return; }
    const targetPath = newFolderTargetPath || path;
    const base = targetPath.replace(/[/\\]?$/, '/');
    try {
      await onMkDir(base + newFolderName);
      if (showSearchResults) await refreshAfterSearchMutation();
      else if (treeMode) await treeRefreshDir(targetPath);
      else await onRefresh();
    } catch {}
    newFolderName = '';
    newFolderMode = false;
    newFolderTargetPath = '';
  }

  // ── Transfer ──────────────────────────────────────────────────────────────

  function doQueueTransfer(entry, destNameOverride) {
    const dest = otherPath.replace(/[/\\]?$/, '/') + (destNameOverride || entry.name);
    if (entry.isDir) {
      if (side === 'local') QueueUploadDir(entry.path, dest);
      else QueueDownloadDir(entry.path, dest);
    } else {
      if (side === 'local') QueueUpload(entry.path, dest);
      else QueueDownload(entry.path, dest);
    }
  }

  // ── Conflict resolution ───────────────────────────────────────────────────

  // conflictState:
  //   null                              → no conflict dialog
  //   { mode:'choose', conflicts:[] }   → initial 4-button dialog
  //   { mode:'rename', entry, remaining:[], inputVal:'', index, total } → rename input
  let conflictState = null;

  function checkConflicts(entriesToTransfer) {
    const otherNames = new Set((otherEntries || []).map(e => e.name.toLowerCase()));
    return {
      conflicts: entriesToTransfer.filter(e => otherNames.has(e.name.toLowerCase())),
      nonConflicts: entriesToTransfer.filter(e => !otherNames.has(e.name.toLowerCase())),
    };
  }

  function resolveConflict(action) {
    if (!conflictState || conflictState.mode !== 'choose') return;
    const { conflicts } = conflictState;
    if (action === 'replace') {
      for (const entry of conflicts) doQueueTransfer(entry);
      queueVisible.set(true);
      conflictState = null;
    } else if (action === 'rename') {
      conflictState = {
        mode: 'rename',
        entry: conflicts[0],
        remaining: conflicts.slice(1),
        inputVal: conflicts[0].name,
        index: 0,
        total: conflicts.length,
      };
    }
  }

  function confirmRename() {
    if (!conflictState || conflictState.mode !== 'rename') return;
    const { entry, remaining, inputVal, index, total } = conflictState;
    const name = (inputVal || '').trim();
    if (name) doQueueTransfer(entry, name);
    advanceRename(remaining, index + 1, total);
  }

  function skipRename() {
    if (!conflictState || conflictState.mode !== 'rename') return;
    const { remaining, index, total } = conflictState;
    advanceRename(remaining, index + 1, total);
  }

  function advanceRename(remaining, nextIndex, total) {
    if (remaining.length > 0) {
      conflictState = {
        mode: 'rename',
        entry: remaining[0],
        remaining: remaining.slice(1),
        inputVal: remaining[0].name,
        index: nextIndex,
        total,
      };
    } else {
      queueVisible.set(true);
      conflictState = null;
    }
  }

  function transferEntries(entries) {
    if (!entries || !entries.length) return;
    const { conflicts, nonConflicts } = checkConflicts(entries);
    for (const e of nonConflicts) doQueueTransfer(e);
    if (conflicts.length > 0) {
      conflictState = { mode: 'choose', conflicts };
      if (nonConflicts.length > 0) queueVisible.set(true);
    } else {
      queueVisible.set(true);
    }
  }

  async function transferSelected() {
    if (!selected.length) return;
    transferEntries(selected);
  }

  async function transferEntry(entry) {
    closeContext();
    transferEntries([entry]);
  }

  // ── Keyboard navigation ───────────────────────────────────────────────────

  let parentFocused = false;
  $: if (path) parentFocused = false;
  $: if (path) closeSearch();
  $: dblClickUp = $settings?.doubleClickNavigateUp === true;

  function handleParentClick() {
    if (dblClickUp) {
      parentFocused = true;
    } else {
      parentFocused = false;
      onNavigateUp();
    }
  }

  function scrollEntryIntoView(idx) {
    const rows = fileListEl?.querySelectorAll('.file-row:not(.parent-row)');
    if (rows && rows[idx]) rows[idx].scrollIntoView({ block: 'nearest' });
  }

  function handlePanelKeydown(e) {
    if (renamingEntry || editingPath) return;

    if (e.ctrlKey && e.key === 'c' && selected.length > 0) {
      e.preventDefault();
      copyEntries(selected);
      return;
    }
    if (e.ctrlKey && e.key === 'x' && selected.length > 0) {
      e.preventDefault();
      cutEntries(selected);
      return;
    }
    if (e.ctrlKey && e.key === 'v' && $clipboard?.side === side) {
      e.preventDefault();
      pasteToFolder(path);
      return;
    }

    if (e.key === 'F2' && selected.length === 1) {
      e.preventDefault();
      startRename(selected[0]);
      return;
    }
    if (e.key === 'Delete' && selected.length > 0) {
      e.preventDefault();
      handleDelete(selected);
      return;
    }

    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      if (e.key === 'ArrowDown') {
        if (parentFocused) {
          parentFocused = false;
          if (sortedEntries.length > 0) {
            selected = [sortedEntries[0]];
            scrollEntryIntoView(0);
          }
        } else if (selected.length > 0) {
          const idx = sortedEntries.findIndex(en => en.path === selected[0].path);
          if (idx < sortedEntries.length - 1) {
            selected = [sortedEntries[idx + 1]];
            scrollEntryIntoView(idx + 1);
          }
        } else {
          if (sortedEntries.length > 0) {
            selected = [sortedEntries[0]];
            scrollEntryIntoView(0);
          }
        }
      } else {
        if (parentFocused) {
          // already at top
        } else if (selected.length > 0) {
          const idx = sortedEntries.findIndex(en => en.path === selected[0].path);
          if (idx > 0) {
            selected = [sortedEntries[idx - 1]];
            scrollEntryIntoView(idx - 1);
          } else {
            selected = [];
            parentFocused = true;
            fileListEl?.querySelector('.parent-row')?.scrollIntoView({ block: 'nearest' });
          }
        } else {
          parentFocused = true;
          fileListEl?.querySelector('.parent-row')?.scrollIntoView({ block: 'nearest' });
        }
      }
      panelEl?.focus({ preventScroll: true });
      return;
    }

    if (e.key === 'Enter') {
      e.preventDefault();
      if (parentFocused) {
        parentFocused = false;
        onNavigateUp();
      } else if (selected.length === 1 && selected[0].isDir) {
        onNavigate(selected[0].path);
      }
    }
  }

  // ── Search ────────────────────────────────────────────────────────────────
  // Works the same in list view and tree view: while active, the results
  // replace whichever view is currently shown. Non-recursive search filters
  // the already-loaded `entries` (instant, no backend call); recursive
  // search calls onSearch() (LocalSearch/RemoteSearch) with a debounce.

  let searchMode = false;
  let searchQuery = '';
  let searchRecursive = true;
  let searching = false;
  let searchResults = [];
  let searchDebounceTimer = null;
  let searchReqId = 0;
  let searchInputEl;

  $: searchQueryTrim = searchQuery.trim();
  $: showSearchResults = searchMode && searchQueryTrim.length > 0;
  $: nonRecursiveResults = searchQueryTrim
    ? entries.filter(e => e.name.toLowerCase().includes(searchQueryTrim.toLowerCase()))
    : [];
  $: searchDisplayResults = searchRecursive ? searchResults : nonRecursiveResults;
  $: sortedSearchResults = [...searchDisplayResults].sort((a, b) => {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
    return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
  });

  function toggleSearch() {
    if (searchMode) {
      closeSearch();
    } else {
      searchMode = true;
      setTimeout(() => searchInputEl?.focus(), 0);
    }
  }

  function closeSearch() {
    searchMode = false;
    searchQuery = '';
    searchResults = [];
    searching = false;
    clearTimeout(searchDebounceTimer);
  }

  function handleSearchKeydown(e) {
    if (e.key === 'Escape') {
      e.stopPropagation();
      closeSearch();
      panelEl?.focus({ preventScroll: true });
    }
  }

  function onSearchInput() {
    clearTimeout(searchDebounceTimer);
    if (!searchRecursive) return;
    searchDebounceTimer = setTimeout(runRecursiveSearch, 350);
  }

  async function toggleSearchRecursive() {
    searchRecursive = !searchRecursive;
    clearTimeout(searchDebounceTimer);
    if (searchRecursive && searchQueryTrim) await runRecursiveSearch();
    else searchResults = [];
  }

  async function runRecursiveSearch() {
    const q = searchQueryTrim;
    if (!q) { searchResults = []; searching = false; return; }
    const reqId = ++searchReqId;
    searching = true;
    try {
      const res = await onSearch(path, q, true);
      if (reqId === searchReqId) searchResults = res || [];
    } catch {
      if (reqId === searchReqId) searchResults = [];
    } finally {
      if (reqId === searchReqId) searching = false;
    }
  }

  // Called after a mutation (delete/rename) made while search results are shown.
  async function refreshAfterSearchMutation() {
    await onRefresh();
    if (searchRecursive && searchQueryTrim) await runRecursiveSearch();
  }

  function relativeSearchPath(entryPath) {
    const base = path.replace(/[\\/]+$/, '');
    if (!entryPath.startsWith(base)) return '';
    const rest = entryPath.slice(base.length).replace(/^[\\/]/, '');
    const sep = rest.includes('/') ? '/' : '\\';
    const idx = rest.lastIndexOf(sep);
    return idx > 0 ? rest.slice(0, idx) : '';
  }

  function handleSearchResultDblClick(entry) {
    if (entry.isDir) {
      onNavigate(entry.path);
      closeSearch();
    } else {
      transferEntry(entry);
    }
  }

  // ── Tree view ─────────────────────────────────────────────────────────────

  let treeMode = false;
  let treeNodes = [];      // [{path, name, depth, expanded, loading, leaf}]
  let treeLoaded = new Set();
  let treeChildrenMap = {};
  let treeSelected = null;

  async function fetchDirChildren(dirPath) {
    if (treeLoaded.has(dirPath)) return treeChildrenMap[dirPath] || [];
    try {
      const fn = side === 'local' ? LocalListDir : RemoteListDir;
      const list = await fn(dirPath);
      const cmp = (a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
      const dirs  = (list || []).filter(e =>  e.isDir).sort(cmp);
      const files = (list || []).filter(e => !e.isDir).sort(cmp);
      const all = [...dirs, ...files];
      treeChildrenMap[dirPath] = all;
      treeLoaded.add(dirPath);
      return all;
    } catch {
      treeChildrenMap[dirPath] = [];
      treeLoaded.add(dirPath);
      return [];
    }
  }

  async function enterTreeMode() {
    treeMode = true;
    treeNodes = [];
    treeLoaded = new Set();
    treeChildrenMap = {};
    const roots = await fetchDirChildren('/');
    treeNodes = roots.map(e => ({
      path: e.path, name: e.name, depth: 0, isDir: e.isDir,
      size: e.size, modTime: e.modTime,
      expanded: false, loading: false, leaf: !e.isDir,
    }));
    await autoExpandToPath(path);
  }

  function treeParentDir(entryPath) {
    const idx = entryPath.lastIndexOf('/');
    if (idx <= 0) return '/';
    return entryPath.substring(0, idx);
  }

  // Refresh a specific directory node in the tree without collapsing the rest.
  async function treeRefreshDir(dirPath) {
    treeLoaded.delete(dirPath);
    delete treeChildrenMap[dirPath];
    if (dirPath === '/' || dirPath === '') {
      await enterTreeMode();
      return;
    }
    const idx = treeNodes.findIndex(n => n.path === dirPath && n.isDir);
    if (idx === -1 || !treeNodes[idx].expanded) return;
    const depth = treeNodes[idx].depth;
    let end = idx + 1;
    while (end < treeNodes.length && treeNodes[end].depth > depth) end++;
    treeNodes = [...treeNodes.slice(0, idx + 1), ...treeNodes.slice(end)];
    treeNodes[idx] = { ...treeNodes[idx], expanded: false, leaf: false };
    treeNodes = [...treeNodes];
    await expandTreeNode(idx);
  }

  // Remove entries from treeNodes directly after delete/move operations.
  function treeRemoveEntries(entries) {
    treeNodes = treeNodes.filter(n =>
      !entries.some(e => e.path === n.path || (e.isDir && n.path.startsWith(e.path + '/')))
    );
    entries.forEach(e => {
      const pdir = treeParentDir(e.path);
      treeLoaded.delete(pdir);
      delete treeChildrenMap[pdir];
    });
  }

  function transferTreeFile(node) {
    const dest = otherPath.replace(/[/\\]?$/, '/') + node.name;
    if (side === 'local') QueueUpload(node.path, dest);
    else QueueDownload(node.path, dest);
    queueVisible.set(true);
  }

  async function autoExpandToPath(targetPath) {
    if (!targetPath || targetPath === '/') return;
    const parts = targetPath.split('/').filter(Boolean);
    let current = '';
    for (let i = 0; i < parts.length - 1; i++) {
      current += '/' + parts[i];
      const idx = treeNodes.findIndex(n => n.path === current);
      if (idx !== -1 && !treeNodes[idx].expanded) {
        await expandTreeNode(idx);
      }
    }
  }

  async function expandTreeNode(idx) {
    if (treeNodes[idx].loading || treeNodes[idx].leaf) return;
    treeNodes = treeNodes.map((n, i) => i === idx ? { ...n, loading: true } : n);
    const node = treeNodes[idx];
    const children = await fetchDirChildren(node.path);
    const childNodes = children.map(e => ({
      path: e.path, name: e.name, depth: node.depth + 1, isDir: e.isDir,
      size: e.size, modTime: e.modTime,
      expanded: false, loading: false, leaf: !e.isDir,
    }));
    treeNodes = [
      ...treeNodes.slice(0, idx + 1),
      ...childNodes,
      ...treeNodes.slice(idx + 1),
    ];
    treeNodes[idx] = { ...treeNodes[idx], loading: false, expanded: true, leaf: childNodes.length === 0 };
    treeNodes = [...treeNodes];
  }

  function collapseTreeNode(idx) {
    if (!treeNodes[idx].expanded) return;
    const depth = treeNodes[idx].depth;
    let end = idx + 1;
    while (end < treeNodes.length && treeNodes[end].depth > depth) end++;
    treeNodes = [
      ...treeNodes.slice(0, idx + 1),
      ...treeNodes.slice(end),
    ];
    treeNodes[idx] = { ...treeNodes[idx], expanded: false };
    treeNodes = [...treeNodes];
  }

  async function toggleTreeNode(idx) {
    const node = treeNodes[idx];
    if (node.loading || node.leaf) return;
    if (node.expanded) collapseTreeNode(idx);
    else await expandTreeNode(idx);
  }

  // ── Drag and drop ─────────────────────────────────────────────────────────

  function handleDragStart(e, entry) {
    e.dataTransfer.effectAllowed = 'copy';
    const isInSelection = selected.some(s => s.path === entry.path);
    const dragEntries = (isInSelection && selected.length > 1) ? selected : [entry];
    e.dataTransfer.setData('application/glideftp', JSON.stringify({
      entries: dragEntries.map(en => ({ path: en.path, name: en.name, isDir: en.isDir })),
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
      for (const { path: srcPath, name, isDir } of dragEntries) {
        const dest = path.replace(/[/\\]?$/, '/') + name;
        if (isDir) {
          if (side === 'remote') QueueUploadDir(srcPath, dest);
          else QueueDownloadDir(srcPath, dest);
        } else {
          if (side === 'remote') QueueUpload(srcPath, dest);
          else QueueDownload(srcPath, dest);
        }
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
      <button class="icon-btn" on:click={() => { if (!dblClickUp) onNavigateUp(); }} on:dblclick={onNavigateUp} title="Parent folder">
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
      <button
        class="icon-btn"
        class:active={treeMode}
        on:click={() => treeMode ? (treeMode = false) : enterTreeMode()}
        title={treeMode ? $t('listView') : $t('treeView')}
      >
        {#if treeMode}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/></svg>
        {:else}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="3" y1="6" x2="3" y2="18"/><line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="12" x2="15" y2="12"/><line x1="3" y1="18" x2="21" y2="18"/><line x1="9" y1="9" x2="9" y2="15"/></svg>
        {/if}
      </button>
      <button class="icon-btn" class:active={searchMode} on:click={toggleSearch} title={$t('search')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
      </button>
      <button class="icon-btn" class:spinning={refreshing} on:click={handleRefresh} title={$t('refresh')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>
      </button>
      <button class="icon-btn" on:click={() => startNewFolder(path)} title={$t('newFolder')}>
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
  {#if pasteMsg}
    <div class="paste-msg-bar" class:paste-ok={pasteMsg.ok}>{pasteMsg.text}</div>
  {/if}

  {#if newFolderMode}
    <div class="new-folder-row" on:contextmenu|stopPropagation>
      {#if newFolderTargetPath && newFolderTargetPath !== path}
        <span class="new-folder-target" title={newFolderTargetPath}>{$t('newFolderIn')} {folderBaseName(newFolderTargetPath)}</span>
      {/if}
      <input
        type="text"
        bind:value={newFolderName}
        placeholder={$t('newFolder')}
        autofocus
        on:keydown={(e) => { if (e.key === 'Enter') handleNewFolder(); if (e.key === 'Escape') { newFolderMode = false; newFolderName = ''; newFolderTargetPath = ''; } }}
      />
      <button on:click={handleNewFolder}>{$t('save')}</button>
      <button on:click={() => { newFolderMode = false; newFolderName = ''; newFolderTargetPath = ''; }}>{$t('cancel')}</button>
    </div>
  {/if}

  {#if searchMode}
    <div class="search-row" on:contextmenu|stopPropagation>
      <svg class="search-row-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
      <input
        type="text"
        bind:value={searchQuery}
        bind:this={searchInputEl}
        placeholder={$t('searchPlaceholder')}
        autofocus
        on:input={onSearchInput}
        on:keydown={handleSearchKeydown}
      />
      <button
        class="icon-btn small"
        class:active={searchRecursive}
        on:click={toggleSearchRecursive}
        title={$t('searchRecursive')}
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="3" y1="4" x2="3" y2="14"/><path d="M3 14a4 4 0 0 0 4 4h5"/><polyline points="9 15 12 18 9 21"/></svg>
      </button>
      {#if searching}
        <svg class="tree-spin search-row-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
      {/if}
      <button class="icon-btn small" on:click={closeSearch} title={$t('close')}>✕</button>
    </div>
  {/if}

  {#if showSearchResults}
    <div
      class="file-list"
      bind:this={fileListEl}
      on:mousedown={handleListMouseDown}
    >
      <div class="file-list-header" on:contextmenu|stopPropagation>
        <span class="col-name">{$t('searchResults')}{sortedSearchResults.length ? ` (${sortedSearchResults.length})` : ''}</span>
      </div>

      {#if searching && sortedSearchResults.length === 0}
        <div class="tree-loading">
          <svg class="tree-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
        </div>
      {:else if sortedSearchResults.length === 0}
        <div class="empty">{$t('noSearchResults')}</div>
      {:else}
        {#each sortedSearchResults as entry (entry.path)}
          <div
            class="file-row"
            class:selected={selected.some(s => s.path === entry.path)}
            class:is-dir={entry.isDir}
            draggable={true}
            on:click={(e) => handleClick(e, entry)}
            on:dblclick={() => handleSearchResultDblClick(entry)}
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
                {#if searchRecursive && relativeSearchPath(entry.path)}
                  <span class="search-result-path">{relativeSearchPath(entry.path)}</span>
                {/if}
              {/if}
            </span>
            <span class="col-size">{entry.isDir ? '' : formatBytes(entry.size)}</span>
            <span class="col-date">{formatDate(entry.modTime)}</span>
          </div>
        {/each}
      {/if}
    </div>
  {:else if treeMode}
    <div class="tree-list" on:contextmenu={handleBrowserContextMenu}>
      {#if treeNodes.length === 0}
        <div class="tree-loading">
          <svg class="tree-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
        </div>
      {/if}
      {#each treeNodes as node, i (node.path)}
        <div
          class="tree-row"
          class:tree-active={node.isDir && node.path === path}
          class:tree-selected={!node.isDir && node.path === treeSelected}
          class:tree-file-row={!node.isDir}
          class:drag-target={node.isDir && rowDragOverPath === node.path}
          style="padding-left: {8 + node.depth * 16}px"
          draggable={true}
          on:click={() => { treeSelected = node.path; if (node.isDir) onNavigate(node.path); }}
          on:dblclick={() => { if (!node.isDir) transferTreeFile(node); }}
          on:dragstart={(e) => { e.dataTransfer.effectAllowed = 'copy'; e.dataTransfer.setData('application/glideftp', JSON.stringify({ entries: [{ path: node.path, name: node.name, isDir: node.isDir }], fromSide: side })); }}
          on:dragover={node.isDir ? (e) => handleRowDragOver(e, node.path) : null}
          on:dragleave={node.isDir ? handleRowDragLeave : null}
          on:drop={node.isDir ? (e) => handleDropOnFolder(e, node.path) : null}
          on:contextmenu|stopPropagation={(e) => { const ent = { path: node.path, name: node.name, isDir: node.isDir, size: node.size || 0, modTime: node.modTime || '' }; treeSelected = node.path; selected = [ent]; handleFileContextMenu(e, ent); }}
          title={node.path}
        >
          {#if node.isDir}
            <button
              class="tree-toggle"
              on:click|stopPropagation={() => toggleTreeNode(i)}
              tabindex="-1"
            >
              {#if node.loading}
                <svg class="tree-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
              {:else if node.leaf}
                <span class="tree-leaf-dot">·</span>
              {:else if node.expanded}
                <svg viewBox="0 0 24 24" fill="currentColor"><path d="M7 10l5 5 5-5z"/></svg>
              {:else}
                <svg viewBox="0 0 24 24" fill="currentColor"><path d="M10 17l5-5-5-5v10z"/></svg>
              {/if}
            </button>
          {:else}
            <span class="tree-toggle-spacer"></span>
          {/if}
          <span class="file-icon">{node.isDir ? '📁' : '📄'}</span>
          <span class="tree-name">
            {#if renamingEntry?.path === node.path}
              <input
                class="tree-rename-input"
                type="text"
                bind:value={renameValue}
                on:click|stopPropagation
                on:keydown={(e) => { if (e.key === 'Enter') doRename(renamingEntry); if (e.key === 'Escape') renamingEntry = null; }}
                autofocus
              />
            {:else}
              {node.name}
            {/if}
          </span>
          {#if !node.isDir}
            <span class="tree-file-size">{formatBytes(node.size || 0)}</span>
            <button class="tree-transfer-btn" on:click|stopPropagation={() => transferTreeFile(node)} title={$t('transfer')}>
              {#if side === 'local'}
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/></svg>
              {:else}
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/></svg>
              {/if}
            </button>
          {/if}
        </div>
      {/each}
    </div>
  {:else}
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
        class:focused={parentFocused}
        class:drag-target={rowDragOverPath === '__parent__'}
        on:click={handleParentClick}
        on:dblclick={onNavigateUp}
        on:contextmenu|stopPropagation
        on:dragover={(e) => handleRowDragOver(e, '__parent__')}
        on:dragleave={handleRowDragLeave}
        on:drop={(e) => handleDropOnFolder(e, parentPath)}
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
            class:drag-target={entry.isDir && rowDragOverPath === entry.path}
            draggable={true}
            on:click={(e) => handleClick(e, entry)}
            on:dblclick={() => handleDblClick(entry)}
            on:contextmenu|stopPropagation={(e) => handleFileContextMenu(e, entry)}
            on:dragstart={(e) => handleDragStart(e, entry)}
            on:dragover={entry.isDir ? (e) => handleRowDragOver(e, entry.path) : null}
            on:dragleave={entry.isDir ? handleRowDragLeave : null}
            on:drop={entry.isDir ? (e) => handleDropOnFolder(e, entry.path) : null}
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
  {/if}
</div>

<!-- Context menu -->
{#if contextMenu}
  <div
    class="context-menu"
    style="left: {contextMenu.x}px; top: {contextMenu.y}px"
    on:click|stopPropagation
  >
    {#if contextIsEmpty}
      <button on:click={() => startNewFolder(path)}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/><line x1="12" y1="11" x2="12" y2="17"/><line x1="9" y1="14" x2="15" y2="14"/></svg>
        {$t('newFolder')}
      </button>
    {:else}
      <button on:click={() => startRename(contextEntry)}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
        {$t('rename')}
      </button>
      <button on:click={() => { const ents = selected.some(s => s.path === contextEntry?.path) ? selected : [contextEntry]; closeContext(); transferEntries(ents); }}>
        {#if side === 'local'}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/></svg>
        {:else}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/></svg>
        {/if}
        {$t('transfer')}
      </button>
      <button on:click={() => requestNewFolderForEntry(contextEntry)}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/><line x1="12" y1="11" x2="12" y2="17"/><line x1="9" y1="14" x2="15" y2="14"/></svg>
        {$t('newFolder')}
      </button>
      <hr class="menu-sep" />
      <button on:click={() => { cutEntries(selected.some(s => s.path === contextEntry?.path) ? selected : [contextEntry]); closeContext(); }}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="6" cy="20" r="2"/><circle cx="6" cy="4" r="2"/><line x1="6" y1="6" x2="6" y2="18"/><line x1="6" y1="12" x2="18" y2="4"/><line x1="6" y1="12" x2="18" y2="20"/></svg>
        {$t('cut')}
      </button>
      <button on:click={() => { copyEntries(selected.some(s => s.path === contextEntry?.path) ? selected : [contextEntry]); closeContext(); }}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
        {$t('copy')}
      </button>
      {#if $clipboard?.side === side}
        <button on:click={() => { pasteToFolder(contextEntry?.isDir ? contextEntry.path : path); closeContext(); }}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/><rect x="8" y="2" width="8" height="4" rx="1"/></svg>
          {$t('pasteHere')}
        </button>
      {/if}
      <hr class="menu-sep" />
      <button class="danger" on:click={() => handleDelete(selected.some(s => s.path === contextEntry?.path) ? selected : [contextEntry])}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/></svg>
        {$t('delete')}
      </button>
    {/if}
  </div>
{/if}

<!-- New folder: choose current folder vs the right-clicked folder -->
{#if newFolderChoice}
  <div class="confirm-overlay" on:click|self={() => newFolderChoice = null}>
    <div class="confirm-box" on:click|stopPropagation use:trapFocus>
      <div class="confirm-icon confirm-icon-neutral">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/><line x1="12" y1="11" x2="12" y2="17"/><line x1="9" y1="14" x2="15" y2="14"/></svg>
      </div>
      <div class="confirm-msg">{$t('newFolderWhereTitle')}</div>
      <div class="conflict-actions">
        <button class="conflict-replace-btn" on:click={() => chooseNewFolderTarget(false)}>
          {$t('newFolderCurrentDir')} ({folderBaseName(path)})
        </button>
        <button class="conflict-rename-btn" on:click={() => chooseNewFolderTarget(true)}>
          {$t('newFolderClickedDir')} ({newFolderChoice.clickedName})
        </button>
        <button class="confirm-cancel-btn" on:click={() => newFolderChoice = null}>{$t('cancel')}</button>
      </div>
    </div>
  </div>
{/if}

<!-- Delete confirmation dialog -->
{#if confirmDeleteEntries}
  <div class="confirm-overlay" on:click|self={() => confirmDeleteEntries = null}>
    <div class="confirm-box" on:click|stopPropagation use:trapFocus>
      <div class="confirm-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/></svg>
      </div>
      <div class="confirm-msg">{$t('confirmDeleteFile')}</div>
      <div class="confirm-name">
        {#if confirmDeleteEntries.length === 1}
          {confirmDeleteEntries[0].name}
        {:else}
          {confirmDeleteEntries.length} {$t('items')}
        {/if}
      </div>
      <div class="confirm-actions">
        <button class="confirm-del-btn" on:click={() => doDeleteAll(confirmDeleteEntries)}>{$t('deleteConfirm')}</button>
        <button class="confirm-cancel-btn" on:click={() => confirmDeleteEntries = null}>{$t('cancel')}</button>
      </div>
    </div>
  </div>
{/if}

<!-- Conflict resolution dialog -->
{#if conflictState}
  <div class="confirm-overlay" on:click|self={() => conflictState = null}>
    <div class="confirm-box" on:click|stopPropagation use:trapFocus>

      {#if conflictState.mode === 'choose'}
        <div class="conflict-warn-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
        </div>
        <div class="confirm-msg">{$t('conflictTitle')}</div>
        <div class="confirm-name">
          {#if conflictState.conflicts.length === 1}
            {conflictState.conflicts[0].name}
          {:else}
            {conflictState.conflicts.length} {$t('items')}
          {/if}
        </div>
        <div class="conflict-actions">
          <button class="conflict-replace-btn" on:click={() => resolveConflict('replace')}>{$t('conflictReplace')}</button>
          <button class="conflict-rename-btn" on:click={() => resolveConflict('rename')}>{$t('conflictRenameHost')}</button>
          <button class="conflict-rename-btn" on:click={() => resolveConflict('rename')}>{$t('conflictRenameServer')}</button>
          <button class="confirm-cancel-btn" on:click={() => conflictState = null}>{$t('cancel')}</button>
        </div>

      {:else if conflictState.mode === 'rename'}
        <div class="confirm-msg">{$t('conflictRenameTitle')}</div>
        {#if conflictState.total > 1}
          <div class="conflict-progress">{conflictState.index + 1} / {conflictState.total}</div>
        {/if}
        {#key conflictState.index}
          <input
            class="conflict-rename-input"
            type="text"
            bind:value={conflictState.inputVal}
            on:keydown={(e) => { if (e.key === 'Enter') confirmRename(); if (e.key === 'Escape') conflictState = null; }}
            autofocus
          />
        {/key}
        <div class="confirm-actions">
          <button class="conflict-replace-btn" on:click={confirmRename}>{$t('save')}</button>
          <button class="confirm-cancel-btn" on:click={skipRename}>{$t('conflictSkip')}</button>
        </div>
      {/if}

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

.file-row.drag-target,
.tree-row.drag-target {
  background: var(--accent-subtle);
  outline: 1px solid var(--accent);
  outline-offset: -1px;
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

.paste-msg-bar {
  font-size: 12px;
  padding: 4px 10px;
  flex-shrink: 0;
  background: #7a3a00;
  color: #ffd0a0;
}
.paste-msg-bar.paste-ok {
  background: #1a4a1a;
  color: #a0e8a0;
}

.new-folder-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.new-folder-target {
  color: var(--accent);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 40%;
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

.search-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.search-row-icon {
  width: 14px;
  height: 14px;
  color: var(--text-muted);
  flex-shrink: 0;
}
.search-row input {
  flex: 1;
  background: var(--bg-input);
  border: 1px solid var(--accent);
  border-radius: 4px;
  color: var(--text-primary);
  padding: 3px 8px;
  font-size: 12px;
  outline: none;
  min-width: 0;
}
.icon-btn.small {
  width: 22px;
  height: 22px;
  flex-shrink: 0;
}
.icon-btn.small svg { width: 13px; height: 13px; }
.search-row-spin {
  width: 14px;
  height: 14px;
  color: var(--accent);
  flex-shrink: 0;
}
.search-result-path {
  color: var(--text-muted);
  font-size: 11px;
  margin-left: 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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
.parent-row.focused { background: var(--accent-subtle); color: var(--accent); }

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
.confirm-icon-neutral svg { color: var(--accent); }
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

/* ── Refresh animation ── */
.icon-btn.spinning svg {
  animation: spin-once 0.5s linear;
  transform-origin: center;
}
@keyframes spin-once {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* ── Conflict resolution ── */
.conflict-warn-icon svg { width: 36px; height: 36px; color: #f59e0b; }
.conflict-progress {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: -4px;
}
.conflict-rename-input {
  width: 100%;
  background: var(--bg-input);
  border: 1px solid var(--accent);
  border-radius: 5px;
  color: var(--text-primary);
  font-size: 13px;
  padding: 6px 10px;
  outline: none;
  box-sizing: border-box;
}
.conflict-actions { display: flex; gap: 6px; flex-wrap: wrap; justify-content: center; margin-top: 4px; }
.conflict-replace-btn {
  background: var(--accent); border: none; border-radius: 5px;
  color: white; padding: 7px 14px; font-size: 13px; font-weight: 500; cursor: pointer;
}
.conflict-replace-btn:hover { background: var(--accent-hover); }
.conflict-rename-btn {
  background: var(--bg-button); border: 1px solid var(--border); border-radius: 5px;
  color: var(--text-secondary); padding: 7px 14px; font-size: 13px; cursor: pointer;
}
.conflict-rename-btn:hover { background: var(--bg-button-hover); }

/* ── Tree view ── */
.tree-list {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  user-select: none;
}

.tree-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px;
  color: var(--text-muted);
}

.tree-row {
  display: flex;
  align-items: center;
  gap: 4px;
  height: 26px;
  padding-right: 10px;
  font-size: 13px;
  cursor: pointer;
  border-bottom: 1px solid var(--border-subtle);
  color: var(--text-primary);
  transition: background 0.08s;
  white-space: nowrap;
  overflow: hidden;
}
.tree-row:hover { background: var(--bg-hover); }
.tree-row.tree-active {
  background: var(--accent-subtle);
  color: var(--accent);
  font-weight: 600;
}

.tree-toggle {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 0;
  border-radius: 3px;
  transition: color 0.1s, background 0.1s;
}
.tree-toggle:hover { color: var(--text-primary); background: var(--bg-button-hover); }
.tree-toggle svg { width: 14px; height: 14px; }
.tree-active .tree-toggle { color: var(--accent); }

.tree-leaf-dot {
  font-size: 16px;
  line-height: 1;
  color: var(--border);
}

.tree-name {
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.tree-rename-input {
  background: var(--bg-input);
  border: 1px solid var(--accent);
  border-radius: 3px;
  color: var(--text-primary);
  font-size: 13px;
  padding: 1px 5px;
  outline: none;
  width: 160px;
  max-width: 100%;
}

.tree-toggle-spacer {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}

.tree-file-row { color: var(--text-secondary); }
.tree-file-row:hover { color: var(--text-primary); }
.tree-file-row:hover .tree-transfer-btn { opacity: 1; }
.tree-selected { background: var(--accent-subtle); color: var(--accent); }
.tree-selected .tree-transfer-btn { opacity: 1; }

.tree-file-size {
  font-size: 11px;
  color: var(--text-muted);
  flex-shrink: 0;
  margin-left: auto;
  padding-right: 4px;
}

.tree-transfer-btn {
  flex-shrink: 0;
  width: 22px;
  height: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  color: var(--accent);
  cursor: pointer;
  border-radius: 3px;
  padding: 0;
  opacity: 0;
  transition: opacity 0.15s, background 0.1s;
}
.tree-transfer-btn svg { width: 14px; height: 14px; }
.tree-transfer-btn:hover { background: var(--accent-subtle); }

.tree-spin {
  animation: tree-spin-anim 0.8s linear infinite;
  transform-origin: center;
}
@keyframes tree-spin-anim {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.icon-btn.active {
  color: var(--accent);
  background: var(--accent-subtle);
}

/* ── Rubber band ── */
.rubber-band {
  position: fixed;
  border: 1px solid var(--accent);
  background: var(--accent-subtle);
  pointer-events: none;
  z-index: 200;
}
</style>
