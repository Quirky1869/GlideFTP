<script>
  import { t } from '../i18n/index.js';
  import { connectionStatus, connectionError, connect, disconnect, refreshRemote, remotePath } from '../stores/connection.js';
  import { GetSites, ConnectToSite } from '../../wailsjs/go/main/App.js';

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

  $: isConnected = $connectionStatus === 'connected';
  $: isConnecting = $connectionStatus === 'connecting';

  $: if (protocol === 'sftp' && port === 21) port = 22;
  $: if (protocol === 'ftp' && port === 22) port = 21;

  async function handleConnect() {
    if (isConnected) {
      await disconnect();
      return;
    }
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
    if (e.key === 'Enter') handleConnect();
  }
</script>

<div class="conn-bar">
  <div class="conn-fields">
    <div class="field-group">
      <label>{$t('protocol')}</label>
      <select bind:value={protocol} disabled={isConnected || isConnecting}>
        <option value="ftp">FTP</option>
        <option value="sftp">SFTP</option>
      </select>
    </div>

    <div class="field-group flex1">
      <label>{$t('host')}</label>
      <input
        type="text"
        bind:value={host}
        placeholder="ftp.example.com"
        disabled={isConnected || isConnecting}
        on:keydown={handleKeydown}
      />
    </div>

    <div class="field-group">
      <label>{$t('user')}</label>
      <input
        type="text"
        bind:value={user}
        placeholder="user"
        disabled={isConnected || isConnecting}
        on:keydown={handleKeydown}
      />
    </div>

    <div class="field-group">
      <label>{$t('password')}</label>
      <input
        type="password"
        bind:value={password}
        placeholder="••••••••"
        disabled={isConnected || isConnecting}
        on:keydown={handleKeydown}
      />
    </div>

    <div class="field-group port-group">
      <label>{$t('port')}</label>
      <input
        type="number"
        bind:value={port}
        min="1"
        max="65535"
        disabled={isConnected || isConnecting}
        on:keydown={handleKeydown}
      />
    </div>

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

.port-group input {
  width: 70px;
}

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
  font-size: 12px;
  color: var(--danger);
  padding: 2px 0;
}
</style>
