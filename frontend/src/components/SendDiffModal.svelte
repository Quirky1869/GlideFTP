<script>
  // v1.8.0 - "Envoyer les différences": the entries of a panel marked by the
  // comparison (yellow / green / red), each one can be unchecked before they
  // go through FileBrowser's transferEntries (usual conflict check). Check
  // boxes are buttons (hidden native inputs don't work on WebKit-GTK).
  import { t } from '../i18n/index.js';
  import { trapFocus } from '../utils/focusTrap.js';
  import { formatBytes } from '../stores/transfers.js';

  export let side;            // panel the entries come from
  export let items;           // [{ entry, mark, title }]
  export let onSend;          // (entries) => void
  export let onClose;

  let checked = new Set(items.map(i => i.entry.path));

  function toggle(path) {
    if (checked.has(path)) checked.delete(path);
    else checked.add(path);
    checked = checked;
  }

  function toggleAll() {
    checked = checked.size === items.length ? new Set() : new Set(items.map(i => i.entry.path));
  }

  $: chosen = items.filter(i => checked.has(i.entry.path));
  // Folder sizes are never computed automatically (they would flood the
  // server with listings): they are counted apart.
  $: bytes = chosen.reduce((n, i) => n + (i.entry.isDir ? 0 : (i.entry.size || 0)), 0);
  $: dirCount = chosen.filter(i => i.entry.isDir).length;
  $: sendLabel = (() => {
    const parts = [];
    if (chosen.length > dirCount) parts.push(formatBytes(bytes));
    if (dirCount) parts.push((dirCount === 1 ? $t('sendDiffOneDir') : $t('sendDiffDirs')).replace('{n}', dirCount));
    const what = side === 'local' ? $t('sendDiffSend') : $t('sendDiffDownload');
    return parts.length ? `${what} (${parts.join(' + ')})` : what;
  })();

  function send() {
    if (!chosen.length) return;
    onSend(chosen.map(i => i.entry));
  }

  function focusOnOpen(node) {
    requestAnimationFrame(() => node.focus());
  }
</script>

<div class="sd-overlay" on:click|self={onClose}>
  <div
    class="sd-box"
    role="dialog"
    aria-modal="true"
    use:trapFocus
    on:keydown|stopPropagation={(e) => { if (e.key === 'Escape') onClose(); }}
  >
    <div class="sd-title">{side === 'local' ? $t('sendDiffTitleUpload') : $t('sendDiffTitleDownload')}</div>

    <button class="sd-row sd-all" on:click={toggleAll}>
      <span class="sd-check" class:on={checked.size === items.length} class:partial={checked.size > 0 && checked.size < items.length}></span>
      <span class="sd-name">{$t('sendDiffAll')}</span>
      <span class="sd-count">{checked.size} / {items.length}</span>
    </button>

    <div class="sd-list">
      {#each items as item (item.entry.path)}
        <button class="sd-row" class:off={!checked.has(item.entry.path)} on:click={() => toggle(item.entry.path)} title={item.title}>
          <span class="sd-check" class:on={checked.has(item.entry.path)}></span>
          <span class="sd-dot {item.mark.kind}"></span>
          <span class="sd-icon">{#if item.entry.isDir}📁{:else}📄{/if}</span>
          <span class="sd-name" class:is-dir={item.entry.isDir}>{item.entry.name}</span>
          <span class="sd-size">{item.entry.isDir ? $t('sendDiffFolder') : formatBytes(item.entry.size || 0)}</span>
        </button>
      {/each}
    </div>

    <div class="sd-actions">
      <button class="sd-cancel" on:click={onClose}>{$t('cancel')}</button>
      <button class="sd-ok" disabled={!chosen.length} use:focusOnOpen on:click={send}>{sendLabel}</button>
    </div>
  </div>
</div>

<style>
.sd-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 4800;
}
.sd-box {
  background: linear-gradient(var(--bg-secondary), var(--bg-secondary)) padding-box,
              var(--accent-gradient) border-box;
  border: 2px solid transparent;
  border-radius: 10px;
  padding: 22px 26px;
  width: 560px;
  max-width: 95vw;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  gap: 12px;
  box-shadow: 0 16px 48px rgba(0,0,0,0.5);
}
.sd-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}
.sd-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow-y: auto;
  min-height: 0;
  max-height: 50vh;
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 4px;
  background: var(--bg-primary);
}
.sd-row {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  text-align: left;
  background: none;
  border: 1px solid transparent;
  border-radius: 5px;
  padding: 6px 8px;
  color: var(--text-primary);
  font-size: 13px;
  cursor: pointer;
}
.sd-row:hover { background: var(--bg-hover); }
.sd-row:focus-visible { outline: none; border-color: var(--accent); }
.sd-row.off .sd-name, .sd-row.off .sd-size, .sd-row.off .sd-icon { opacity: 0.5; }
.sd-all {
  background: var(--bg-button);
  border-color: var(--border);
  font-weight: 500;
}
.sd-all:hover { background: var(--bg-button-hover); }
.sd-check {
  width: 15px;
  height: 15px;
  flex-shrink: 0;
  border: 2px solid var(--text-muted);
  border-radius: 4px;
  box-sizing: border-box;
  position: relative;
}
.sd-check.on, .sd-check.partial {
  border-color: transparent;
  background: var(--accent-bg) border-box;
}
.sd-check.on::after {
  content: '';
  position: absolute;
  left: 3.5px;
  top: 0.5px;
  width: 4px;
  height: 8px;
  border: solid white;
  border-width: 0 2px 2px 0;
  transform: rotate(45deg);
}
.sd-check.partial::after {
  content: '';
  position: absolute;
  left: 2.5px;
  right: 2.5px;
  top: 4.5px;
  height: 2px;
  background: white;
}
.sd-dot {
  width: 9px;
  height: 9px;
  flex-shrink: 0;
  border-radius: 50%;
}
.sd-dot.only  { background: var(--cmp-only); }
.sd-dot.newer { background: var(--cmp-newer); }
.sd-dot.size  { background: var(--cmp-size); }
.sd-icon { flex-shrink: 0; font-size: 13px; }
.sd-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sd-name.is-dir { font-weight: 500; }
.sd-size, .sd-count {
  flex-shrink: 0;
  color: var(--text-secondary);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}
.sd-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 2px;
}
.sd-cancel, .sd-ok {
  border-radius: 5px;
  padding: 7px 16px;
  font-size: 13px;
  cursor: pointer;
}
.sd-cancel {
  background: var(--bg-button);
  border: 1px solid var(--border);
  color: var(--text-secondary);
}
.sd-cancel:hover { background: var(--bg-button-hover); }
.sd-ok {
  background: var(--accent-bg) border-box;
  border: 1px solid transparent;
  color: white;
  font-weight: 500;
}
.sd-ok:hover:not(:disabled) { background: var(--accent-hover-bg) border-box; }
.sd-ok:disabled { opacity: 0.5; cursor: default; }
</style>
