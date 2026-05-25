<script>
  import { onMount, onDestroy } from 'svelte';
  import { get } from 'svelte/store';
  import { t } from '../i18n/index.js';
  import { connectionStatus, connectionError, connect, disconnect, activeConnectionConfig, connections, activeConnectionId, addConnectionAdHoc, closeTab } from '../stores/connection.js';
  import { settings } from '../stores/settings.js';
  import { trapFocus } from '../utils/focusTrap.js';

  export let onMultiDisconnect = null;

  let host = '';
  let user = '';
  let password = '';
  let port = 21;
  let protocol = 'ftp';
  let sshKeyPath = '';
  let authType = 'normal';
  let encryption = 'none';

  let showError = false;
  let errorMsg = '';

  let quickConnectMode = false;
  let pendingQuickCfg = null;
  let quickConnectDialog = null; // { cfg } when keep-or-replace dialog is visible
  let hostError = false;
  let connBarRef;
  let hostInputRef;

  $: isConnected = $connectionStatus === 'connected';
  $: isConnecting = $connectionStatus === 'connecting';

  $: if (!isConnected && protocol === 'sftp' && port === 21) port = 22;
  $: if (!isConnected && protocol === 'ftp' && port === 22) port = 21;
  $: if (quickConnectMode && protocol === 'sftp' && port === 21) port = 22;
  $: if (quickConnectMode && protocol === 'ftp' && port === 22) port = 21;

  // Fill fields from active connection config when connected (not in quick connect mode)
  $: if (isConnected && !quickConnectMode && $activeConnectionConfig) {
    protocol = $activeConnectionConfig.protocol || 'ftp';
    host = $activeConnectionConfig.host || '';
    user = $activeConnectionConfig.user || '';
    port = $activeConnectionConfig.port || 21;
    password = '············';
  }

  // After multi-disconnect with pending quick connect config, restore typed values
  $: if (!isConnected && pendingQuickCfg) {
    const cfg = pendingQuickCfg;
    pendingQuickCfg = null;
    protocol = cfg.protocol;
    host = cfg.host;
    user = cfg.user;
    port = cfg.port;
    password = cfg.password;
    authType = cfg.authType;
    sshKeyPath = cfg.sshKeyPath;
    encryption = cfg.encryption;
  }

  function enterQuickConnect() {
    quickConnectMode = true;
    protocol = 'ftp';
    host = '';
    user = '';
    password = '';
    port = 21;
    setTimeout(() => hostInputRef?.focus(), 0);
  }

  function exitQuickConnect() {
    quickConnectMode = false;
    if (isConnected && $activeConnectionConfig) {
      protocol = $activeConnectionConfig.protocol || 'ftp';
      host = $activeConnectionConfig.host || '';
      user = $activeConnectionConfig.user || '';
      port = $activeConnectionConfig.port || 21;
      password = '············';
    }
  }

  async function handleQuickConnect() {
    if (!host.trim()) {
      hostError = true;
      setTimeout(() => { hostError = false; }, 1300);
      return;
    }
    if ($connections.length > 1 && onMultiDisconnect) {
      // Multiple connections: preserve typed values, trigger disconnect-all dialog
      pendingQuickCfg = { protocol, host, port: Number(port), user, password, authType, sshKeyPath, encryption };
      quickConnectMode = false;
      onMultiDisconnect();
      return;
    }
    // Single connection: capture values, exit quick connect mode, show keep-or-replace dialog
    const cfg = { protocol, host, port: Number(port), user, password, authType, sshKeyPath, encryption };
    quickConnectMode = false;
    quickConnectDialog = { cfg };
  }

  async function doQuickReplace(cfg) {
    quickConnectDialog = null;
    const oldId = get(activeConnectionId);
    showError = false;
    try {
      await addConnectionAdHoc(cfg);
      if (oldId) await closeTab(oldId);
    } catch (e) {
      errorMsg = e?.toString() || 'Connection failed';
      showError = true;
      setTimeout(() => showError = false, 5000);
    }
  }

  async function doQuickOpenTab(cfg) {
    quickConnectDialog = null;
    showError = false;
    try {
      await addConnectionAdHoc(cfg);
    } catch (e) {
      errorMsg = e?.toString() || 'Connection failed';
      showError = true;
      setTimeout(() => showError = false, 5000);
    }
  }

  async function handleConnect() {
    if (isConnected) {
      if (quickConnectMode) quickConnectMode = false;
      if ($connections.length > 1 && onMultiDisconnect) {
        onMultiDisconnect();
        return;
      }
      password = '';
      await disconnect();
      return;
    }
    pendingQuickCfg = null;
    showError = false;
    try {
      await connect({ protocol, host, port: Number(port), user, password, authType, sshKeyPath, encryption });
    } catch (e) {
      errorMsg = e?.toString() || 'Connection failed';
      showError = true;
      setTimeout(() => showError = false, 5000);
    }
  }

  function handleKeydown(e) {
    if (e.key === 'Enter') {
      if (quickConnectMode) handleQuickConnect();
      else handleConnect();
    }
  }

  function stepPort(delta) {
    let next = (Number(port) || 21) + delta;
    if (protocol === 'ftp' && next === 22) next += delta > 0 ? 1 : -1;
    port = Math.max(1, Math.min(65535, next));
  }

  function handleDocMousedown(e) {
    if (!connBarRef?.contains(e.target)) {
      if (quickConnectMode) exitQuickConnect();
      else if (quickConnectDialog) quickConnectDialog = null;
    }
  }

  onMount(() => document.addEventListener('mousedown', handleDocMousedown));
  onDestroy(() => document.removeEventListener('mousedown', handleDocMousedown));
