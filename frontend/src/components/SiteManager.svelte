<script>
  import { t } from '../i18n/index.js';
  import { GetSites, CreateSite, UpdateSite, DeleteSite, BrowseSSHKey, ExportSites, ImportSites } from '../../wailsjs/go/main/App.js';
  import { connectBySite, connectBySiteWithPassword, refreshRemote } from '../stores/connection.js';

  export let onClose = () => {};

  let sites = [];
  let selectedSite = null;
  let editMode = false;
  let confirmDeleteId = null;

  // Password prompt for ask_password sites
  let showPasswordPrompt = false;
  let promptSiteId = null;
  let promptSiteName = '';
  let promptPassword = '';
  let promptError = '';
  let showPwd = false;

  // Paste context menu
  let pasteMenu = null;

  function handlePwdContextMenu(e) {
    e.preventDefault();
    pasteMenu = { x: e.clientX, y: e.clientY };
  }

  async function doPaste() {
    pasteMenu = null;
    try {
      const text = await navigator.clipboard.readText();
      if (text) promptPassword = (promptPassword || '') + text;
    } catch {}
  }

  let form = emptyForm();

  function emptyForm() {
    return {
      name: '',
      protocol: 'ftp',
      host: '',
      port: 21,
      encryption: 'none',
      authType: 'normal',
      user: '',
      password: '',
      sshKeyPath: '',
      remoteDir: '/',
      note: '',
    };
  }

  async function loadSites() {
    sites = await GetSites();
  }

  loadSites();

  function setProtocol(p) {
    let authType = form.authType;
    let port = form.port;
    if (p === 'sftp') {
      authType = 'interactive';
      if (port === 21) port = 22;
    } else {
      if (authType === 'interactive') authType = 'normal';
      if (port === 22) port = 21;
    }
    form = { ...form, protocol: p, authType, port };
  }

  function setAuthType(a) {
    let protocol = form.protocol;
    let port = form.port;
    if (a === 'interactive') {
      protocol = 'sftp';
      if (port === 21) port = 22;
    } else {
      protocol = 'ftp';
      if (port === 22) port = 21;
    }
    form = { ...form, authType: a, protocol, port };
  }

  function selectSite(s) {
    selectedSite = s;
    form = { ...s };
    editMode = false;
  }

  function newSite() {
    selectedSite = null;
    form = emptyForm();
    editMode = true;
  }

  async function saveSite() {
    const toSave = { ...form };
    // Don't persist password when using ask_password auth
    if (toSave.authType === 'ask_password') {
      toSave.password = '';
    }
    if (selectedSite?.id) {
      await UpdateSite({ ...toSave, id: selectedSite.id });
    } else {
      await CreateSite(toSave);
    }
    await loadSites();
    editMode = false;
    selectedSite = null;
    form = emptyForm();
  }

  async function deleteSite(id) {
    await DeleteSite(id);
    confirmDeleteId = null;
    selectedSite = null;
    form = emptyForm();
    await loadSites();
  }

  function siteConfig(site) {
    return { protocol: site.protocol, host: site.host, port: site.port, user: site.user };
  }

  async function connectToSite(id) {
    const site = sites.find(s => s.id === id);
    if (site?.authType === 'ask_password') {
      promptSiteId = id;
      promptSiteName = site.name;
      promptPassword = '';
      promptError = '';
      showPasswordPrompt = true;
      return;
    }
    try {
      await connectBySite(id, siteConfig(site));
      await refreshRemote(site?.remoteDir || '/');
      onClose();
    } catch (e) {
      alert(e?.toString() || 'Connection failed');
    }
  }

  async function confirmPasswordConnect() {
    promptError = '';
    const site = sites.find(s => s.id === promptSiteId);
    try {
      await connectBySiteWithPassword(promptSiteId, promptPassword, site ? siteConfig(site) : null);
      await refreshRemote(site?.remoteDir || '/');
      showPasswordPrompt = false;
      onClose();
    } catch (e) {
      promptError = e?.toString() || 'Connection failed';
    }
  }

  let noteCopied = false;

  async function copyNote(text) {
    try {
      await navigator.clipboard.writeText(text);
      noteCopied = true;
      setTimeout(() => noteCopied = false, 1500);
    } catch {}
  }

  async function browseSshKey() {
    const path = await BrowseSSHKey();
    if (path) form = { ...form, sshKeyPath: path };
  }

  async function exportSites() {
    try {
      await ExportSites();
    } catch (e) {
      if (e) alert(e.toString());
    }
  }

  async function importSites() {
    try {
      const count = await ImportSites();
      if (count > 0) {
        await loadSites();
        alert(`${count} ${$t('importedCount')}`);
      }
    } catch (e) {
      if (e) alert(e.toString());
    }
  }

  const authTypes = (t) => [
    { value: 'normal', label: t('authNormal') },
    { value: 'anonymous', label: t('authAnonymous') },
    { value: 'account', label: t('authAccount') },
    { value: 'ask_password', label: t('authAskPassword') },
    { value: 'interactive', label: t('authInteractive') },
  ];

  const encryptionTypes = (t) => [
    { value: 'none', label: t('encNone') },
    { value: 'tls', label: t('encTLS') },
    { value: 'ftpes', label: t('encFTPES') },
  ];
