<script>
  import { t } from '../i18n/index.js';
  import { GetSites, CreateSite, UpdateSite, DeleteSite, ConnectToSite, BrowseSSHKey } from '../../wailsjs/go/main/App.js';
  import { connectionStatus, refreshRemote } from '../stores/connection.js';
  import { connect } from '../stores/connection.js';

  export let onClose = () => {};

  let sites = [];
  let selectedSite = null;
  let editMode = false;
  let confirmDeleteId = null;

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
    };
  }

  async function loadSites() {
    sites = await GetSites();
  }

  $: if (form.protocol === 'sftp' && form.port === 21) form.port = 22;
  $: if (form.protocol === 'ftp' && form.port === 22) form.port = 21;

  loadSites();

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
    if (selectedSite?.id) {
      await UpdateSite({ ...form, id: selectedSite.id });
    } else {
      await CreateSite(form);
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

  async function connectToSite(id) {
    try {
      await ConnectToSite(id);
      const site = sites.find(s => s.id === id);
      if (site?.remoteDir) {
        await refreshRemote(site.remoteDir);
      } else {
        await refreshRemote('/');
      }
      onClose();
    } catch (e) {
      alert(e?.toString() || 'Connection failed');
    }
  }

  async function browseSshKey() {
    const path = await BrowseSSHKey();
    if (path) form.sshKeyPath = path;
  }

  const authTypes = (t) => [
    { value: 'anonymous', label: t('authAnonymous') },
    { value: 'account', label: t('authAccount') },
    { value: 'ask_password', label: t('authAskPassword') },
    { value: 'interactive', label: t('authInteractive') },
    { value: 'normal', label: t('authNormal') },
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
                <button class="proto-btn" class:active={form.protocol === 'ftp'} on:click={() => form.protocol = 'ftp'}>FTP</button>
                <button class="proto-btn" class:active={form.protocol === 'sftp'} on:click={() => form.protocol = 'sftp'}>SFTP</button>
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
              <select bind:value={form.authType}>
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
              <div class="form-row">
                <label>{$t('password')}</label>
                <input type="password" bind:value={form.password} />
              </div>
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
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border);
}

.modal-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

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

.site-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.site-host {
  font-size: 11px;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.no-sites {
  padding: 20px;
  text-align: center;
  font-size: 12px;
  color: var(--text-muted);
}

.site-detail {
  flex: 1;
  overflow-y: auto;
  padding: 18px;
}

.form { display: flex; flex-direction: column; gap: 12px; }

.form-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-row-2 {
  display: flex;
  gap: 10px;
}

.form-row-2 .form-row { flex: 1; }

label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  gap: 4px;
}

.info-icon {
  cursor: help;
  color: var(--accent);
  font-size: 14px;
}

input, select {
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  padding: 6px 10px;
  font-size: 13px;
  outline: none;
}

input:focus, select:focus { border-color: var(--accent); }

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

.proto-btn.active {
  background: var(--accent);
  border-color: var(--accent);
  color: white;
}

.protocol-row label { font-size: 12px; }

.input-with-btn {
  display: flex;
  gap: 6px;
}

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

.form-actions {
  display: flex;
  gap: 8px;
  margin-top: 4px;
}

.btn-primary {
  display: flex;
  align-items: center;
  gap: 6px;
  background: var(--accent);
  border: none;
  border-radius: 5px;
  color: white;
  padding: 7px 16px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.12s;
}

.btn-primary:hover { background: var(--accent-hover); }

.btn-secondary {
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-secondary);
  padding: 7px 16px;
  font-size: 13px;
  cursor: pointer;
}

.btn-secondary:hover { background: var(--bg-button-hover); }

.btn-danger {
  background: var(--danger);
  border: none;
  border-radius: 5px;
  color: white;
  padding: 7px 16px;
  font-size: 13px;
  cursor: pointer;
}

.btn-danger-outline {
  display: flex;
  align-items: center;
  gap: 6px;
  background: transparent;
  border: 1px solid var(--danger);
  border-radius: 5px;
  color: var(--danger);
  padding: 7px 16px;
  font-size: 13px;
  cursor: pointer;
}

.btn-danger-outline svg { width: 14px; height: 14px; }

.btn-primary svg { width: 14px; height: 14px; }

.site-view {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.site-view-header { display: flex; flex-direction: column; gap: 4px; }

.view-name { font-size: 18px; font-weight: 600; color: var(--text-primary); }

.view-sub { font-size: 13px; color: var(--text-muted); }

.view-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.confirm-text {
  font-size: 12px;
  color: var(--danger);
}

.no-selection {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  gap: 12px;
  color: var(--text-muted);
}

.no-selection svg {
  width: 48px;
  height: 48px;
  opacity: 0.3;
}

.no-selection p { font-size: 13px; }
</style>
