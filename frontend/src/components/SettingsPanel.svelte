<script>
  import { t } from '../i18n/index.js';
  import { settings, saveSettings } from '../stores/settings.js';
  import { BrowseLocalDir } from '../../wailsjs/go/main/App.js';

  export let onClose = () => {};

  let form = {};
  let saved = false;

  $: if ($settings) {
    form = { ...$settings };
  }

  async function save() {
    await saveSettings(form);
    saved = true;
    setTimeout(() => saved = false, 2000);
  }

  async function browseDir() {
    const dir = await BrowseLocalDir();
    if (dir) form.defaultLocalDir = dir;
  }
</script>

<!-- Backdrop for click-outside -->
<div class="panel-backdrop" on:click|self={onClose}></div>

<div class="settings-panel">
  <div class="panel-header">
    <span class="panel-title">{$t('settingsTitle')}</span>
    <button class="close-btn" on:click={onClose}>✕</button>
  </div>

  <div class="panel-body">

    <!-- Appearance -->
    <section>
      <h3>{$t('appearance')}</h3>
      <div class="setting-row">
        <label>{$t('theme')}</label>
        <div class="toggle-group">
          <button class="toggle-btn" class:active={form.theme === 'dark'} on:click={() => form.theme = 'dark'}>
            🌙 {$t('themeDark')}
          </button>
          <button class="toggle-btn" class:active={form.theme === 'light'} on:click={() => form.theme = 'light'}>
            ☀️ {$t('themeLight')}
          </button>
        </div>
      </div>
      <div class="setting-row">
        <label>{$t('language')}</label>
        <div class="toggle-group">
          <button class="toggle-btn" class:active={form.language === 'en'} on:click={() => form.language = 'en'}>
            🇬🇧 English
          </button>
          <button class="toggle-btn" class:active={form.language === 'fr'} on:click={() => form.language = 'fr'}>
            🇫🇷 Français
          </button>
        </div>
      </div>
    </section>

    <div class="divider"></div>

    <!-- Transfers -->
    <section>
      <h3>{$t('transfers')}</h3>
      <div class="setting-row">
        <label>{$t('maxConcurrent')}</label>
        <input type="number" bind:value={form.maxConcurrentTransfers} min="1" max="10" style="width: 70px" />
      </div>
      <div class="setting-row">
        <label>{$t('defaultLocalDir')}</label>
        <div class="input-with-btn">
          <input type="text" bind:value={form.defaultLocalDir} />
          <button class="browse-btn" on:click={browseDir}>…</button>
        </div>
      </div>
    </section>

    <div class="divider"></div>

    <!-- Connection -->
    <section>
      <h3>{$t('connection')}</h3>
      <div class="setting-row">
        <label>{$t('defaultPort')}</label>
        <input type="number" bind:value={form.defaultPort} min="1" max="65535" style="width: 90px" />
      </div>
      <div class="setting-row">
        <label>{$t('timeout')}</label>
        <input type="number" bind:value={form.connectionTimeoutSec} min="5" max="120" style="width: 90px" />
      </div>
      <div class="setting-row">
        <label>{$t('passiveMode')}</label>
        <label class="switch">
          <input type="checkbox" bind:checked={form.passiveMode} />
          <span class="slider"></span>
        </label>
      </div>
      <div class="setting-row">
        <label>{$t('autoReconnect')}</label>
        <label class="switch">
          <input type="checkbox" bind:checked={form.autoReconnect} />
          <span class="slider"></span>
        </label>
      </div>
    </section>

    <div class="divider"></div>

    <!-- Interface -->
    <section>
      <h3>{$t('interface')}</h3>
      <div class="setting-row">
        <label>{$t('showHiddenFiles')}</label>
        <label class="switch">
          <input type="checkbox" bind:checked={form.showHiddenFiles} />
          <span class="slider"></span>
        </label>
      </div>
      <div class="setting-row">
        <label>{$t('confirmOnDelete')}</label>
        <label class="switch">
          <input type="checkbox" bind:checked={form.confirmOnDelete} />
          <span class="slider"></span>
        </label>
      </div>
      <div class="setting-row">
        <label>{$t('dateFormat')}</label>
        <input type="text" bind:value={form.dateFormat} placeholder="2006-01-02 15:04" style="width: 160px" />
      </div>
    </section>

  </div>

  <div class="panel-footer">
    {#if saved}
      <span class="saved-msg">{$t('settingsSaved')} ✓</span>
    {/if}
    <button class="btn-primary" on:click={save}>{$t('saveSettings')}</button>
  </div>
</div>

<style>
.panel-backdrop {
  position: fixed;
  inset: 0;
  z-index: 300;
}

.settings-panel {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  width: 75%;
  max-width: 700px;
  background: var(--bg-secondary);
  border-left: 1px solid var(--border);
  box-shadow: -8px 0 32px rgba(0,0,0,0.4);
  z-index: 400;
  display: flex;
  flex-direction: column;
  animation: slideIn 0.22s ease;
}

@keyframes slideIn {
  from { transform: translateX(100%); opacity: 0; }
  to { transform: translateX(0); opacity: 1; }
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.panel-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.close-btn {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 18px;
}

.close-btn:hover { color: var(--text-primary); }

.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 0;
}

section { padding-bottom: 4px; }

h3 {
  font-size: 13px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--accent);
  margin: 0 0 12px;
}

.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-subtle);
}

.setting-row label:first-child {
  font-size: 13px;
  color: var(--text-primary);
  flex: 1;
}

.toggle-group {
  display: flex;
  gap: 4px;
}

.toggle-btn {
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-secondary);
  padding: 5px 14px;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.12s;
}

.toggle-btn.active {
  background: var(--accent);
  border-color: var(--accent);
  color: white;
}

input[type="text"], input[type="number"] {
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  padding: 5px 8px;
  font-size: 13px;
  outline: none;
}

input:focus { border-color: var(--accent); }

.input-with-btn {
  display: flex;
  gap: 6px;
  flex: 1;
}

.input-with-btn input { flex: 1; }

.browse-btn {
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-secondary);
  padding: 5px 10px;
  cursor: pointer;
}

/* Toggle switch */
.switch {
  position: relative;
  display: inline-block;
  width: 40px;
  height: 22px;
  flex-shrink: 0;
  cursor: pointer;
}

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
  position: absolute;
}

.slider {
  position: absolute;
  inset: 0;
  background: var(--border);
  border-radius: 22px;
  transition: background 0.2s;
}

.slider:before {
  content: '';
  position: absolute;
  height: 16px;
  width: 16px;
  left: 3px;
  bottom: 3px;
  background: white;
  border-radius: 50%;
  transition: transform 0.2s;
}

.switch input:checked + .slider { background: var(--accent); }
.switch input:checked + .slider:before { transform: translateX(18px); }

.divider {
  height: 1px;
  background: var(--border);
  margin: 16px 0;
}

.panel-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding: 14px 20px;
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}

.saved-msg {
  font-size: 13px;
  color: var(--success);
}

.btn-primary {
  background: var(--accent);
  border: none;
  border-radius: 5px;
  color: white;
  padding: 8px 20px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
}

.btn-primary:hover { background: var(--accent-hover); }
</style>
