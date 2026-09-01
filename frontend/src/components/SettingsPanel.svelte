<script>
  import { t } from '../i18n/index.js';
  import { settings, saveSettings } from '../stores/settings.js';
  import { BrowseLocalDir, ExportSettings, ImportSettings } from '../../wailsjs/go/main/App.js';
  import ColorPicker from './ColorPicker.svelte';
  import { trapFocus } from '../utils/focusTrap.js';
  import { NOTIFICATION_SOUNDS, playNotificationSound } from '../utils/sound.js';
  import { notify } from '../stores/notify.js';

  export let onClose = () => {};
  export let onSaved = (_settings) => {};

  let form = {};
  let saved = false;
  let formReady = false;
  let showColorPicker = false;

  $: if ($settings && !formReady) {
    form = { ...$settings };
    formReady = true;
  }

  async function save() {
    await saveSettings(form);
    saved = true;
    setTimeout(() => saved = false, 2000);
    onSaved(form);
  }

  async function browseDir() {
    const dir = await BrowseLocalDir();
    if (dir) form = { ...form, defaultLocalDir: dir };
  }

  function toggle(key) {
    form = { ...form, [key]: !form[key] };
  }

  function step(key, delta, min, max) {
    const current = Number(form[key]) || 0;
    const next = Math.max(min, Math.min(max, current + delta));
    form = { ...form, [key]: next };
  }

  function onColorApply(hex) {
    form = { ...form, accentColor: hex };
    showColorPicker = false;
  }

  const DEFAULTS = {
    theme: 'dark',
    language: 'en',
    maxConcurrentTransfers: 3,
    defaultPort: 21,
    connectionTimeoutSec: 60,
    showHiddenFiles: false,
    passiveMode: true,
    autoReconnect: false,
    confirmOnDelete: true,
    dateFormat: '2006-01-02 15:04',
    maxTransferSpeedKBps: 0,
    maxConnections: 3,
    connectCardShadow: false,
    windowWidth: 1400,
    windowHeight: 900,
    startMaximized: false,
    closeSiteManagerOnClickOutside: true,
    doubleClickNavigateUp: false,
    notificationSoundEnabled: false,
    notificationSound: 'chime',
  };

  function resetSetting(key) {
    if (key in DEFAULTS) form = { ...form, [key]: DEFAULTS[key] };
  }

  async function exportSettings() {
    try {
      await ExportSettings();
    } catch (e) {
      if (e) notify(e.toString(), 'error');
    }
  }

  async function importSettings() {
    try {
      const imported = await ImportSettings();
      if (!imported) return;
      await saveSettings(imported);
      form = { ...imported };
      notify($t('settingsImported'));
    } catch (e) {
      if (e) notify(e.toString(), 'error');
    }
  }
</script>

<div class="panel-backdrop" on:click|self={onClose}></div>