</script>

<div class="modal-backdrop" on:click|self={onClose}>
  <div class="modal">
    <div class="modal-header">
      <span class="modal-title">{$t('savedSites')}</span>
      <div class="header-actions">
        <button class="header-btn" on:click={exportSites} title={$t('exportSites')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
          {$t('exportSites')}
        </button>
        <button class="header-btn" on:click={importSites} title={$t('importSites')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
          {$t('importSites')}
        </button>
      </div>
      <button class="close-btn" on:click={onClose}>✕</button>
    </div>

    <div class="modal-body">
      <!-- Site list -->
      <div class="site-list">
        <button class="new-site-btn" on:click={newSite}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
          {$t('newSite')}
        </button>

        {#each sites as site (site.id)}
          <div
            class="site-item"
            class:active={selectedSite?.id === site.id}
            on:click={() => selectSite(site)}
          >
            <span class="site-protocol">{site.protocol.toUpperCase()}</span>
            <div class="site-info">
              <span class="site-name">{site.name}</span>
              <span class="site-host">{site.host}:{site.port}</span>
            </div>
          </div>
        {/each}

        {#if sites.length === 0}
          <div class="no-sites">{$t('noTransfers')}</div>
        {/if}
      </div>

      <!-- Site form / detail -->
      <div class="site-detail">
        {#if editMode}
          <div class="form">
            <div class="form-row">
              <label>{$t('siteName')}</label>
              <input type="text" bind:value={form.name} placeholder="Mon serveur FTP" />
            </div>

            <div class="form-row protocol-row">
              <label>
                {$t('protocol')}
                <span class="info-icon" title="{$t('ftpTooltip')}&#10;&#10;{$t('sftpTooltip')}">ⓘ</span>
              </label>
              <div class="proto-select">
                <button class="proto-btn" class:active={form.protocol === 'ftp'} on:click={() => setProtocol('ftp')}>FTP</button>
                <button class="proto-btn" class:active={form.protocol === 'sftp'} on:click={() => setProtocol('sftp')}>SFTP</button>
              </div>
            </div>

            <div class="form-row-2">
              <div class="form-row">
                <label>{$t('host_label')}</label>
                <input type="text" bind:value={form.host} placeholder="ftp.example.com" />
              </div>
              <div class="form-row" style="width: 100px">
                <label>{$t('port')}</label>
                <input type="number" bind:value={form.port} min="1" max="65535" />
              </div>
            </div>

            {#if form.protocol === 'ftp'}
              <div class="form-row">
                <label>{$t('encryption')}</label>
                <select bind:value={form.encryption}>
                  {#each encryptionTypes($t) as et}
                    <option value={et.value}>{et.label}</option>
                  {/each}
                </select>
              </div>
            {/if}

            <div class="form-row">
              <label>{$t('authType')}</label>
              <select value={form.authType} on:change={(e) => setAuthType(e.target.value)}>
                {#each authTypes($t) as at}
                  <option value={at.value}>{at.label}</option>
                {/each}
              </select>
            </div>

            {#if form.authType !== 'anonymous'}
              <div class="form-row">
                <label>{$t('user')}</label>
                <input type="text" bind:value={form.user} />
              </div>
              {#if form.authType !== 'ask_password'}
                <div class="form-row">
                  <label>{$t('password')}</label>
                  <input type="password" bind:value={form.password} />
                </div>
              {/if}
            {/if}

            {#if form.protocol === 'sftp' && (form.authType === 'key' || form.authType === 'interactive')}
              <div class="form-row">
                <label>{$t('sshKey')}</label>
                <div class="input-with-btn">
                  <input type="text" bind:value={form.sshKeyPath} placeholder="/home/user/.ssh/id_rsa" />
                  <button class="browse-btn" on:click={browseSshKey}>{$t('browse')}</button>
                </div>
              </div>
            {/if}

            <div class="form-row">
              <label>{$t('remoteDir')}</label>
              <input type="text" bind:value={form.remoteDir} placeholder="/" />
              <p class="field-hint">{$t('remoteDirHint')}</p>
            </div>

            <div class="form-row">
              <label>{$t('siteNote')}</label>
              <textarea class="note-area" bind:value={form.note} rows="3" placeholder="..."></textarea>
            </div>

            <div class="form-actions">
              <button class="btn-primary" on:click={saveSite}>{$t('save')}</button>
              <button class="btn-secondary" on:click={() => { editMode = false; form = emptyForm(); selectedSite = null; }}>{$t('close')}</button>
            </div>
          </div>

        {:else if selectedSite}
          <div class="site-view">
            <div class="site-view-header">
              <span class="view-name">{selectedSite.name}</span>
              <span class="view-sub">{selectedSite.protocol.toUpperCase()} — {selectedSite.host}:{selectedSite.port}</span>
              {#if selectedSite.note}
                <div class="view-note-wrap">
                  <p class="view-note">{selectedSite.note}</p>
                  <button class="copy-note-btn" title={noteCopied ? $t('copied') : $t('copyNote')} on:click={() => copyNote(selectedSite.note)}>
                    {#if noteCopied}
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>
                    {:else}
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
                    {/if}
                  </button>
                </div>
              {/if}
            </div>
            <div class="view-actions">
              <button class="btn-primary" on:click={() => connectToSite(selectedSite.id)}>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12h14"/><path d="m12 5 7 7-7 7"/></svg>
                {$t('connect')}
              </button>
              <button class="btn-secondary" on:click={() => { editMode = true; }}>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                {$t('editSite')}
              </button>
              {#if confirmDeleteId === selectedSite.id}
                <span class="confirm-text">{$t('confirmDelete')}</span>
                <button class="btn-danger" on:click={() => deleteSite(selectedSite.id)}>{$t('yes')}</button>
                <button class="btn-secondary" on:click={() => confirmDeleteId = null}>{$t('no')}</button>
              {:else}
                <button class="btn-danger-outline" on:click={() => confirmDeleteId = selectedSite.id}>
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/></svg>
                  {$t('deleteSite')}
                </button>
              {/if}
            </div>
          </div>

        {:else}
          <div class="no-selection">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M21 10H3M16 2v4M8 2v4M3 6h18v14a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V6z"/></svg>
            <p>{$t('manageSites')}</p>
          </div>
        {/if}
      </div>
    </div>
  </div>
</div>

<svelte:window on:click={() => pasteMenu = null} />

<!-- Paste context menu -->
{#if pasteMenu}
  <div
    class="paste-ctx-menu"
    style="left: {pasteMenu.x}px; top: {pasteMenu.y}px"
    on:click|stopPropagation
  >
    <button on:click={doPaste}>{$t('paste')}</button>
  </div>
{/if}

<!-- Password prompt overlay -->
{#if showPasswordPrompt}
  <div class="pwd-overlay">
    <div class="pwd-box">
      <div class="pwd-title">{$t('passwordPromptTitle')}</div>
      <div class="pwd-site">{promptSiteName}</div>
      <label class="pwd-label">{$t('passwordPromptLabel')} {promptSiteName}</label>
      <div class="pwd-input-wrap">
        {#if showPwd}
          <input
            class="pwd-input"
            type="text"
            bind:value={promptPassword}
            autofocus
            on:keydown={(e) => { if (e.key === 'Enter') confirmPasswordConnect(); if (e.key === 'Escape') showPasswordPrompt = false; }}
            on:contextmenu={handlePwdContextMenu}
          />
        {:else}
          <input
            class="pwd-input"
            type="password"
            bind:value={promptPassword}
            autofocus
            on:keydown={(e) => { if (e.key === 'Enter') confirmPasswordConnect(); if (e.key === 'Escape') showPasswordPrompt = false; }}
            on:contextmenu={handlePwdContextMenu}
          />
        {/if}
        <button type="button" class="eye-btn" on:click={() => showPwd = !showPwd} tabindex="-1">
          {#if showPwd}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
          {:else}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
          {/if}
        </button>
      </div>
      {#if promptError}
        <div class="pwd-error">{promptError}</div>
      {/if}
      <div class="pwd-actions">
        <button class="btn-primary" on:click={confirmPasswordConnect}>{$t('connect')}</button>
        <button class="btn-secondary" on:click={() => showPasswordPrompt = false}>{$t('cancel')}</button>
      </div>
    </div>
  </div>
{/if}

<style>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 500;
}

.modal {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 10px;
  width: 700px;
  max-width: 95vw;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 60px rgba(0,0,0,0.5);
}

.modal-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
}

.modal-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
  flex: 1;
}

.header-actions {
  display: flex;
  gap: 4px;
}

.header-btn {
  display: flex;
  align-items: center;
  gap: 5px;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-secondary);
  padding: 4px 10px;
  font-size: 11px;
  cursor: pointer;
  transition: background 0.1s;
}
.header-btn:hover { background: var(--bg-button-hover); color: var(--text-primary); }
.header-btn svg { width: 13px; height: 13px; }

.close-btn {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 16px;
  padding: 0 4px;
}
.close-btn:hover { color: var(--text-primary); }

.modal-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.site-list {
  width: 220px;
  border-right: 1px solid var(--border);
  overflow-y: auto;
  flex-shrink: 0;
}

.new-site-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  background: none;
  border: none;
  border-bottom: 1px solid var(--border);
  color: var(--accent);
  padding: 10px 14px;
  font-size: 13px;
  cursor: pointer;
  font-weight: 500;
  transition: background 0.1s;
}
.new-site-btn:hover { background: var(--accent-subtle); }
.new-site-btn svg { width: 14px; height: 14px; }

.site-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  cursor: pointer;
  border-bottom: 1px solid var(--border-subtle);
  transition: background 0.1s;
}
.site-item:hover { background: var(--bg-hover); }
.site-item.active { background: var(--accent-subtle); }

.site-protocol {
  font-size: 10px;
  font-weight: 700;
  background: var(--accent);
  color: white;
  padding: 2px 5px;
  border-radius: 3px;
  flex-shrink: 0;
}

.site-info {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.site-name { font-size: 13px; font-weight: 500; color: var(--text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.site-host { font-size: 11px; color: var(--text-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.no-sites { padding: 20px; text-align: center; font-size: 12px; color: var(--text-muted); }

.site-detail { flex: 1; overflow-y: auto; padding: 18px; }

.form { display: flex; flex-direction: column; gap: 12px; }

.form-row { display: flex; flex-direction: column; gap: 4px; }

.form-row-2 { display: flex; gap: 10px; }
.form-row-2 .form-row { flex: 1; }

label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  gap: 4px;
}

.info-icon { cursor: help; color: var(--accent); font-size: 14px; }

input, select, textarea {
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  padding: 6px 10px;
  font-size: 13px;
  outline: none;
  font-family: inherit;
}
input:focus, select:focus, textarea:focus { border-color: var(--accent); }

.note-area { resize: vertical; min-height: 60px; }

.field-hint { font-size: 11px; color: var(--text-muted); margin-top: 2px; }

.proto-select { display: flex; gap: 6px; }
.proto-btn {
  flex: 1;
  padding: 6px;
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.12s;
}
.proto-btn.active { background: var(--accent); border-color: var(--accent); color: white; }
.protocol-row label { font-size: 12px; }

.input-with-btn { display: flex; gap: 6px; }
.input-with-btn input { flex: 1; }
.browse-btn {
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-secondary);
  padding: 6px 12px;
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
}
.browse-btn:hover { background: var(--bg-button-hover); }

.form-actions { display: flex; gap: 8px; margin-top: 4px; }

.btn-primary {
  display: flex; align-items: center; gap: 6px;
  background: var(--accent);
  border: none; border-radius: 5px;
  color: white; padding: 7px 16px; font-size: 13px; font-weight: 500; cursor: pointer;
  transition: background 0.12s;
}
.btn-primary:hover { background: var(--accent-hover); }

.btn-secondary {
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-secondary);
  padding: 7px 16px; font-size: 13px; cursor: pointer;
}
.btn-secondary:hover { background: var(--bg-button-hover); }

.btn-danger {
  background: var(--danger); border: none; border-radius: 5px;
  color: white; padding: 7px 16px; font-size: 13px; cursor: pointer;
}
.btn-danger-outline {
  display: flex; align-items: center; gap: 6px;
  background: transparent; border: 1px solid var(--danger); border-radius: 5px;
  color: var(--danger); padding: 7px 16px; font-size: 13px; cursor: pointer;
}
.btn-danger-outline svg { width: 14px; height: 14px; }
.btn-primary svg { width: 14px; height: 14px; }

.site-view { display: flex; flex-direction: column; gap: 16px; }
.site-view-header { display: flex; flex-direction: column; gap: 4px; }
.view-name { font-size: 18px; font-weight: 600; color: var(--text-primary); }
.view-sub { font-size: 13px; color: var(--text-muted); }
.view-note {
  margin-top: 6px;
  font-size: 13px;
  color: var(--text-secondary);
  background: var(--bg-hover);
  border-radius: 6px;
  padding: 8px 10px;
  white-space: pre-wrap;
  border-left: 3px solid var(--border);
}

.view-note-wrap {
  position: relative;
  display: inline-flex;
  width: 100%;
}
.view-note-wrap .view-note {
  flex: 1;
  padding-right: 32px;
}
.copy-note-btn {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  right: 8px;
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 2px;
  border-radius: 3px;
  display: flex;
  align-items: center;
  transition: color 0.12s;
}
.copy-note-btn:hover { color: var(--accent); }
.copy-note-btn svg { width: 14px; height: 14px; }

.view-actions { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
.confirm-text { font-size: 12px; color: var(--danger); }

.no-selection {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  height: 100%; gap: 12px; color: var(--text-muted);
}
.no-selection svg { width: 48px; height: 48px; opacity: 0.3; }
.no-selection p { font-size: 13px; }

/* ── Password prompt ──────────────────────────────────────────────────── */
.pwd-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.65);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 600;
}

.pwd-box {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 24px;
  width: 340px;
  max-width: 95vw;
  display: flex;
  flex-direction: column;
  gap: 12px;
  box-shadow: 0 16px 48px rgba(0,0,0,0.5);
}

.pwd-title { font-size: 15px; font-weight: 600; color: var(--text-primary); }
.pwd-site { font-size: 13px; color: var(--text-muted); }
.pwd-label { font-size: 12px; color: var(--text-muted); font-weight: 500; }

.pwd-input-wrap {
  display: flex;
  align-items: center;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--bg-input);
  overflow: hidden;
}
.pwd-input-wrap:focus-within { border-color: var(--accent); }

.pwd-input {
  flex: 1;
  background: transparent;
  border: none;
  color: var(--text-primary);
  padding: 7px 10px;
  font-size: 13px;
  outline: none;
  width: 100%;
}

.eye-btn {
  background: none;
  border: none;
  border-left: 1px solid var(--border);
  color: var(--text-muted);
  padding: 0 8px;
  cursor: pointer;
  display: flex;
  align-items: center;
  height: 100%;
}
.eye-btn:hover { color: var(--text-primary); }
.eye-btn svg { width: 15px; height: 15px; }

.pwd-error { font-size: 12px; color: var(--danger); }

.pwd-actions { display: flex; gap: 8px; justify-content: flex-end; }

/* ── Paste context menu ── */
.paste-ctx-menu {
  position: fixed;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 6px;
  box-shadow: 0 4px 16px rgba(0,0,0,0.3);
  z-index: 700;
  overflow: hidden;
  min-width: 100px;
}
.paste-ctx-menu button {
  display: flex; align-items: center; gap: 6px;
  width: 100%; background: none; border: none;
  color: var(--text-primary); padding: 8px 14px;
  font-size: 13px; text-align: left; cursor: pointer;
  transition: background 0.1s;
}
.paste-ctx-menu button:hover { background: var(--bg-hover); }
</style>