</script>

<div class="conn-bar" bind:this={connBarRef}>
  <div class="conn-fields">
    <div class="field-group">
      <label>{$t('protocol')}</label>
      <select bind:value={protocol} disabled={(isConnected && !quickConnectMode) || isConnecting}>
        <option value="ftp">FTP</option>
        <option value="sftp">SFTP</option>
      </select>
    </div>

    <div class="field-group flex1">
      <label>{$t('host')}</label>
      <input
        type="text"
        bind:value={host}
        bind:this={hostInputRef}
        placeholder="ftp.example.com"
        disabled={(isConnected && !quickConnectMode) || isConnecting}
        class:host-error={hostError}
        on:keydown={handleKeydown}
      />
    </div>

    <div class="field-group">
      <label>{$t('user')}</label>
      <input
        type="text"
        bind:value={user}
        placeholder="user"
        disabled={(isConnected && !quickConnectMode) || isConnecting}
        on:keydown={handleKeydown}
      />
    </div>

    <div class="field-group">
      <label>{$t('password')}</label>
      <input
        type="password"
        bind:value={password}
        placeholder="••••••••"
        disabled={(isConnected && !quickConnectMode) || isConnecting}
        on:keydown={handleKeydown}
      />
    </div>

    <div class="field-group port-group">
      <label>{$t('port')}</label>
      <div class="port-stepper" class:disabled={(isConnected && !quickConnectMode) || isConnecting}>
        <button class="port-step-btn" on:click={() => stepPort(-1)} disabled={(isConnected && !quickConnectMode) || isConnecting} tabindex="-1">−</button>
        <input
          type="number"
          bind:value={port}
          min="1"
          max="65535"
          disabled={(isConnected && !quickConnectMode) || isConnecting}
          on:keydown={handleKeydown}
        />
        <button class="port-step-btn" on:click={() => stepPort(1)} disabled={(isConnected && !quickConnectMode) || isConnecting} tabindex="-1">+</button>
      </div>
    </div>

    {#if isConnected}
      <button
        class="btn-quick"
        class:active={quickConnectMode}
        on:click={quickConnectMode ? handleQuickConnect : enterQuickConnect}
        disabled={isConnecting}
        title={quickConnectMode ? $t('connect') : $t('quickConnect')}
      >
        {#if quickConnectMode}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12h14"/><path d="m12 5 7 7-7 7"/></svg>
        {:else}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 19V5"/><path d="m5 12 7-7 7 7"/></svg>
        {/if}
      </button>
    {/if}

    <button
      class="btn-connect"
      class:connected={isConnected}
      class:connecting={isConnecting}
      on:click={handleConnect}
      disabled={isConnecting || (!isConnected && !host)}
      title={isConnected ? $t('disconnect') : $t('connect')}
    >
      {#if isConnecting}
        <span class="spinner"></span>
      {:else if isConnected}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18.36 6.64a9 9 0 1 1-12.73 0"/><line x1="12" y1="2" x2="12" y2="12"/></svg>
      {:else}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12h14"/><path d="m12 5 7 7-7 7"/></svg>
      {/if}
    </button>
  </div>

  {#if showError}
    <div class="conn-error">{errorMsg}</div>
  {/if}

  {#if quickConnectDialog}
    <div class="quick-dialog" use:trapFocus>
      <p class="quick-dialog-title">{$t('keepOrReplaceTitle')}</p>
      <p class="quick-dialog-host">{quickConnectDialog.cfg.host}</p>
      <div class="quick-dialog-btns">
        {#if !$settings || $connections.length < ($settings.maxConnections || 3)}
          <button class="qdlg-btn" on:click={() => doQuickOpenTab(quickConnectDialog.cfg)}>
            {$t('keepConnection')}
          </button>
        {/if}
        <button class="qdlg-btn" on:click={() => doQuickReplace(quickConnectDialog.cfg)}>
          {$t('replaceConnection')}
        </button>
        <button class="qdlg-btn cancel" on:click={() => quickConnectDialog = null}>
          {$t('close')}
        </button>
      </div>
    </div>
  {/if}
</div>

<style>
.conn-bar {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px 12px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border);
  width: 100%;
}

.conn-fields {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  flex-wrap: nowrap;
}

.field-group {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.flex1 {
  flex: 1;
  min-width: 150px;
}

label {
  font-size: 11px;
  color: var(--text-muted);
  font-weight: 500;
  white-space: nowrap;
}

input, select {
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  padding: 5px 8px;
  font-size: 13px;
  outline: none;
  transition: border-color 0.15s;
  height: 30px;
}

input:focus, select:focus {
  border-color: var(--accent);
}

input:disabled, select:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.host-error, .host-error:focus {
  animation: host-error-blink 1.2s ease-in-out forwards;
}

@keyframes host-error-blink {
  0%     { border-color: var(--danger); }
  16.67% { border-color: var(--border); }
  33.33% { border-color: var(--danger); }
  50%    { border-color: var(--border); }
  66.67% { border-color: var(--danger); }
  83.33% { border-color: var(--border); }
  100%   { border-color: var(--border); }
}

.port-stepper {
  display: flex;
  align-items: center;
  border: 1px solid var(--border);
  border-radius: 4px;
  overflow: hidden;
  height: 30px;
  background: var(--bg-input);
}
.port-stepper.disabled { opacity: 0.5; }
.port-step-btn {
  width: 22px;
  height: 30px;
  border: none;
  background: var(--bg-button);
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 15px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  padding: 0;
  line-height: 1;
  transition: background 0.12s, color 0.12s;
}
.port-step-btn:hover:not(:disabled) { background: var(--bg-button-hover); color: var(--text-primary); }
.port-step-btn:disabled { cursor: not-allowed; }
.port-stepper input {
  width: 48px;
  border: none;
  border-left: 1px solid var(--border);
  border-right: 1px solid var(--border);
  border-radius: 0;
  background: var(--bg-input);
  text-align: center;
  padding: 0 2px;
  -moz-appearance: textfield;
  -webkit-appearance: none;
  height: 28px;
  color: var(--text-primary);
  font-size: 13px;
}
.port-stepper input::-webkit-outer-spin-button,
.port-stepper input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
.port-stepper input:focus { outline: none; }
.port-stepper input:disabled { opacity: 1; cursor: not-allowed; }

.btn-connect {
  width: 34px;
  height: 30px;
  border-radius: 4px;
  border: none;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--accent);
  color: white;
  flex-shrink: 0;
  transition: background 0.15s;
}

.btn-connect:hover:not(:disabled) {
  background: var(--accent-hover);
}

.btn-connect.connected {
  background: var(--danger);
}

.btn-connect.connected:hover {
  background: var(--danger-hover);
}

.btn-connect:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-connect svg {
  width: 16px;
  height: 16px;
}

.spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255,255,255,0.3);
  border-top-color: white;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.conn-error {
  position: fixed;
  bottom: 20px;
  right: 20px;
  font-size: 12px;
  color: white;
  background: var(--danger);
  border-radius: 4px;
  padding: 6px 12px;
  z-index: 500;
  box-shadow: 0 2px 10px rgba(0,0,0,0.3);
  pointer-events: none;
}

.btn-quick {
  width: 30px;
  height: 30px;
  border-radius: 4px;
  border: 1px solid var(--border);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-button);
  color: var(--text-secondary);
  flex-shrink: 0;
  transition: background 0.15s, color 0.15s, border-color 0.15s;
}

.btn-quick:hover:not(:disabled) {
  background: var(--bg-button-hover);
  color: var(--text-primary);
}

.btn-quick.active {
  background: var(--accent);
  color: white;
  border-color: var(--accent);
}

.btn-quick.active:hover:not(:disabled) {
  background: var(--accent-hover);
}

.btn-quick svg {
  width: 14px;
  height: 14px;
}

.quick-dialog {
  position: fixed;
  top: 48px;
  right: 12px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 12px 14px;
  box-shadow: 0 6px 24px rgba(0,0,0,0.35);
  z-index: 300;
  min-width: 220px;
}

.quick-dialog-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin: 0 0 4px 0;
}

.quick-dialog-host {
  font-size: 13px;
  color: var(--accent);
  font-weight: 500;
  margin: 0 0 10px 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.quick-dialog-btns {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.qdlg-btn {
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-secondary);
  padding: 5px 10px;
  font-size: 12px;
  cursor: pointer;
  text-align: left;
  transition: background 0.12s, color 0.12s;
}

.qdlg-btn:hover {
  background: var(--bg-button-hover);
  color: var(--text-primary);
}

.qdlg-btn.cancel {
  color: var(--text-muted);
  border-color: transparent;
  background: transparent;
}

.qdlg-btn.cancel:hover {
  color: var(--danger);
  background: transparent;
}
</style>
