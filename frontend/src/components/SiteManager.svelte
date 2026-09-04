<script>
  import { onDestroy } from 'svelte';
  import { t } from '../i18n/index.js';
  import { GetSites, CreateSite, UpdateSite, DeleteSite, ReorderSites, BrowseSSHKey, GetKeyringStatus, ExportSitesPlainSelected, ExportSitesEncryptedSelected, OpenImportDialog, DoImportSites, TestConnection } from '../../wailsjs/go/main/App.js';
  import { connectBySite, connectBySiteWithPassword, addConnection, connections, connectionStatus, refreshRemote } from '../stores/connection.js';
  import { settings } from '../stores/settings.js';
  import { trapFocus } from '../utils/focusTrap.js';
  import { notify } from '../stores/notify.js';

  export let onClose = () => {};

  let sites = [];
  let selectedSite = null;
  let editMode = false;
  let confirmDeleteId = null;

  // Drag-and-drop reordering
  let reorderMode = false;
  let dragSiteId = null;
  let dragOverSiteId = null;

  // Keyring availability warning
  let keyringStatus = '';
  GetKeyringStatus().then(s => { keyringStatus = s; });

  // Export dialog
  let showExportDialog = false;
  let exportStep = 'choice'; // 'choice' | 'passphrase'

  // Export selection state
  let exportSelectMode = false;
  let exportSelectedIds = new Set();
  let exportPassphrase = '';
  let exportPassphraseConfirm = '';
  let exportPassphraseError = '';
  let showExportPwd = false;
  let showExportPwdConfirm = false;

  // Import passphrase dialog
  let showImportPassphrase = false;
  let importFilePath = '';
  let importPassphrase = '';
  let importPassphraseError = '';
  let showImportPwd = false;

  // Connection loading state
  let connecting = false;

  // "Test connection" button state
  let testState = 'idle'; // 'idle' | 'testing' | 'success' | 'error'
  let testErrorMsg = '';
  let testTimer = null;
  let lastTestKey = null;

  $: testKey = `${form.protocol}|${form.host}|${form.port}|${form.user}|${form.password}|${form.authType}|${form.sshKeyPath}|${form.encryption}`;
  $: if (testState !== 'testing' && lastTestKey !== null && lastTestKey !== testKey) {
    testState = 'idle';
    testErrorMsg = '';
    clearTimeout(testTimer);
  }
  $: lastTestKey = testKey;

  async function testConnection() {
    if (testState === 'testing') return;
    clearTimeout(testTimer);
    testState = 'testing';
    testErrorMsg = '';
    try {
      await TestConnection({
        protocol: form.protocol,
        host: form.host,
        port: form.port,
        user: form.user,
        password: form.password,
        encryption: form.encryption,
        authType: form.authType,
        sshKeyPath: form.sshKeyPath,
        timeoutSec: 0,
        passive: false,
      });
      testState = 'success';
    } catch (err) {
      testState = 'error';
      testErrorMsg = String(err?.message || err || '');
    }
    testTimer = setTimeout(() => { testState = 'idle'; testErrorMsg = ''; }, 3000);
  }

  onDestroy(() => clearTimeout(testTimer));

  // Keep-or-replace dialog (shown when already connected)
  let showKeepOrReplace = false;
  let pendingSiteId = null;
  let pendingConfig = null;
  let keepOrReplaceMode = 'normal'; // 'normal' | 'ask_password'

  $: canAddConnection = $connections.length < ($settings?.maxConnections ?? 3);

  // Password prompt for ask_password sites
  let showPasswordPrompt = false;
  let promptSiteId = null;
  let promptSiteName = '';
  let promptPassword = '';
  let promptError = '';
  let showPwd = false;
  let showFormPwd = false;
  let promptIsAdd = false;

  // Paste context menu (password prompt overlay)
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

  // Form field context menu (cut / copy / paste)
  let ctxMenu = null;

  function handleCtxMenu(e, field, pasteOnly = false) {
    e.preventDefault();
    const el = e.target;
    let selStart = 0, selEnd = 0, value = '';
    try { selStart = el.selectionStart ?? 0; selEnd = el.selectionEnd ?? 0; } catch {}
    try { value = el.value ?? ''; } catch {}
    ctxMenu = { x: e.clientX, y: e.clientY, field, selStart, selEnd, value, pasteOnly };
  }

  async function ctxCopy() {
    const { value, selStart, selEnd } = ctxMenu;
    ctxMenu = null;
    try { await navigator.clipboard.writeText(value.slice(selStart, selEnd)); } catch {}
  }

  async function ctxCut() {
    const { field, value, selStart, selEnd } = ctxMenu;
    ctxMenu = null;
    try { await navigator.clipboard.writeText(value.slice(selStart, selEnd)); } catch {}
    const newVal = value.slice(0, selStart) + value.slice(selEnd);
    form = { ...form, [field]: field === 'port' ? (parseInt(newVal, 10) || 0) : newVal };
  }

  async function ctxPaste() {
    const { field, value, selStart, selEnd } = ctxMenu;
    ctxMenu = null;
    try {
      const text = await navigator.clipboard.readText();
      if (text) {
        const newVal = value.slice(0, selStart) + text + value.slice(selEnd);
        form = { ...form, [field]: field === 'port' ? (parseInt(newVal, 10) || form.port) : newVal };
      }
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
      if (authType !== 'interactive' && authType !== 'key') authType = 'interactive';
      if (port === 21) port = 22;
    } else {
      if (authType === 'interactive' || authType === 'key') authType = 'normal';
      if (port === 22) port = 21;
    }
    form = { ...form, protocol: p, authType, port };
  }

  function setAuthType(a) {
    let protocol = form.protocol;
    let port = form.port;
    if (a === 'interactive' || a === 'key') {
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

  function toggleReorderMode() {
    reorderMode = !reorderMode;
    dragSiteId = null;
    dragOverSiteId = null;
  }

  function handleSiteDragStart(e, site) {
    dragSiteId = site.id;
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', site.id);
  }

  function handleSiteDragOver(e, site) {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
    dragOverSiteId = site.id !== dragSiteId ? site.id : null;
  }

  function handleSiteDragLeave(site) {
    if (dragOverSiteId === site.id) dragOverSiteId = null;
  }

  async function handleSiteDrop(e, targetSite) {
    e.preventDefault();
    dragOverSiteId = null;
    const fromId = dragSiteId;
    dragSiteId = null;
    if (!fromId || fromId === targetSite.id) return;
    const fromIdx = sites.findIndex(s => s.id === fromId);
    const toIdx = sites.findIndex(s => s.id === targetSite.id);
    if (fromIdx === -1 || toIdx === -1) return;
    const reordered = [...sites];
    const [moved] = reordered.splice(fromIdx, 1);
    reordered.splice(toIdx, 0, moved);
    sites = reordered;
    await ReorderSites(sites.map(s => s.id));
  }

  function handleSiteDragEnd() {
    dragSiteId = null;
    dragOverSiteId = null;
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

  async function duplicateSite() {
    const copy = { ...form, name: form.name + ' (copie)' };
    await CreateSite(copy);
    await loadSites();
  }

  function siteConfig(site) {
    return { protocol: site.protocol, host: site.host, port: site.port, user: site.user };
  }

  async function connectToSite(id) {
    const site = sites.find(s => s.id === id);
    if ($connectionStatus === 'connected') {
      pendingSiteId = id;
      pendingConfig = site ? siteConfig(site) : null;
      keepOrReplaceMode = site?.authType === 'ask_password' ? 'ask_password' : 'normal';
      showKeepOrReplace = true;
      return;
    }
    if (site?.authType === 'ask_password') {
      promptSiteId = id;
      promptSiteName = site.name || site.host;
      promptPassword = '';
      promptError = '';
      promptIsAdd = false;
      showPasswordPrompt = true;
      return;
    }
    connecting = true;
    try {
      await connectBySite(id, siteConfig(site));
      await refreshRemote(site?.remoteDir || '/');
      onClose();
    } catch (e) {
      notify(e?.toString() || $t('connectError'), 'error');
    } finally {
      connecting = false;
    }
  }

  async function doKeepAndAdd() {
    showKeepOrReplace = false;
    const site = sites.find(s => s.id === pendingSiteId);

    // If an identical connection (same host/port/protocol/user) is already open,
    // reconnect to it instead of opening a duplicate tab.
    const isDuplicate = $connections.some(c =>
      c.host === site?.host &&
      Number(c.port) === Number(site?.port) &&
      c.protocol === site?.protocol &&
      c.user === site?.user
    );

    if (keepOrReplaceMode === 'ask_password') {
      promptSiteId = pendingSiteId;
      promptSiteName = site?.name || site?.host || '';
      promptPassword = '';
      promptError = '';
      promptIsAdd = !isDuplicate;
      showPasswordPrompt = true;
      return;
    }

    if (isDuplicate) {
      connecting = true;
      try {
        await connectBySite(pendingSiteId, pendingConfig);
        await refreshRemote(site?.remoteDir || '/');
        onClose();
      } catch (e) {
        notify(e?.toString() || $t('connectError'), 'error');
      } finally {
        connecting = false;
      }
      return;
    }

    connecting = true;
    try {
      await addConnection(pendingSiteId, '');
      await refreshRemote(site?.remoteDir || '/');
      onClose();
    } catch (e) {
      notify(e?.toString() || $t('connectError'), 'error');
    } finally {
      connecting = false;
    }
  }

  async function doReplace() {
    showKeepOrReplace = false;
    const site = sites.find(s => s.id === pendingSiteId);
    if (keepOrReplaceMode === 'ask_password') {
      promptSiteId = pendingSiteId;
      promptSiteName = site?.name || site?.host || '';
      promptPassword = '';
      promptError = '';
      promptIsAdd = false;
      showPasswordPrompt = true;
      return;
    }
    connecting = true;
    try {
      await connectBySite(pendingSiteId, pendingConfig);
      await refreshRemote(site?.remoteDir || '/');
      onClose();
    } catch (e) {
      notify(e?.toString() || $t('connectError'), 'error');
    } finally {
      connecting = false;
    }
  }

  async function confirmPasswordConnect() {
    promptError = '';
    const site = sites.find(s => s.id === promptSiteId);
    connecting = true;
    try {
      if (promptIsAdd) {
        await addConnection(promptSiteId, promptPassword);
      } else {
        await connectBySiteWithPassword(promptSiteId, promptPassword, site ? siteConfig(site) : null);
      }
      await refreshRemote(site?.remoteDir || '/');
      showPasswordPrompt = false;
      onClose();
    } catch (e) {
      promptError = e?.toString() || 'Connection failed';
    } finally {
      connecting = false;
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

  function openExportDialog() {
    // First enter selection mode (all sites pre-selected)
    editMode = false;
    selectedSite = null;
    reorderMode = false;
    exportSelectedIds = new Set(sites.map(s => s.id));
    exportSelectMode = true;
  }

  function cancelExportSelect() {
    exportSelectMode = false;
    exportSelectedIds = new Set();
  }

  function confirmExportSelect() {
    exportSelectMode = false;
    exportStep = 'choice';
    exportPassphrase = '';
    exportPassphraseConfirm = '';
    exportPassphraseError = '';
    showExportPwd = false;
    showExportPwdConfirm = false;
    showExportDialog = true;
  }

  function toggleExportSite(id) {
    exportSelectedIds = new Set(exportSelectedIds);
    if (exportSelectedIds.has(id)) exportSelectedIds.delete(id);
    else exportSelectedIds.add(id);
  }

  function toggleExportAll() {
    if (exportSelectedIds.size === sites.length) {
      exportSelectedIds = new Set();
    } else {
      exportSelectedIds = new Set(sites.map(s => s.id));
    }
  }

  $: exportAllChecked = exportSelectedIds.size === sites.length && sites.length > 0;
  $: exportNoneChecked = exportSelectedIds.size === 0;

  async function doExportPlain() {
    showExportDialog = false;
    try {
      await ExportSitesPlainSelected([...exportSelectedIds]);
    } catch (e) {
      if (e) notify(e.toString(), 'error');
    }
  }

  function goToExportPassphrase() {
    exportStep = 'passphrase';
    exportPassphrase = '';
    exportPassphraseConfirm = '';
    exportPassphraseError = '';
  }

  async function doExportEncrypted() {
    exportPassphraseError = '';
    if (!exportPassphrase) {
      exportPassphraseError = $t('exportPassphraseEmpty');
      return;
    }
    if (exportPassphrase !== exportPassphraseConfirm) {
      exportPassphraseError = $t('exportPassphraseMismatch');
      return;
    }
    showExportDialog = false;
    try {
      await ExportSitesEncryptedSelected(exportPassphrase, [...exportSelectedIds]);
    } catch (e) {
      if (e) notify(e.toString(), 'error');
    }
  }

  async function importSites() {
    try {
      const info = await OpenImportDialog();
      if (!info || !info.path) return;
      if (info.needsPassphrase) {
        importFilePath = info.path;
        importPassphrase = '';
        importPassphraseError = '';
        showImportPwd = false;
        showImportPassphrase = true;
      } else {
        const count = await DoImportSites(info.path, '');
        if (count > 0) {
          await loadSites();
          notify(`${count} ${$t('importedCount')}`);
        }
      }
    } catch (e) {
      if (e) notify(e.toString(), 'error');
    }
  }

  async function doImportWithPassphrase() {
    importPassphraseError = '';
    try {
      const count = await DoImportSites(importFilePath, importPassphrase);
      showImportPassphrase = false;
      if (count > 0) {
        await loadSites();
        notify(`${count} ${$t('importedCount')}`);
      }
    } catch (e) {
      importPassphraseError = e?.toString() || $t('importPassphraseError');
    }
  }

  const authTypes = (t) => [
    { value: 'normal', label: t('authNormal') },
    { value: 'anonymous', label: t('authAnonymous') },
    { value: 'ask_password', label: t('authAskPassword') },
    { value: 'interactive', label: t('authInteractive') },
    { value: 'key', label: t('authSSHKey') },
  ];

  const encryptionTypes = (t) => [
    { value: 'none', label: t('encNone') },
    { value: 'tls', label: t('encTLS') },
    { value: 'ftpes', label: t('encFTPES') },
  ];
</script>

<div class="modal-backdrop" on:click|self={() => { if ($settings?.closeSiteManagerOnClickOutside !== false) onClose(); }}>
  <div class="modal" use:trapFocus>
    <div class="modal-header">
      <span class="modal-title">{$t('savedSites')}</span>
      <div class="header-actions">
        <button class="header-btn" on:click={openExportDialog} title={$t('exportSites')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
          {$t('exportSites')}
        </button>
        <button class="header-btn" on:click={importSites} title={$t('importSites')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
          {$t('importSites')}
        </button>
      </div>
      <button class="close-btn" on:click={onClose}>✕</button>
    </div>

    {#if keyringStatus === 'keyring_unavailable'}
      <div class="keyring-warning">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
        <span>{$t('keyringUnavailable')}</span>
      </div>
    {/if}

    <div class="modal-body">
      <!-- Site list -->
      <div class="site-list">
        {#if exportSelectMode}
          <!-- Select-all toggle -->
          <button class="new-site-btn" on:click={toggleExportAll}>
            <span class="export-checkbox" class:checked={exportAllChecked} class:partial={!exportAllChecked && !exportNoneChecked}>
              {#if exportAllChecked}<svg viewBox="0 0 12 12" fill="none" stroke="white" stroke-width="2.5"><polyline points="2,6 5,9 10,3"/></svg>{/if}
            </span>
            {$t('exportSelectAll')}
          </button>
        {:else}
          <div class="site-list-toolbar">
            <button class="new-site-btn" on:click={newSite}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
              {$t('newSite')}
            </button>
            <button
              type="button"
              class="reorder-toggle-btn"
              class:active={reorderMode}
              on:click={toggleReorderMode}
              title={reorderMode ? $t('reorderSitesDone') : $t('reorderSites')}
              aria-pressed={reorderMode}
            >
              {#if reorderMode}
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 12 9 17 20 6"/></svg>
              {:else}
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="7 10 12 5 17 10"/><polyline points="7 14 12 19 17 14"/></svg>
              {/if}
            </button>
          </div>
        {/if}

        {#each sites as site (site.id)}
          <div
            class="site-item"
            class:active={!exportSelectMode && selectedSite?.id === site.id}
            class:export-selectable={exportSelectMode}
            class:reorder-mode={reorderMode}
            class:drag-over={dragOverSiteId === site.id}
            draggable={reorderMode}
            on:dragstart={(e) => handleSiteDragStart(e, site)}
            on:dragover={reorderMode ? (e) => handleSiteDragOver(e, site) : null}
            on:dragleave={() => handleSiteDragLeave(site)}
            on:drop={reorderMode ? (e) => handleSiteDrop(e, site) : null}
            on:dragend={handleSiteDragEnd}
            on:click={() => { if (reorderMode) return; exportSelectMode ? toggleExportSite(site.id) : selectSite(site); }}
          >
            {#if reorderMode}
              <span class="drag-handle" title={$t('reorderSites')}>
                <svg viewBox="0 0 24 24" fill="currentColor"><circle cx="9" cy="6" r="1.4"/><circle cx="9" cy="12" r="1.4"/><circle cx="9" cy="18" r="1.4"/><circle cx="15" cy="6" r="1.4"/><circle cx="15" cy="12" r="1.4"/><circle cx="15" cy="18" r="1.4"/></svg>
              </span>
            {/if}
            {#if exportSelectMode}
              <span class="export-checkbox" class:checked={exportSelectedIds.has(site.id)}>
                {#if exportSelectedIds.has(site.id)}<svg viewBox="0 0 12 12" fill="none" stroke="white" stroke-width="2.5"><polyline points="2,6 5,9 10,3"/></svg>{/if}
              </span>
            {/if}
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
              <input type="text" bind:value={form.name} placeholder="Mon serveur FTP" on:contextmenu={(e) => handleCtxMenu(e, 'name')} />
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
                <input type="text" bind:value={form.host} placeholder="ftp.example.com" on:contextmenu={(e) => handleCtxMenu(e, 'host')} />
              </div>
              <div class="form-row" style="width: 100px">
                <label>{$t('port')}</label>
                <input type="number" bind:value={form.port} min="1" max="65535" on:contextmenu={(e) => handleCtxMenu(e, 'port')} />
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
                <input type="text" bind:value={form.user} on:contextmenu={(e) => handleCtxMenu(e, 'user')} />
              </div>
              {#if form.authType !== 'ask_password'}
                <div class="form-row">
                  <label>{$t('password')}</label>
                  <div class="pwd-input-wrap">
                    {#if showFormPwd}
                      <input class="pwd-input" type="text" bind:value={form.password} on:contextmenu={(e) => handleCtxMenu(e, 'password', true)} />
                    {:else}
                      <input class="pwd-input" type="password" bind:value={form.password} on:contextmenu={(e) => handleCtxMenu(e, 'password', true)} />
                    {/if}
                    <button type="button" class="eye-btn" on:click={() => showFormPwd = !showFormPwd} tabindex="-1">
                      {#if showFormPwd}
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                      {:else}
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
                      {/if}
                    </button>
                  </div>
                </div>
              {/if}
            {/if}

            {#if form.protocol === 'sftp' && form.authType === 'key'}
              <div class="form-row">
                <label>{$t('sshKey')}</label>
                <div class="input-with-btn">
                  <input type="text" bind:value={form.sshKeyPath} placeholder="/home/user/.ssh/id_rsa" on:contextmenu={(e) => handleCtxMenu(e, 'sshKeyPath')} />
                  <button class="browse-btn" on:click={browseSshKey}>{$t('browse')}</button>
                </div>
              </div>
            {/if}

            <div class="form-row">
              <label>{$t('remoteDir')}</label>
              <input type="text" bind:value={form.remoteDir} placeholder="/" on:contextmenu={(e) => handleCtxMenu(e, 'remoteDir')} />
              <p class="field-hint">{$t('remoteDirHint')}</p>
            </div>

            <div class="form-row">
              <label>{$t('siteNote')}</label>
              <textarea class="note-area" bind:value={form.note} rows="3" placeholder="..." on:contextmenu={(e) => handleCtxMenu(e, 'note')}></textarea>
            </div>

            <button
              type="button"
              class="btn-test"
              class:test-success={testState === 'success'}
              class:test-error={testState === 'error'}
              disabled={testState === 'testing'}
              title={testState === 'error' ? testErrorMsg : ''}
              on:click={testConnection}
            >
              {#key testState}
                <span class="btn-test-content">
                  {#if testState === 'testing'}
                    <span class="test-dots"><i></i><i></i><i></i></span>
                  {:else if testState === 'success'}
                    {$t('connectionOk')}
                  {:else if testState === 'error'}
                    {$t('connectionFailed')}
                  {:else}
                    {$t('testConnection')}
                  {/if}
                </span>
              {/key}
            </button>

            <div class="form-actions">
              <button class="btn-primary" on:click={saveSite}>{$t('save')}</button>
              <button class="btn-secondary" on:click={() => { editMode = false; form = emptyForm(); selectedSite = null; }}>{$t('close')}</button>
              {#if selectedSite}
                <button class="btn-secondary" style="margin-left: auto;" on:click={duplicateSite}>{$t('duplicateSite')}</button>
              {/if}
            </div>
          </div>

        {:else if selectedSite}
          <div class="site-view">
            <div class="site-view-header">
              <span class="view-name">{selectedSite.name}</span>
              <span class="view-sub">{selectedSite.protocol.toUpperCase()} - {selectedSite.host}:{selectedSite.port}</span>
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
              <button class="btn-primary" on:click={() => connectToSite(selectedSite.id)} disabled={connecting}>
                {#if connecting}
                  <svg class="btn-spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
                  {$t('connecting')}
                {:else}
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12h14"/><path d="m12 5 7 7-7 7"/></svg>
                  {$t('connect')}
                {/if}
              </button>
              <button class="btn-secondary" on:click={() => { editMode = true; }}>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                {$t('editSite')}
              </button>
              {#if confirmDeleteId !== selectedSite.id}
                <button class="btn-danger-outline" on:click={() => confirmDeleteId = selectedSite.id}>
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/></svg>
                  {$t('deleteSite')}
                </button>
              {/if}
            </div>
            {#if confirmDeleteId === selectedSite.id}
              <p class="confirm-text">{$t('confirmDelete')}</p>
              <div class="view-actions">
                <button class="btn-danger" on:click={() => deleteSite(selectedSite.id)}>{$t('yes')}</button>
                <button class="btn-secondary" on:click={() => confirmDeleteId = null}>{$t('no')}</button>
              </div>
            {/if}
          </div>

        {:else}
          <div class="no-selection">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M21 10H3M16 2v4M8 2v4M3 6h18v14a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V6z"/></svg>
            {#if exportSelectMode}
              <p>{$t('exportSelectTitle')}</p>
              <div class="export-select-actions">
                <button class="btn-primary" disabled={exportSelectedIds.size === 0} on:click={confirmExportSelect}>{$t('exportValidate')}</button>
                <button class="btn-secondary" on:click={cancelExportSelect}>{$t('cancel')}</button>
              </div>
            {:else}
              <p>{$t('manageSites')}</p>
            {/if}
          </div>
        {/if}
      </div>
    </div>
  </div>
</div>

<svelte:window on:click={() => { pasteMenu = null; ctxMenu = null; }} />

<!-- Export dialog -->
{#if showExportDialog}
  <div class="pwd-overlay">
    <div class="pwd-box" style="width: 420px" use:trapFocus>
      {#if exportStep === 'choice'}
        <div class="pwd-title">{$t('exportChoiceTitle')}</div>
        <div class="pwd-site">{$t('exportChoiceMsg')}</div>
        <div style="display: flex; flex-direction: column; gap: 10px; margin-top: 4px;">
          <button class="export-choice-btn" on:click={doExportPlain}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width:18px;height:18px;flex-shrink:0"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
            <div>
              <div style="font-weight:600">{$t('exportWithoutPasswords')}</div>
              <div style="font-size:11px;opacity:0.7;margin-top:2px">{$t('exportWithoutPasswordsDesc')}</div>
            </div>
          </button>
          <button class="export-choice-btn export-choice-btn--accent" on:click={goToExportPassphrase}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width:18px;height:18px;flex-shrink:0"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
            <div>
              <div style="font-weight:600">{$t('exportWithPasswords')}</div>
              <div style="font-size:11px;opacity:0.7;margin-top:2px">{$t('exportWithPasswordsDesc')}</div>
            </div>
          </button>
        </div>
        <div class="pwd-actions" style="margin-top: 4px;">
          <button class="btn-secondary" on:click={() => showExportDialog = false}>{$t('cancel')}</button>
        </div>

      {:else if exportStep === 'passphrase'}
        <div class="pwd-title">{$t('exportPassphraseTitle')}</div>
        <label class="pwd-label">{$t('exportPassphraseLabel')}</label>
        <div class="pwd-input-wrap">
          {#if showExportPwd}
            <input class="pwd-input" type="text" bind:value={exportPassphrase} autofocus
              on:keydown={(e) => { if (e.key === 'Enter') doExportEncrypted(); if (e.key === 'Escape') showExportDialog = false; }}
              on:contextmenu={(e) => { e.preventDefault(); }} />
          {:else}
            <input class="pwd-input" type="password" bind:value={exportPassphrase} autofocus
              on:keydown={(e) => { if (e.key === 'Enter') doExportEncrypted(); if (e.key === 'Escape') showExportDialog = false; }}
              on:contextmenu={(e) => { e.preventDefault(); }} />
          {/if}
          <button type="button" class="eye-btn" on:click={() => showExportPwd = !showExportPwd} tabindex="-1">
            {#if showExportPwd}
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
            {:else}
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
            {/if}
          </button>
        </div>
        <label class="pwd-label">{$t('exportPassphraseConfirmLabel')}</label>
        <div class="pwd-input-wrap">
          {#if showExportPwdConfirm}
            <input class="pwd-input" type="text" bind:value={exportPassphraseConfirm}
              on:keydown={(e) => { if (e.key === 'Enter') doExportEncrypted(); if (e.key === 'Escape') showExportDialog = false; }}
              on:contextmenu={(e) => { e.preventDefault(); }} />
          {:else}
            <input class="pwd-input" type="password" bind:value={exportPassphraseConfirm}
              on:keydown={(e) => { if (e.key === 'Enter') doExportEncrypted(); if (e.key === 'Escape') showExportDialog = false; }}
              on:contextmenu={(e) => { e.preventDefault(); }} />
          {/if}
          <button type="button" class="eye-btn" on:click={() => showExportPwdConfirm = !showExportPwdConfirm} tabindex="-1">
            {#if showExportPwdConfirm}
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
            {:else}
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
            {/if}
          </button>
        </div>
        {#if exportPassphraseError}
          <div class="pwd-error">{exportPassphraseError}</div>
        {/if}
        <div class="pwd-actions">
          <button class="btn-primary" on:click={doExportEncrypted}>{$t('exportBtn')}</button>
          <button class="btn-secondary" on:click={() => exportStep = 'choice'}>{$t('cancel')}</button>
        </div>
      {/if}
    </div>
  </div>
{/if}

<!-- Import passphrase dialog -->
{#if showImportPassphrase}
  <div class="pwd-overlay">
    <div class="pwd-box" use:trapFocus>
      <div class="pwd-title">{$t('importPassphraseTitle')}</div>
      <div class="pwd-site">{$t('importPassphraseMsg')}</div>
      <label class="pwd-label">{$t('importPassphraseLabel')}</label>
      <div class="pwd-input-wrap">
        {#if showImportPwd}
          <input class="pwd-input" type="text" bind:value={importPassphrase} autofocus
            on:keydown={(e) => { if (e.key === 'Enter') doImportWithPassphrase(); if (e.key === 'Escape') showImportPassphrase = false; }}
            on:contextmenu={(e) => { e.preventDefault(); }} />
        {:else}
          <input class="pwd-input" type="password" bind:value={importPassphrase} autofocus
            on:keydown={(e) => { if (e.key === 'Enter') doImportWithPassphrase(); if (e.key === 'Escape') showImportPassphrase = false; }}
            on:contextmenu={(e) => { e.preventDefault(); }} />
        {/if}
        <button type="button" class="eye-btn" on:click={() => showImportPwd = !showImportPwd} tabindex="-1">
          {#if showImportPwd}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
          {:else}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
          {/if}
        </button>
      </div>
      {#if importPassphraseError}
        <div class="pwd-error">{importPassphraseError}</div>
      {/if}
      <div class="pwd-actions">
        <button class="btn-primary" on:click={doImportWithPassphrase}>{$t('importSites')}</button>
        <button class="btn-secondary" on:click={() => showImportPassphrase = false}>{$t('cancel')}</button>
      </div>
    </div>
  </div>
{/if}

<!-- Form field context menu (cut / copy / paste) -->
{#if ctxMenu}
  <div
    class="paste-ctx-menu"
    style="left: {ctxMenu.x}px; top: {ctxMenu.y}px"
    on:click|stopPropagation
  >
    {#if !ctxMenu.pasteOnly && ctxMenu.selStart !== ctxMenu.selEnd}
      <button on:click={ctxCut}>{$t('cut')}</button>
      <button on:click={ctxCopy}>{$t('copy')}</button>
    {/if}
    <button on:click={ctxPaste}>{$t('paste')}</button>
  </div>
{/if}

<!-- Paste context menu (password prompt overlay) -->
{#if pasteMenu}
  <div
    class="paste-ctx-menu"
    style="left: {pasteMenu.x}px; top: {pasteMenu.y}px"
    on:click|stopPropagation
  >
    <button on:click={doPaste}>{$t('paste')}</button>
  </div>
{/if}

<!-- Keep-or-replace overlay -->
{#if showKeepOrReplace}
  <div class="pwd-overlay">
    <div class="pwd-box" style="width: 380px" use:trapFocus>
      <div class="pwd-title">{$t('keepOrReplaceTitle')}</div>
      <div class="pwd-site">{$t('keepOrReplaceMsg')}</div>
      <div class="pwd-actions" style="flex-direction: column; gap: 8px; margin-top: 4px;">
        <button
          class="btn-primary"
          style="width: 100%; justify-content: center;"
          disabled={!canAddConnection || connecting}
          title={!canAddConnection ? $t('maxConnectionsReached') : ''}
          on:click={doKeepAndAdd}
        >
          {#if connecting}
            <svg class="btn-spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
            {$t('connecting')}
          {:else}
            {$t('keepConnection')}
            {#if !canAddConnection}
              <span style="font-size: 11px; opacity: 0.7; margin-left: 4px;">({$t('maxConnectionsReached')})</span>
            {/if}
          {/if}
        </button>
        <button class="btn-secondary" style="width: 100%; justify-content: center;" disabled={connecting} on:click={doReplace}>
          {#if connecting}
            <svg class="btn-spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
            {$t('connecting')}
          {:else}
            {$t('replaceConnection')}
          {/if}
        </button>
        <button class="btn-secondary" style="width: 100%; justify-content: center;" on:click={() => showKeepOrReplace = false}>
          {$t('cancel')}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Password prompt overlay -->
{#if showPasswordPrompt}
  <div class="pwd-overlay">
    <div class="pwd-box" use:trapFocus>
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
        <button class="btn-primary" on:click={confirmPasswordConnect} disabled={connecting}>
          {#if connecting}
            <svg class="btn-spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
            {$t('connecting')}
          {:else}
            {$t('connect')}
          {/if}
        </button>
        <button class="btn-secondary" on:click={() => showPasswordPrompt = false} disabled={connecting}>{$t('cancel')}</button>
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
  width: 780px;
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

.site-list-toolbar {
  display: flex;
  border-bottom: 1px solid var(--border);
}
.site-list-toolbar .new-site-btn { flex: 1; border-bottom: none; }

.reorder-toggle-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  flex-shrink: 0;
  background: none;
  border: none;
  border-left: 1px solid var(--border);
  color: var(--text-secondary);
  cursor: pointer;
  transition: background 0.1s, color 0.1s;
}
.reorder-toggle-btn:hover { background: var(--bg-hover); color: var(--text-primary); }
.reorder-toggle-btn.active { background: var(--accent-subtle); color: var(--accent); }
.reorder-toggle-btn svg { width: 16px; height: 16px; }

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
.site-item.reorder-mode { cursor: grab; }
.site-item.reorder-mode:active { cursor: grabbing; }
.site-item.drag-over { background: var(--accent-subtle); outline: 2px dashed var(--accent); outline-offset: -2px; }

.drag-handle {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  color: var(--text-muted);
  cursor: grab;
}
.drag-handle svg { width: 14px; height: 14px; }

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

.site-detail { flex: 1; overflow-y: auto; padding: 18px; display: flex; flex-direction: column; align-items: center; justify-content: center; }

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

.btn-test {
  display: flex; align-items: center; justify-content: center;
  width: 100%; height: 34px;
  background: var(--bg-button);
  border: 1px solid var(--border); border-radius: 5px;
  color: var(--text-secondary); font-size: 13px; font-weight: 500; cursor: pointer;
  transition: background-color 0.35s ease, border-color 0.35s ease, color 0.35s ease;
  overflow: hidden;
}
.btn-test:hover:not(:disabled) { background: var(--bg-button-hover); }
.btn-test:disabled { cursor: default; }
.btn-test.test-success,
.btn-test.test-success:hover {
  background: var(--success); border-color: var(--success); color: white;
}
.btn-test.test-error,
.btn-test.test-error:hover {
  background: var(--danger); border-color: var(--danger); color: white;
}

.btn-test-content {
  display: flex; align-items: center; justify-content: center;
  animation: test-content-in 0.25s ease;
}
@keyframes test-content-in {
  from { opacity: 0; transform: translateY(3px); }
  to { opacity: 1; transform: translateY(0); }
}

.test-dots { display: flex; align-items: center; gap: 5px; height: 8px; }
.test-dots i {
  width: 6px; height: 6px; border-radius: 50%;
  background: currentColor; opacity: 0.25;
  animation: test-dot-pulse 1.1s ease-in-out infinite;
}
.test-dots i:nth-child(2) { animation-delay: 0.18s; }
.test-dots i:nth-child(3) { animation-delay: 0.36s; }
@keyframes test-dot-pulse {
  0%, 80%, 100% { opacity: 0.25; transform: scale(0.85); }
  40% { opacity: 1; transform: scale(1.15); }
}

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
  display: flex; align-items: center; gap: 6px;
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-secondary);
  padding: 7px 16px; font-size: 13px; cursor: pointer;
}
.btn-secondary svg { width: 14px; height: 14px; }
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

.site-view { display: flex; flex-direction: column; gap: 16px; align-items: center; width: 100%; }
.site-view-header { display: flex; flex-direction: column; gap: 4px; align-items: center; text-align: center; }
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

.view-actions { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; justify-content: center; }
.confirm-text { margin: 0; font-size: 12px; color: var(--danger); text-align: center; }

.no-selection {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  height: 100%; gap: 12px; color: var(--text-muted);
}
.no-selection svg { width: 48px; height: 48px; opacity: 0.3; }
.no-selection p { font-size: 13px; }

.export-select-actions { display: flex; gap: 8px; margin-top: 4px; }

/* ── Export selection checkbox ── */
.export-checkbox {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  border-radius: 3px;
  border: 2px solid var(--border);
  background: var(--bg-input);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: background 0.12s, border-color 0.12s;
}
.export-checkbox.checked {
  background: var(--accent);
  border-color: var(--accent);
}
.export-checkbox.partial {
  border-color: var(--accent);
}
.export-checkbox svg { width: 10px; height: 10px; }

.export-selectable { cursor: pointer; }
.export-selectable:hover { background: var(--bg-hover); }

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

/* ── Connect button spinner ── */
@keyframes btn-spin { to { transform: rotate(360deg); } }
.btn-spinner {
  width: 14px; height: 14px;
  animation: btn-spin 0.75s linear infinite;
  transform-origin: center;
  flex-shrink: 0;
}

/* ── Keyring warning banner ── */
.keyring-warning {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 8px 16px;
  background: color-mix(in srgb, #f59e0b 12%, var(--bg-secondary));
  border-bottom: 1px solid color-mix(in srgb, #f59e0b 35%, transparent);
  color: #b45309;
  font-size: 12px;
  line-height: 1.5;
}
.keyring-warning svg { width: 15px; height: 15px; flex-shrink: 0; margin-top: 1px; stroke: #f59e0b; }

/* ── Export choice buttons ── */
.export-choice-btn {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  width: 100%;
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 7px;
  color: var(--text-primary);
  padding: 12px 14px;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s;
}
.export-choice-btn:hover { background: var(--bg-button-hover); border-color: var(--accent); }
.export-choice-btn--accent { border-color: var(--accent); }
.export-choice-btn--accent:hover { background: var(--accent-subtle); }

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