<div class="settings-panel" use:trapFocus>
  <div class="panel-header">
    <span class="panel-title">{$t('settingsTitle')}</span>
    <button class="close-btn" on:click={onClose}>✕</button>
  </div>

  <div class="panel-body">

    <!-- Appearance -->
    <section>
      <h3>{$t('appearance')}</h3>
      <div class="setting-row">
        <label>{$t('theme')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('theme')} title={$t('resetToDefault')}>↺</button></label>
        <div class="toggle-group">
          <button class="toggle-btn" class:active={form.theme === 'dark'} on:click={() => form = { ...form, theme: 'dark' }}>
            <svg class="theme-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
            {$t('themeDark')}
          </button>
          <button class="toggle-btn" class:active={form.theme === 'light'} on:click={() => form = { ...form, theme: 'light' }}>
            <svg class="theme-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg>
            {$t('themeLight')}
          </button>
        </div>
      </div>
      <div class="setting-row">
        <label>{$t('language')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('language')} title={$t('resetToDefault')}>↺</button></label>
        <div class="toggle-group">
          <button class="toggle-btn" class:active={form.language === 'en'} on:click={() => form = { ...form, language: 'en' }}>
            🇬🇧 English
          </button>
          <button class="toggle-btn" class:active={form.language === 'fr'} on:click={() => form = { ...form, language: 'fr' }}>
            🇫🇷 Français
          </button>
        </div>
      </div>
      <div class="setting-row">
        <label>{$t('accentColor')}</label>
        <div class="color-row">
          <div class="color-swatch" style="background: {form.accentColor || '#5B8AF5'}"></div>
          <span class="color-hex">{form.accentColor || '#5B8AF5'}</span>
          <button class="color-pick-btn" on:click={() => showColorPicker = true}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="4"/><line x1="12" y1="2" x2="12" y2="4"/><line x1="12" y1="20" x2="12" y2="22"/><line x1="2" y1="12" x2="4" y2="12"/><line x1="20" y1="12" x2="22" y2="12"/></svg>
            {$t('accentColor')}
          </button>
        </div>
      </div>
    </section>

    <div class="divider"></div>

    <!-- Transfers -->
    <section>
      <h3>{$t('transfers')}</h3>
      <div class="setting-row">
        <label>{$t('maxConcurrent')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('maxConcurrentTransfers')} title={$t('resetToDefault')}>↺</button></label>
        <div class="num-input">
          <button class="num-btn" on:click={() => step('maxConcurrentTransfers', -1, 1, 10)}>−</button>
          <input type="number" bind:value={form.maxConcurrentTransfers} min="1" max="10" />
          <button class="num-btn" on:click={() => step('maxConcurrentTransfers', 1, 1, 10)}>+</button>
        </div>
      </div>
      <div class="setting-row">
        <label>{$t('transferSpeedLimit')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('maxTransferSpeedKBps')} title={$t('resetToDefault')}>↺</button></label>
        <div class="num-input">
          <button class="num-btn" on:click={() => step('maxTransferSpeedKBps', -100, 0, 100000)}>−</button>
          <input type="number" bind:value={form.maxTransferSpeedKBps} min="0" max="100000" style="width: 80px" />
          <button class="num-btn" on:click={() => step('maxTransferSpeedKBps', 100, 0, 100000)}>+</button>
        </div>
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

    <!-- Notifications -->
    <section>
      <h3>{$t('notifications')}</h3>
      <div class="setting-row">
        <label>{$t('notificationSoundEnabled')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('notificationSoundEnabled')} title={$t('resetToDefault')}>↺</button></label>
        <button
          type="button"
          class="sw"
          class:on={form.notificationSoundEnabled}
          on:click={() => toggle('notificationSoundEnabled')}
          aria-pressed={form.notificationSoundEnabled}
        ><span class="sw-knob"></span></button>
      </div>
      {#if form.notificationSoundEnabled}
        <div class="setting-row">
          <label>{$t('notificationSound')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('notificationSound')} title={$t('resetToDefault')}>↺</button></label>
          <div class="sound-row">
            <select bind:value={form.notificationSound}>
              {#each NOTIFICATION_SOUNDS as sound (sound.id)}
                <option value={sound.id}>{$t(sound.label)}</option>
              {/each}
            </select>
            <button
              type="button"
              class="sound-test-btn"
              title={$t('testSound')}
              on:click={() => playNotificationSound(form.notificationSound)}
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/>
                <path d="M15.54 8.46a5 5 0 0 1 0 7.07"/>
                <path d="M19.07 4.93a10 10 0 0 1 0 14.14"/>
              </svg>
            </button>
          </div>
        </div>
      {/if}
    </section>

    <div class="divider"></div>

    <!-- Connection -->
    <section>
      <h3>{$t('connection')}</h3>
      <div class="setting-row">
        <label>{$t('defaultPort')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('defaultPort')} title={$t('resetToDefault')}>↺</button></label>
        <div class="num-input">
          <button class="num-btn" on:click={() => step('defaultPort', -1, 1, 65535)}>−</button>
          <input type="number" bind:value={form.defaultPort} min="1" max="65535" />
          <button class="num-btn" on:click={() => step('defaultPort', 1, 1, 65535)}>+</button>
        </div>
      </div>
      <div class="setting-row">
        <label>{$t('timeout')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('connectionTimeoutSec')} title={$t('resetToDefault')}>↺</button></label>
        <div class="num-input">
          <button class="num-btn" on:click={() => step('connectionTimeoutSec', -5, 5, 300)}>−</button>
          <input type="number" bind:value={form.connectionTimeoutSec} min="5" max="300" />
          <button class="num-btn" on:click={() => step('connectionTimeoutSec', 5, 5, 300)}>+</button>
        </div>
      </div>
      <div class="setting-row">
        <label>{$t('passiveMode')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('passiveMode')} title={$t('resetToDefault')}>↺</button></label>
        <button
          type="button"
          class="sw"
          class:on={form.passiveMode}
          on:click={() => toggle('passiveMode')}
          aria-pressed={form.passiveMode}
        ><span class="sw-knob"></span></button>
      </div>
      <div class="setting-row">
        <label>{$t('autoReconnect')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('autoReconnect')} title={$t('resetToDefault')}>↺</button></label>
        <button
          type="button"
          class="sw"
          class:on={form.autoReconnect}
          on:click={() => toggle('autoReconnect')}
          aria-pressed={form.autoReconnect}
        ><span class="sw-knob"></span></button>
      </div>
      <div class="setting-row">
        <label>{$t('maxConnections')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('maxConnections')} title={$t('resetToDefault')}>↺</button></label>
        <div class="num-input">
          <button class="num-btn" on:click={() => step('maxConnections', -1, 1, 5)}>−</button>
          <input type="number" bind:value={form.maxConnections} min="1" max="5" />
          <button class="num-btn" on:click={() => step('maxConnections', 1, 1, 5)}>+</button>
        </div>
      </div>
    </section>

    <div class="divider"></div>

    <!-- Interface -->
    <section>
      <h3>{$t('interface')}</h3>
      <div class="setting-row">
        <label>{$t('connectCardShadow')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('connectCardShadow')} title={$t('resetToDefault')}>↺</button></label>
        <button
          type="button"
          class="sw"
          class:on={form.connectCardShadow}
          on:click={() => toggle('connectCardShadow')}
          aria-pressed={form.connectCardShadow}
        ><span class="sw-knob"></span></button>
      </div>
      <div class="setting-row">
        <label>{$t('closeSiteManagerOnClickOutside')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('closeSiteManagerOnClickOutside')} title={$t('resetToDefault')}>↺</button></label>
        <button
          type="button"
          class="sw"
          class:on={form.closeSiteManagerOnClickOutside}
          on:click={() => toggle('closeSiteManagerOnClickOutside')}
          aria-pressed={form.closeSiteManagerOnClickOutside}
        ><span class="sw-knob"></span></button>
      </div>
      <div class="setting-row">
        <label>{$t('doubleClickNavigateUp')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('doubleClickNavigateUp')} title={$t('resetToDefault')}>↺</button></label>
        <button
          type="button"
          class="sw"
          class:on={form.doubleClickNavigateUp}
          on:click={() => toggle('doubleClickNavigateUp')}
          aria-pressed={form.doubleClickNavigateUp}
        ><span class="sw-knob"></span></button>
      </div>
      <div class="setting-row">
        <label>{$t('showHiddenFiles')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('showHiddenFiles')} title={$t('resetToDefault')}>↺</button></label>
        <button
          type="button"
          class="sw"
          class:on={form.showHiddenFiles}
          on:click={() => toggle('showHiddenFiles')}
          aria-pressed={form.showHiddenFiles}
        ><span class="sw-knob"></span></button>
      </div>
      <div class="setting-row">
        <label>{$t('confirmOnDelete')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('confirmOnDelete')} title={$t('resetToDefault')}>↺</button></label>
        <button
          type="button"
          class="sw"
          class:on={form.confirmOnDelete}
          on:click={() => toggle('confirmOnDelete')}
          aria-pressed={form.confirmOnDelete}
        ><span class="sw-knob"></span></button>
      </div>
      <div class="setting-row">
        <label>{$t('dateFormat')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('dateFormat')} title={$t('resetToDefault')}>↺</button></label>
        <input type="text" bind:value={form.dateFormat} placeholder="2006-01-02 15:04" style="width: 160px" />
      </div>
      <div class="setting-row">
        <label>{$t('startMaximized')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('startMaximized')} title={$t('resetToDefault')}>↺</button></label>
        <button
          type="button"
          class="sw"
          class:on={form.startMaximized}
          on:click={() => toggle('startMaximized')}
          aria-pressed={form.startMaximized}
        ><span class="sw-knob"></span></button>
      </div>
      <div class="setting-row" class:row-disabled={form.startMaximized}>
        <label>{$t('windowWidth')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('windowWidth')} title={$t('resetToDefault')}>↺</button></label>
        <div class="num-input">
          <button class="num-btn" disabled={form.startMaximized} on:click={() => step('windowWidth', -50, 800, 3840)}>−</button>
          <input type="number" bind:value={form.windowWidth} min="800" max="3840" disabled={form.startMaximized} />
          <button class="num-btn" disabled={form.startMaximized} on:click={() => step('windowWidth', 50, 800, 3840)}>+</button>
        </div>
      </div>
      <div class="setting-row" class:row-disabled={form.startMaximized}>
        <label>{$t('windowHeight')} <button class="reset-btn" on:click|stopPropagation={() => resetSetting('windowHeight')} title={$t('resetToDefault')}>↺</button></label>
        <div class="num-input">
          <button class="num-btn" disabled={form.startMaximized} on:click={() => step('windowHeight', -50, 600, 2160)}>−</button>
          <input type="number" bind:value={form.windowHeight} min="600" max="2160" disabled={form.startMaximized} />
          <button class="num-btn" disabled={form.startMaximized} on:click={() => step('windowHeight', 50, 600, 2160)}>+</button>
        </div>
      </div>
    </section>

  </div>

  <div class="panel-footer">
    <span class="version-badge">v1.7.7</span>
    <button class="io-icon-btn" on:click={exportSettings} title={$t('exportSettings')} aria-label={$t('exportSettings')}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
    </button>
    <button class="io-icon-btn" on:click={importSettings} title={$t('importSettings')} aria-label={$t('importSettings')}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
    </button>
    <div class="footer-spacer"></div>
    {#if saved}
      <span class="saved-msg">{$t('settingsSaved')} ✓</span>
    {/if}
    <button class="btn-primary" on:click={save}>{$t('saveSettings')}</button>
  </div>
</div>

{#if showColorPicker}
  <ColorPicker
    value={form.accentColor || '#5B8AF5'}
    onClose={() => showColorPicker = false}
    onApply={onColorApply}
  />
{/if}

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

.panel-title { font-size: 16px; font-weight: 600; color: var(--text-primary); }

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

.toggle-group { display: flex; gap: 4px; }

.toggle-btn {
  display: flex;
  align-items: center;
  gap: 6px;
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
.theme-icon {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
}

/* ── Color row ── */
.color-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.color-swatch {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  border: 2px solid var(--border);
  flex-shrink: 0;
}

.color-hex {
  font-size: 12px;
  color: var(--text-muted);
  font-family: monospace;
}

.color-pick-btn {
  display: flex;
  align-items: center;
  gap: 5px;
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-secondary);
  padding: 4px 10px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.12s;
}
.color-pick-btn:hover { background: var(--bg-button-hover); color: var(--text-primary); }
.color-pick-btn svg { width: 13px; height: 13px; }

/* ── Toggle switch ── */
.sw {
  position: relative;
  width: 42px;
  height: 24px;
  border-radius: 24px;
  background: var(--border);
  border: none;
  cursor: pointer;
  flex-shrink: 0;
  padding: 0;
  transition: background 0.2s;
  outline: none;
}
.sw.on { background: var(--accent); }
.sw:focus-visible { box-shadow: 0 0 0 2px var(--accent); }

.sw-knob {
  position: absolute;
  width: 18px;
  height: 18px;
  background: white;
  border-radius: 50%;
  top: 3px;
  left: 3px;
  transition: transform 0.2s;
  pointer-events: none;
}
.sw.on .sw-knob { transform: translateX(18px); }

/* ── Number stepper ── */
.num-input {
  display: flex;
  align-items: center;
  border: 1px solid var(--border);
  border-radius: 4px;
  overflow: hidden;
}

.num-input input {
  width: 60px;
  text-align: center;
  background: var(--bg-input);
  border: none;
  border-left: 1px solid var(--border);
  border-right: 1px solid var(--border);
  color: var(--text-primary);
  padding: 5px 4px;
  font-size: 13px;
  outline: none;
  -moz-appearance: textfield;
}
.num-input input::-webkit-inner-spin-button,
.num-input input::-webkit-outer-spin-button { -webkit-appearance: none; margin: 0; }

.num-btn {
  background: var(--bg-button);
  border: none;
  color: var(--text-secondary);
  padding: 0 10px;
  height: 32px;
  font-size: 16px;
  cursor: pointer;
  transition: background 0.1s, color 0.1s;
  user-select: none;
}
.num-btn:hover { background: var(--bg-button-hover); color: var(--text-primary); }
.num-btn:active { background: var(--accent); color: white; }

/* ── Text / number inputs ── */
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

.input-with-btn { display: flex; gap: 6px; flex: 1; }
.input-with-btn input { flex: 1; }

.browse-btn {
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-secondary);
  padding: 5px 10px;
  cursor: pointer;
}

/* ── Notification sound row ── */
.sound-row { display: flex; align-items: center; gap: 8px; }

.sound-row select {
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  padding: 5px 8px;
  font-size: 13px;
  outline: none;
  min-width: 130px;
}
.sound-row select:focus { border-color: var(--accent); }

.sound-test-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-secondary);
  width: 30px;
  height: 30px;
  flex-shrink: 0;
  cursor: pointer;
  transition: all 0.12s;
}
.sound-test-btn:hover { background: var(--bg-button-hover); color: var(--accent); }
.sound-test-btn svg { width: 15px; height: 15px; }

.divider { height: 1px; background: var(--border); margin: 16px 0; }

.reset-btn {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 13px;
  padding: 0 3px;
  opacity: 0.45;
  transition: opacity 0.1s, color 0.1s;
  vertical-align: middle;
  line-height: 1;
}
.reset-btn:hover { opacity: 1; color: var(--accent); }

.row-disabled { opacity: 0.4; pointer-events: none; }

.panel-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding: 14px 20px;
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}

.version-badge { font-size: 12px; color: var(--accent); }

.footer-spacer { flex: 1; }

.io-icon-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 50%;
  color: var(--text-secondary);
  cursor: pointer;
  padding: 0;
  transition: background 0.12s, color 0.12s, border-color 0.12s;
}
.io-icon-btn:hover { background: var(--bg-button-hover); color: var(--accent); border-color: var(--accent); }
.io-icon-btn svg { width: 15px; height: 15px; }

.saved-msg { font-size: 13px; color: var(--success); }

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
