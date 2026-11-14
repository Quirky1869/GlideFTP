<script>
  import { onDestroy } from 'svelte';
  import { openReleaseNotes } from '../stores/releaseNotes.js';
  import { t, locale } from '../i18n/index.js';
  import { settings, saveSettings, gradientAngle } from '../stores/settings.js';
  import { BrowseLocalDir, ExportSettings, ImportSettings, GetSearchIndexInfo, ClearSearchIndex, ChooseCustomSound, InstallCustomSound, RemoveCustomSound, GetAppVersion } from '../../wailsjs/go/main/App.js';
  import SearchIndexModal from './SearchIndexModal.svelte';
  import { formatBytes } from '../stores/transfers.js';
  import ColorPicker from './ColorPicker.svelte';
  import { trapFocus } from '../utils/focusTrap.js';
  import { NOTIFICATION_SOUNDS, CUSTOM_SOUND_ID, MAX_SOUND_SECONDS, playNotificationSound, probeSound, setCustomSoundUrl } from '../utils/sound.js';
  import { notify } from '../stores/notify.js';
  import { checkForUpdateNow, previewUpdateBanner } from '../stores/update.js';
  import { WindowGetSize } from '../../wailsjs/runtime/runtime.js';
  import { DEFAULT_DATE_FORMAT, SYSTEM_DATE_FORMAT, DATE_FORMAT_PRESETS, DATE_FORMAT_TOKENS, normalizeDateFormat, formatDateWith } from '../utils/dateFormat.js';

  export let onClose = () => {};
  export let onSaved = (_settings) => {};

  let form = {};
  let saved = false;
  let formReady = false;
  let showColorPicker = false;

  // Alphabetical order depends on the active language (translated labels sort
  // differently in en vs fr) - sorted reactively rather than baked into
  // NOTIFICATION_SOUNDS's fixed array order.
  $: sortedSounds = [...NOTIFICATION_SOUNDS].sort((a, b) =>
    $t(a.label).localeCompare($t(b.label), $locale)
  );

  $: if ($settings && !formReady) {
    form = { ...$settings, dateFormat: normalizeDateFormat($settings.dateFormat) };
    lastSavedJson = JSON.stringify(form);
    syncDateAdvanced();
    formReady = true;
  }

  // ── Instant save (v1.8.0 #141) ──
  // Every change is saved on its own, no "Save" button: switches, buttons,
  // lists and +/- right away (short debounce to group quick clicks); a text
  // or number field only once the user leaves it (focusout), so a path or a
  // number being typed is never saved half-way (the local panel would jump
  // to each partial "default local directory"). Pending changes are saved
  // when the panel closes. "Settings saved ✓" shows after each save.
  const SAVE_DELAY_MS = 300;
  let lastSavedJson = '';
  let saveTimer = null;
  let savedTimer = null;

  const isTyping = () => {
    const el = document.activeElement;
    return !!el && (el.tagName === 'TEXTAREA' || (el.tagName === 'INPUT' && el.type !== 'checkbox'));
  };

  $: if (formReady) scheduleSave(form);

  function scheduleSave(f, delay = SAVE_DELAY_MS) {
    if (JSON.stringify(f) === lastSavedJson || isTyping()) return;
    clearTimeout(saveTimer);
    saveTimer = setTimeout(save, delay);
  }

  // A number field emptied or half-typed: wait for a valid value.
  function formValid(f) {
    return Object.keys(DEFAULTS).every(k => typeof DEFAULTS[k] !== 'number' || (f[k] !== '' && Number.isFinite(Number(f[k]))));
  }

  onDestroy(() => {
    clearTimeout(savedTimer);
    if (formReady && JSON.stringify(form) !== lastSavedJson) save();
  });

  // ── Date format ──
  // The dropdown lists the presets + "Advanced"; advanced reveals a free text
  // input. dateAdvanced is explicit state (not derived from the value) so
  // typing a custom format that happens to match a preset doesn't hide the input.
  const DATE_ADVANCED = '__advanced';
  let dateAdvanced = false;
  let showDateTip = false;
  let dateTipPos = { x: 0, y: 0, above: true };
  let now = new Date();
  // "?" hints next to some labels: themed tooltip on hover/focus, position:
  // fixed like the date tip so .panel-body's overflow can't clip it.
  let hint = null; // { text, x, y, above }

  function syncDateAdvanced() {
    dateAdvanced = !DATE_FORMAT_PRESETS.includes(form.dateFormat);
  }

  function onDatePresetChange(e) {
    const v = e.target.value;
    if (v === DATE_ADVANCED) {
      dateAdvanced = true;
      // start from the current format so the user edits rather than retypes
      if (form.dateFormat === SYSTEM_DATE_FORMAT) form = { ...form, dateFormat: DEFAULT_DATE_FORMAT };
    } else {
      dateAdvanced = false;
      form = { ...form, dateFormat: v };
    }
  }

  // The tooltip is position:fixed so .panel-body's overflow can't clip it.
  function openDateTip(e) {
    const r = e.currentTarget.getBoundingClientRect();
    const above = r.top > window.innerHeight / 2;
    dateTipPos = { x: r.right, y: above ? r.top - 8 : r.bottom + 8, above };
    now = new Date();
    showDateTip = true;
  }

  function openHint(e, key) {
    const r = e.currentTarget.getBoundingClientRect();
    const above = r.top > window.innerHeight / 2;
    hint = { text: $t(key), x: r.left, y: above ? r.top - 8 : r.bottom + 8, above };
  }
  function closeHint() { hint = null; }

  async function save() {
    clearTimeout(saveTimer);
    saveTimer = null;
    if (!formValid(form)) return;
    const snapshot = { ...form };
    for (const k of Object.keys(DEFAULTS)) {
      if (typeof DEFAULTS[k] === 'number') snapshot[k] = Number(snapshot[k]);
    }
    // Default FTP port typed by hand: out of range or 22 (SFTP) → back to 21.
    const fp = snapshot.defaultPort;
    if (!Number.isInteger(fp) || fp < 1 || fp > 65535 || fp === 22) {
      snapshot.defaultPort = DEFAULTS.defaultPort;
      form = { ...form, defaultPort: DEFAULTS.defaultPort };
    }
    const json = JSON.stringify(snapshot);
    if (json === lastSavedJson) return;
    lastSavedJson = json;
    try {
      await saveSettings(snapshot);
    } catch (e) {
      lastSavedJson = '';
      notify(e?.toString() || $t('unknownError'), 'error');
      return;
    }
    saved = true;
    clearTimeout(savedTimer);
    savedTimer = setTimeout(() => saved = false, 2000);
    onSaved(snapshot);
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

  // Default FTP port: skips 22 (SFTP's port), like the connection screen.
  function stepFtpPort(delta) {
    let next = (Number(form.defaultPort) || 21) + delta;
    if (next === 22) next += delta;
    form = { ...form, defaultPort: Math.max(1, Math.min(65535, next)) };
  }

  // ── Custom notification sound ──
  // Pick → check WebKit can decode it → copy to the config dir → select it and
  // save immediately (like the color picker). Played for MAX_SOUND_SECONDS at most.
  let loadingSound = false;

  function soundErrorText(e) {
    const msg = e?.toString() || '';
    if (msg.includes('too_large')) return $t('customSoundTooLarge');
    if (msg.includes('unsupported_format')) return $t('customSoundUnsupported');
    return $t('customSoundError') + (msg ? '\n' + msg : '');
  }

  async function loadCustomSound() {
    if (loadingSound) return;
    loadingSound = true;
    try {
      const file = await ChooseCustomSound();
      if (!file) return;
      let duration;
      try {
        duration = await probeSound(file.dataUrl);
      } catch {
        notify($t('customSoundUnreadable'), 'error');
        return;
      }
      const name = await InstallCustomSound(file.path);
      setCustomSoundUrl(file.dataUrl);
      form = { ...form, notificationSound: CUSTOM_SOUND_ID, customSoundName: name };
      await save();
      playNotificationSound(CUSTOM_SOUND_ID);
      const long = isFinite(duration) && duration > MAX_SOUND_SECONDS;
      notify($t(long ? 'customSoundLoadedLong' : 'customSoundLoaded').replace('{name}', name).replace('{n}', MAX_SOUND_SECONDS));
    } catch (e) {
      notify(soundErrorText(e), 'error');
    } finally {
      loadingSound = false;
    }
  }

  async function removeCustomSound() {
    try {
      await RemoveCustomSound();
      setCustomSoundUrl('');
      form = {
        ...form,
        customSoundName: '',
        notificationSound: form.notificationSound === CUSTOM_SOUND_ID ? DEFAULTS.notificationSound : form.notificationSound,
      };
      await save();
    } catch (e) {
      notify(soundErrorText(e), 'error');
    }
  }

  async function onColorApply({ color, gradient, color2, angle }) {
    form = { ...form, accentColor: color, accentGradient: gradient, accentColor2: color2, accentGradientAngle: angle };
    showColorPicker = false;
    await save();
  }

  const DEFAULTS = {
    theme: 'dark',
    language: 'en',
    maxConcurrentTransfers: 3,
    defaultPort: 21,
    connectionTimeoutSec: 60,
    showHiddenFiles: false,
    confirmOnDelete: true,
    dateFormat: DEFAULT_DATE_FORMAT,
    maxTransferSpeedKBps: 0,
    maxConnections: 3,
    connectCardShadow: false,
    windowWidth: 1400,
    windowHeight: 900,
    startMaximized: false,
    showWindowSizeBadge: true,
    closeSiteManagerOnClickOutside: true,
    doubleClickNavigateUp: false,
    notificationSoundEnabled: false,
    notificationSound: 'chime',
    customSoundName: '',
    queueDoneBubble: false,
    searchIndexEnabled: false,
    checkUpdates: true,
    syncCreateMissing: false,
    compareBy: 'size',
    compareHideIdentical: false,
    compareAlign: false,
    caseSensitiveSort: false,
  };

  // ── Updates (section "Mises à jour") ──
  // Version comes from the Go side (version.go, set at build time by make.sh).
  let appVersion = '';
  GetAppVersion().then(v => appVersion = v).catch(() => {});
  let checkingUpdate = false;
  async function checkUpdateNow() {
    checkingUpdate = true;
    try {
      const info = await checkForUpdateNow();
      if (info?.available) {
        notify($t('updateFoundNotify').replace('{latest}', info.latest), 'success', { label: $t('updateShowBanner'), run: onClose });
      } else {
        notify($t('updateUpToDate').replace('{current}', info?.current || appVersion));
      }
    } catch (e) {
      notify($t('updateCheckFailed') + (e ? `\n${e}` : ''), 'error');
    } finally {
      checkingUpdate = false;
    }
  }
  // Shows the real banner (links of the latest published release), then
  // closes Settings so it is visible.
  async function testUpdateBanner() {
    checkingUpdate = true;
    try {
      await previewUpdateBanner();
      onClose();
    } catch (e) {
      notify($t('updateCheckFailed') + (e ? `\n${e}` : ''), 'error');
    } finally {
      checkingUpdate = false;
    }
  }

  // ── Search index (section "Recherche") ──
  let showSearchIndexHelp = false;
  let searchIndexSize = 0;
  async function refreshSearchIndexSize() {
    try { searchIndexSize = (await GetSearchIndexInfo())?.sizeBytes || 0; } catch {}
  }
  refreshSearchIndexSize();

  // "Activer" in the "?" popup: switch on and save right away (one click,
  // like the color picker's Save).
  async function enableSearchIndexFromHelp() {
    showSearchIndexHelp = false;
    form = { ...form, searchIndexEnabled: true, searchIndexAsked: true };
    await save();
  }

  async function clearSearchIndex() {
    try {
      await ClearSearchIndex();
      searchIndexSize = 0;
      notify($t('searchIndexCleared'));
    } catch (e) {
      notify(e?.toString() || $t('unknownError'), 'error');
    }
  }

  // Fills Window width/height with the current window size (same units as
  // main.go's options.Width/Height), clamped to the fields' min/max, and
  // saves right away - same one-click behavior as onColorApply().
  let sizeCopied = false;
  let sizeCopiedTimer = null;
  async function useCurrentWindowSize() {
    try {
      const { w, h } = await WindowGetSize();
      form = {
        ...form,
        windowWidth: Math.max(800, Math.min(3840, w)),
        windowHeight: Math.max(600, Math.min(2160, h)),
      };
      await save();
      sizeCopied = true;
      clearTimeout(sizeCopiedTimer);
      sizeCopiedTimer = setTimeout(() => sizeCopied = false, 2000);
    } catch (e) {
      notify(e?.toString() || $t('unknownError'), 'error');
    }
  }

  // Light theme: a (joke) warning first - "flashbang mode".
  let confirmFlashbang = false;
  function answerFlashbang(yes) {
    confirmFlashbang = false;
    if (yes) form = { ...form, theme: 'light' };
  }
  function focusOnOpen(node) {
    requestAnimationFrame(() => node.focus());
  }

  function resetSetting(key) {
    if (key in DEFAULTS) form = { ...form, [key]: DEFAULTS[key] };
    if (key === 'dateFormat') syncDateAdvanced();
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
      form = { ...imported, dateFormat: normalizeDateFormat(imported.dateFormat) };
      lastSavedJson = JSON.stringify(form);
      syncDateAdvanced();
      onSaved(form);
      notify($t('settingsImported'));
    } catch (e) {
      if (e) notify(e.toString(), 'error');
    }
  }
</script>

<div class="panel-backdrop" on:click|self={onClose}></div>

<div class="settings-panel" use:trapFocus on:focusout={() => setTimeout(() => scheduleSave(form, 0))}>
  <div class="panel-header">
    <span class="panel-title">{$t('settingsTitle')}</span>
    <button class="close-btn" on:click={onClose}>✕</button>
  </div>

  <div class="panel-body">

    <!-- Appearance -->
    <section>
      <h3>{$t('appearance')}</h3>
      <div class="card">
        <div class="setting-row">
          <label>{$t('theme')} <button class="reset-btn" hidden={form.theme === DEFAULTS.theme} on:click|stopPropagation={() => resetSetting('theme')} title={$t('resetToDefault')}>↺</button></label>
          <div class="toggle-group">
            <button class="toggle-btn" class:active={form.theme === 'dark'} on:click={() => form = { ...form, theme: 'dark' }}>
              <svg class="theme-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
              {$t('themeDark')}
            </button>
            <button class="toggle-btn" class:active={form.theme === 'light'} on:click={() => { if (form.theme !== 'light') confirmFlashbang = true; }}>
              <svg class="theme-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg>
              {$t('themeLight')}
            </button>
          </div>
        </div>
        <div class="setting-row">
          <label>{$t('language')} <button class="reset-btn" hidden={form.language === DEFAULTS.language} on:click|stopPropagation={() => resetSetting('language')} title={$t('resetToDefault')}>↺</button></label>
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
            {#if form.accentGradient}
              <div class="color-swatch" style="background: linear-gradient({gradientAngle(form.accentGradientAngle)}deg, {form.accentColor || '#5B8AF5'}, {form.accentColor2 || '#C15BF5'})"></div>
              <span class="color-hex">{form.accentColor || '#5B8AF5'} → {form.accentColor2 || '#C15BF5'}</span>
            {:else}
              <div class="color-swatch" style="background: {form.accentColor || '#5B8AF5'}"></div>
              <span class="color-hex">{form.accentColor || '#5B8AF5'}</span>
            {/if}
            <button class="color-pick-btn" on:click={() => showColorPicker = true}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="4"/><line x1="12" y1="2" x2="12" y2="4"/><line x1="12" y1="20" x2="12" y2="22"/><line x1="2" y1="12" x2="4" y2="12"/><line x1="20" y1="12" x2="22" y2="12"/></svg>
              {$t('accentColor')}
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- Transfers -->
    <section>
      <h3>{$t('transfers')}</h3>
      <div class="card">
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('maxConcurrent')} <button type="button" class="help-btn hint-btn" aria-label={$t('maxConcurrentHint')} on:mouseenter={(e) => openHint(e, 'maxConcurrentHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'maxConcurrentHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.maxConcurrentTransfers === DEFAULTS.maxConcurrentTransfers} on:click|stopPropagation={() => resetSetting('maxConcurrentTransfers')} title={$t('resetToDefault')}>↺</button></label>
          <div class="num-input">
            <button class="num-btn" on:click={() => step('maxConcurrentTransfers', -1, 1, 10)}>−</button>
            <input type="number" bind:value={form.maxConcurrentTransfers} min="1" max="10" />
            <button class="num-btn" on:click={() => step('maxConcurrentTransfers', 1, 1, 10)}>+</button>
          </div>
        </div>
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('transferSpeedLimit')} <button type="button" class="help-btn hint-btn" aria-label={$t('transferSpeedLimitHint')} on:mouseenter={(e) => openHint(e, 'transferSpeedLimitHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'transferSpeedLimitHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.maxTransferSpeedKBps === DEFAULTS.maxTransferSpeedKBps} on:click|stopPropagation={() => resetSetting('maxTransferSpeedKBps')} title={$t('resetToDefault')}>↺</button></label>
          <div class="num-input">
            <button class="num-btn" on:click={() => step('maxTransferSpeedKBps', -100, 0, 100000)}>−</button>
            <input type="number" bind:value={form.maxTransferSpeedKBps} min="0" max="100000" style="width: 80px" />
            <button class="num-btn" on:click={() => step('maxTransferSpeedKBps', 100, 0, 100000)}>+</button>
          </div>
        </div>
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('defaultLocalDir')} <button type="button" class="help-btn hint-btn" aria-label={$t('defaultLocalDirHint')} on:mouseenter={(e) => openHint(e, 'defaultLocalDirHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'defaultLocalDirHint')} on:blur={closeHint}>?</button></label>
          <div class="input-with-btn">
            <input type="text" bind:value={form.defaultLocalDir} />
            <button class="browse-btn" on:click={browseDir}>…</button>
          </div>
        </div>
      </div>
    </section>

    <!-- Notifications -->
    <section>
      <h3>{$t('notifications')}</h3>
      <div class="card">
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('notificationSoundEnabled')} <button type="button" class="help-btn hint-btn" aria-label={$t('notificationSoundEnabledHint')} on:mouseenter={(e) => openHint(e, 'notificationSoundEnabledHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'notificationSoundEnabledHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.notificationSoundEnabled === DEFAULTS.notificationSoundEnabled} on:click|stopPropagation={() => resetSetting('notificationSoundEnabled')} title={$t('resetToDefault')}>↺</button></label>
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
            <label on:click|self|preventDefault>{$t('notificationSound')} <button type="button" class="help-btn hint-btn" aria-label={$t('notificationSoundHint')} on:mouseenter={(e) => openHint(e, 'notificationSoundHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'notificationSoundHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.notificationSound === DEFAULTS.notificationSound} on:click|stopPropagation={() => resetSetting('notificationSound')} title={$t('resetToDefault')}>↺</button></label>
            <div class="sound-row">
              <select bind:value={form.notificationSound}>
                {#each sortedSounds as sound (sound.id)}
                  <option value={sound.id}>{$t(sound.label)}</option>
                {/each}
                {#if form.customSoundName}
                  <option value={CUSTOM_SOUND_ID}>{$t('soundCustom')} : {form.customSoundName}</option>
                {/if}
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
              <button
                type="button"
                class="sound-test-btn"
                title={$t('customSoundLoad').replace('{n}', MAX_SOUND_SECONDS)}
                on:click={loadCustomSound}
                disabled={loadingSound}
              >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M9 18V5l12-2v13"/>
                  <circle cx="6" cy="18" r="3"/>
                  <circle cx="18" cy="16" r="3"/>
                </svg>
              </button>
              {#if form.customSoundName}
                <button
                  type="button"
                  class="sound-test-btn sound-remove-btn"
                  title={$t('customSoundRemove')}
                  on:click={removeCustomSound}
                >
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/></svg>
                </button>
              {/if}
            </div>
          </div>
        {/if}
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('queueDoneBubbleSetting')} <button type="button" class="help-btn hint-btn" aria-label={$t('queueDoneBubbleSettingHint')} on:mouseenter={(e) => openHint(e, 'queueDoneBubbleSettingHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'queueDoneBubbleSettingHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.queueDoneBubble === DEFAULTS.queueDoneBubble} on:click|stopPropagation={() => resetSetting('queueDoneBubble')} title={$t('resetToDefault')}>↺</button></label>
          <button
            type="button"
            class="sw"
            class:on={form.queueDoneBubble}
            on:click={() => toggle('queueDoneBubble')}
            aria-pressed={form.queueDoneBubble}
          ><span class="sw-knob"></span></button>
        </div>
      </div>
    </section>

    <!-- Connection -->
    <section>
      <h3>{$t('connection')}</h3>
      <div class="card">
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('defaultPort')} <button type="button" class="help-btn hint-btn" aria-label={$t('defaultPortHint')} on:mouseenter={(e) => openHint(e, 'defaultPortHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'defaultPortHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.defaultPort === DEFAULTS.defaultPort} on:click|stopPropagation={() => resetSetting('defaultPort')} title={$t('resetToDefault')}>↺</button></label>
          <div class="num-input">
            <button class="num-btn" on:click={() => stepFtpPort(-1)}>−</button>
            <input type="number" bind:value={form.defaultPort} min="1" max="65535" />
            <button class="num-btn" on:click={() => stepFtpPort(1)}>+</button>
          </div>
        </div>
        <div class="setting-row">
          <!-- |self|preventDefault: clicking the text must not "click" the buttons inside -->
          <label on:click|self|preventDefault>{$t('timeout')} <button type="button" class="help-btn hint-btn" aria-label={$t('timeoutHint')} on:mouseenter={(e) => openHint(e, 'timeoutHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'timeoutHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.connectionTimeoutSec === DEFAULTS.connectionTimeoutSec} on:click|stopPropagation={() => resetSetting('connectionTimeoutSec')} title={$t('resetToDefault')}>↺</button></label>
          <div class="num-input">
            <button class="num-btn" on:click={() => step('connectionTimeoutSec', -5, 5, 300)}>−</button>
            <input type="number" bind:value={form.connectionTimeoutSec} min="5" max="300" />
            <button class="num-btn" on:click={() => step('connectionTimeoutSec', 5, 5, 300)}>+</button>
          </div>
        </div>
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('maxConnections')} <button type="button" class="help-btn hint-btn" aria-label={$t('maxConnectionsHint')} on:mouseenter={(e) => openHint(e, 'maxConnectionsHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'maxConnectionsHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.maxConnections === DEFAULTS.maxConnections} on:click|stopPropagation={() => resetSetting('maxConnections')} title={$t('resetToDefault')}>↺</button></label>
          <div class="num-input">
            <button class="num-btn" on:click={() => step('maxConnections', -1, 1, 5)}>−</button>
            <input type="number" bind:value={form.maxConnections} min="1" max="5" />
            <button class="num-btn" on:click={() => step('maxConnections', 1, 1, 5)}>+</button>
          </div>
        </div>
      </div>
    </section>

    <!-- Interface -->
    <section>
      <h3>{$t('interface')}</h3>
      <div class="card">
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('connectCardShadow')} <button type="button" class="help-btn hint-btn" aria-label={$t('connectCardShadowHint')} on:mouseenter={(e) => openHint(e, 'connectCardShadowHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'connectCardShadowHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.connectCardShadow === DEFAULTS.connectCardShadow} on:click|stopPropagation={() => resetSetting('connectCardShadow')} title={$t('resetToDefault')}>↺</button></label>
          <button
            type="button"
            class="sw"
            class:on={form.connectCardShadow}
            on:click={() => toggle('connectCardShadow')}
            aria-pressed={form.connectCardShadow}
          ><span class="sw-knob"></span></button>
        </div>
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('closeSiteManagerOnClickOutside')} <button type="button" class="help-btn hint-btn" aria-label={$t('closeSiteManagerOnClickOutsideHint')} on:mouseenter={(e) => openHint(e, 'closeSiteManagerOnClickOutsideHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'closeSiteManagerOnClickOutsideHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.closeSiteManagerOnClickOutside === DEFAULTS.closeSiteManagerOnClickOutside} on:click|stopPropagation={() => resetSetting('closeSiteManagerOnClickOutside')} title={$t('resetToDefault')}>↺</button></label>
          <button
            type="button"
            class="sw"
            class:on={form.closeSiteManagerOnClickOutside}
            on:click={() => toggle('closeSiteManagerOnClickOutside')}
            aria-pressed={form.closeSiteManagerOnClickOutside}
          ><span class="sw-knob"></span></button>
        </div>
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('doubleClickNavigateUp')} <button type="button" class="help-btn hint-btn" aria-label={$t('doubleClickNavigateUpHint')} on:mouseenter={(e) => openHint(e, 'doubleClickNavigateUpHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'doubleClickNavigateUpHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.doubleClickNavigateUp === DEFAULTS.doubleClickNavigateUp} on:click|stopPropagation={() => resetSetting('doubleClickNavigateUp')} title={$t('resetToDefault')}>↺</button></label>
          <button
            type="button"
            class="sw"
            class:on={form.doubleClickNavigateUp}
            on:click={() => toggle('doubleClickNavigateUp')}
            aria-pressed={form.doubleClickNavigateUp}
          ><span class="sw-knob"></span></button>
        </div>
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('showHiddenFiles')} <button type="button" class="help-btn hint-btn" aria-label={$t('showHiddenFilesHint')} on:mouseenter={(e) => openHint(e, 'showHiddenFilesHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'showHiddenFilesHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.showHiddenFiles === DEFAULTS.showHiddenFiles} on:click|stopPropagation={() => resetSetting('showHiddenFiles')} title={$t('resetToDefault')}>↺</button></label>
          <button
            type="button"
            class="sw"
            class:on={form.showHiddenFiles}
            on:click={() => toggle('showHiddenFiles')}
            aria-pressed={form.showHiddenFiles}
          ><span class="sw-knob"></span></button>
        </div>
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('caseSensitiveSort')} <button type="button" class="help-btn hint-btn" aria-label={$t('caseSensitiveSortHint')} on:mouseenter={(e) => openHint(e, 'caseSensitiveSortHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'caseSensitiveSortHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.caseSensitiveSort === DEFAULTS.caseSensitiveSort} on:click|stopPropagation={() => resetSetting('caseSensitiveSort')} title={$t('resetToDefault')}>↺</button></label>
          <button
            type="button"
            class="sw"
            class:on={form.caseSensitiveSort}
            on:click={() => toggle('caseSensitiveSort')}
            aria-pressed={form.caseSensitiveSort}
          ><span class="sw-knob"></span></button>
        </div>
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('confirmOnDelete')} <button type="button" class="help-btn hint-btn" aria-label={$t('confirmOnDeleteHint')} on:mouseenter={(e) => openHint(e, 'confirmOnDeleteHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'confirmOnDeleteHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.confirmOnDelete === DEFAULTS.confirmOnDelete} on:click|stopPropagation={() => resetSetting('confirmOnDelete')} title={$t('resetToDefault')}>↺</button></label>
          <button
            type="button"
            class="sw"
            class:on={form.confirmOnDelete}
            on:click={() => toggle('confirmOnDelete')}
            aria-pressed={form.confirmOnDelete}
          ><span class="sw-knob"></span></button>
        </div>
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('dateFormat')} <button type="button" class="help-btn hint-btn" aria-label={$t('dateFormatHint')} on:mouseenter={(e) => openHint(e, 'dateFormatHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'dateFormatHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.dateFormat === DEFAULTS.dateFormat} on:click|stopPropagation={() => resetSetting('dateFormat')} title={$t('resetToDefault')}>↺</button></label>
          <select class="date-select" value={dateAdvanced ? DATE_ADVANCED : form.dateFormat} on:change={onDatePresetChange} on:focus={() => now = new Date()}>
            {#each DATE_FORMAT_PRESETS as preset}
              <option value={preset}>
                {preset === SYSTEM_DATE_FORMAT ? `${$t('dateFormatSystem')} (${formatDateWith(now, preset, $locale)})` : formatDateWith(now, preset, $locale)}
              </option>
            {/each}
            <option value={DATE_ADVANCED}>{$t('dateFormatAdvanced')}</option>
          </select>
        </div>
        {#if dateAdvanced}
          <div class="setting-row">
            <label>{$t('dateFormatCustom')}</label>
            <div class="date-custom">
              <input type="text" bind:value={form.dateFormat} placeholder={DEFAULT_DATE_FORMAT} spellcheck="false" />
              <button
                type="button"
                class="info-btn"
                aria-label={$t('dateFormatTokens')}
                on:mouseenter={openDateTip}
                on:mouseleave={() => showDateTip = false}
                on:focus={openDateTip}
                on:blur={() => showDateTip = false}
              >i</button>
            </div>
          </div>
          <div class="date-preview">
            {$t('dateFormatPreview')} : <strong>{formatDateWith(now, form.dateFormat, $locale) || '-'}</strong>
          </div>
        {/if}
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('startMaximized')} <button type="button" class="help-btn hint-btn" aria-label={$t('startMaximizedHint')} on:mouseenter={(e) => openHint(e, 'startMaximizedHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'startMaximizedHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.startMaximized === DEFAULTS.startMaximized} on:click|stopPropagation={() => resetSetting('startMaximized')} title={$t('resetToDefault')}>↺</button></label>
          <button
            type="button"
            class="sw"
            class:on={form.startMaximized}
            on:click={() => toggle('startMaximized')}
            aria-pressed={form.startMaximized}
          ><span class="sw-knob"></span></button>
        </div>
        <div class="setting-row" class:row-disabled={form.startMaximized}>
          <label>{$t('windowWidth')} <button class="reset-btn" hidden={form.windowWidth === DEFAULTS.windowWidth} on:click|stopPropagation={() => resetSetting('windowWidth')} title={$t('resetToDefault')}>↺</button></label>
          <div class="num-input">
            <button class="num-btn" disabled={form.startMaximized} on:click={() => step('windowWidth', -50, 800, 3840)}>−</button>
            <input type="number" bind:value={form.windowWidth} min="800" max="3840" disabled={form.startMaximized} />
            <button class="num-btn" disabled={form.startMaximized} on:click={() => step('windowWidth', 50, 800, 3840)}>+</button>
          </div>
        </div>
        <div class="setting-row" class:row-disabled={form.startMaximized}>
          <label>{$t('windowHeight')} <button class="reset-btn" hidden={form.windowHeight === DEFAULTS.windowHeight} on:click|stopPropagation={() => resetSetting('windowHeight')} title={$t('resetToDefault')}>↺</button></label>
          <div class="num-input">
            <button class="num-btn" disabled={form.startMaximized} on:click={() => step('windowHeight', -50, 600, 2160)}>−</button>
            <input type="number" bind:value={form.windowHeight} min="600" max="2160" disabled={form.startMaximized} />
            <button class="num-btn" disabled={form.startMaximized} on:click={() => step('windowHeight', 50, 600, 2160)}>+</button>
          </div>
        </div>
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('showWindowSizeBadge')} <button type="button" class="help-btn hint-btn" aria-label={$t('showWindowSizeBadgeHint')} on:mouseenter={(e) => openHint(e, 'showWindowSizeBadgeHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'showWindowSizeBadgeHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.showWindowSizeBadge === DEFAULTS.showWindowSizeBadge} on:click|stopPropagation={() => resetSetting('showWindowSizeBadge')} title={$t('resetToDefault')}>↺</button></label>
          <button
            type="button"
            class="sw"
            class:on={form.showWindowSizeBadge}
            on:click={() => toggle('showWindowSizeBadge')}
            aria-pressed={form.showWindowSizeBadge}
          ><span class="sw-knob"></span></button>
        </div>
        <div class="setting-row" class:row-disabled={form.startMaximized}>
          <label on:click|self|preventDefault>{$t('windowSizeCurrent')} <button type="button" class="help-btn hint-btn" aria-label={$t('windowSizeCurrentHint')} on:mouseenter={(e) => openHint(e, 'windowSizeCurrentHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'windowSizeCurrentHint')} on:blur={closeHint}>?</button></label>
          <div class="current-size-row">
            {#if sizeCopied}<span class="saved-msg">{form.windowWidth} × {form.windowHeight} ✓</span>{/if}
            <button class="browse-btn" disabled={form.startMaximized} on:click={useCurrentWindowSize}>{$t('useCurrentWindowSize')}</button>
          </div>
        </div>
      </div>
    </section>

    <!-- Panel comparison (v1.8.0 point 5) -->
    <section>
      <!-- The ? sits next to the h3, not inside: the h3's gradient text would make it transparent -->
      <div class="section-title">
        <h3>{$t('compareSection')}</h3>
        <button type="button" class="help-btn hint-btn" aria-label={$t('compareSectionHint')} on:mouseenter={(e) => openHint(e, 'compareSectionHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'compareSectionHint')} on:blur={closeHint}>?</button>
      </div>
      <div class="card">
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('compareBy')} <button type="button" class="help-btn hint-btn" aria-label={$t('compareByHint')} on:mouseenter={(e) => openHint(e, 'compareByHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'compareByHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.compareBy === DEFAULTS.compareBy} on:click|stopPropagation={() => resetSetting('compareBy')} title={$t('resetToDefault')}>↺</button></label>
          <select class="date-select" bind:value={form.compareBy}>
            <option value="size">{$t('compareBySize')}</option>
            <option value="date">{$t('compareByDate')}</option>
          </select>
        </div>
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('compareHideIdentical')} <button type="button" class="help-btn hint-btn" aria-label={$t('compareHideIdenticalHint')} on:mouseenter={(e) => openHint(e, 'compareHideIdenticalHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'compareHideIdenticalHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.compareHideIdentical === DEFAULTS.compareHideIdentical} on:click|stopPropagation={() => resetSetting('compareHideIdentical')} title={$t('resetToDefault')}>↺</button></label>
          <button
            type="button"
            class="sw"
            class:on={form.compareHideIdentical}
            on:click={() => toggle('compareHideIdentical')}
            aria-pressed={form.compareHideIdentical}
          ><span class="sw-knob"></span></button>
        </div>
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('compareAlign')} <button type="button" class="help-btn hint-btn" aria-label={$t('compareAlignHint')} on:mouseenter={(e) => openHint(e, 'compareAlignHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'compareAlignHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.compareAlign === DEFAULTS.compareAlign} on:click|stopPropagation={() => resetSetting('compareAlign')} title={$t('resetToDefault')}>↺</button></label>
          <button
            type="button"
            class="sw"
            class:on={form.compareAlign}
            on:click={() => toggle('compareAlign')}
            aria-pressed={form.compareAlign}
          ><span class="sw-knob"></span></button>
        </div>
      </div>
    </section>

    <!-- Synchronized browsing (v1.8.0 point 6) -->
    <section>
      <!-- The ? sits next to the h3, not inside: the h3's gradient text would make it transparent -->
      <div class="section-title">
        <h3>{$t('syncSection')}</h3>
        <button type="button" class="help-btn hint-btn" aria-label={$t('syncSectionHint')} on:mouseenter={(e) => openHint(e, 'syncSectionHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'syncSectionHint')} on:blur={closeHint}>?</button>
      </div>
      <div class="card">
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('syncCreateMissing')} <button type="button" class="help-btn hint-btn" aria-label={$t('syncCreateMissingHint')} on:mouseenter={(e) => openHint(e, 'syncCreateMissingHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'syncCreateMissingHint')} on:blur={closeHint}>?</button> <button class="reset-btn" hidden={form.syncCreateMissing === DEFAULTS.syncCreateMissing} on:click|stopPropagation={() => resetSetting('syncCreateMissing')} title={$t('resetToDefault')}>↺</button></label>
          <button
            type="button"
            class="sw"
            class:on={form.syncCreateMissing}
            on:click={() => toggle('syncCreateMissing')}
            aria-pressed={form.syncCreateMissing}
          ><span class="sw-knob"></span></button>
        </div>
      </div>
    </section>

    <!-- Search -->
    <section>
      <h3>{$t('searchSection')}</h3>
      <div class="card">
        <div class="setting-row">
          <!-- |self|preventDefault: clicking the text must not "click" the ? button inside -->
          <label on:click|self|preventDefault>
            {$t('searchIndexEnabled')}
            <button type="button" class="help-btn icon-help-btn" on:click|stopPropagation={() => showSearchIndexHelp = true} aria-label={$t('searchIndexHelp')}><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M2 4h6a4 4 0 0 1 4 4v13a3 3 0 0 0-3-3H2z"/><path d="M22 4h-6a4 4 0 0 0-4 4v13a3 3 0 0 1 3-3h7z"/></svg></button>
            <button class="reset-btn" hidden={form.searchIndexEnabled === DEFAULTS.searchIndexEnabled} on:click|stopPropagation={() => resetSetting('searchIndexEnabled')} title={$t('resetToDefault')}>↺</button>
          </label>
          <button
            type="button"
            class="sw"
            class:on={form.searchIndexEnabled}
            on:click={() => toggle('searchIndexEnabled')}
            aria-pressed={form.searchIndexEnabled}
          ><span class="sw-knob"></span></button>
        </div>
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('searchIndexCache')} <span class="cache-size">({formatBytes(searchIndexSize)})</span> <button type="button" class="help-btn hint-btn" aria-label={$t('searchIndexCacheHint')} on:mouseenter={(e) => openHint(e, 'searchIndexCacheHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'searchIndexCacheHint')} on:blur={closeHint}>?</button></label>
          <button class="browse-btn" disabled={searchIndexSize === 0} on:click={clearSearchIndex}>{$t('searchIndexClear')}</button>
        </div>
      </div>
    </section>

    <!-- Updates -->
    <section>
      <h3>{$t('updatesSection')}</h3>
      <div class="card">
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('checkUpdates')} <button type="button" class="help-btn hint-btn" aria-label={$t('checkUpdatesHint')} on:mouseenter={(e) => openHint(e, 'checkUpdatesHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'checkUpdatesHint')} on:blur={closeHint}>?</button><button type="button" class="help-btn test-banner-btn" disabled={checkingUpdate} on:click|stopPropagation={testUpdateBanner} title={$t('updateTestBannerBtn')} aria-label={$t('updateTestBannerBtn')}><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg></button> <button class="reset-btn" hidden={form.checkUpdates === DEFAULTS.checkUpdates} on:click|stopPropagation={() => resetSetting('checkUpdates')} title={$t('resetToDefault')}>↺</button></label>
          <button
            type="button"
            class="sw"
            class:on={form.checkUpdates}
            on:click={() => toggle('checkUpdates')}
            aria-pressed={form.checkUpdates}
          ><span class="sw-knob"></span></button>
        </div>
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('currentVersion')} <span class="cache-size">v{appVersion}</span> <button type="button" class="help-btn hint-btn" aria-label={$t('currentVersionHint')} on:mouseenter={(e) => openHint(e, 'currentVersionHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'currentVersionHint')} on:blur={closeHint}>?</button></label>
          <button class="browse-btn" disabled={checkingUpdate} on:click={checkUpdateNow}>
            {checkingUpdate ? $t('updateChecking') : $t('checkUpdatesNow')}
          </button>
        </div>
        <div class="setting-row">
          <label on:click|self|preventDefault>{$t('releaseNotes')} <button type="button" class="help-btn hint-btn" aria-label={$t('releaseNotesHint')} on:mouseenter={(e) => openHint(e, 'releaseNotesHint')} on:mouseleave={closeHint} on:focus={(e) => openHint(e, 'releaseNotesHint')} on:blur={closeHint}>?</button></label>
          <button class="browse-btn" disabled={!appVersion} on:click={() => openReleaseNotes(appVersion)}>{$t('releaseNotesShow')}</button>
        </div>
      </div>
    </section>

  </div>

  <div class="panel-footer">
    <button class="version-badge" disabled={!appVersion} on:click={() => openReleaseNotes(appVersion)} title={$t('releaseNotesShowTitle')}>v{appVersion}</button>
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
  </div>
</div>

<!-- Light theme warning ("flashbang mode") - Escape stays here, it must not close Settings -->
{#if confirmFlashbang}
  <div class="flash-overlay" on:click|self={() => answerFlashbang(false)} on:keydown={(e) => { if (e.key === 'Escape') { e.stopPropagation(); answerFlashbang(false); } }}>
    <div class="flash-box" use:trapFocus role="dialog" aria-modal="true">
      <div class="flash-icon">😎</div>
      <div class="flash-title">{$t('flashbangTitle')}</div>
      <div class="flash-msg">{$t('flashbangMsg')}</div>
      <div class="flash-actions">
        <button class="flash-no" use:focusOnOpen on:click={() => answerFlashbang(false)}>{$t('flashbangNo')}</button>
        <button class="flash-yes" on:click={() => answerFlashbang(true)}>{$t('flashbangYes')}</button>
      </div>
    </div>
  </div>
{/if}

{#if showDateTip}
  <div
    class="date-tip"
    class:above={dateTipPos.above}
    style="left: {dateTipPos.x}px; top: {dateTipPos.y}px"
  >
    <div class="date-tip-title">{$t('dateFormatTokens')}</div>
    <div class="date-tip-grid">
      {#each DATE_FORMAT_TOKENS as tok}
        <code>{tok.token}</code>
        <span>{$t(tok.label)}</span>
      {/each}
    </div>
    <div class="date-tip-example">
      <code>%dd/%MM/%yyyy %hh:%mm</code> → {formatDateWith(now, '%dd/%MM/%yyyy %hh:%mm', $locale)}
    </div>
  </div>
{/if}

{#if hint}
  <div class="hint-tip" class:above={hint.above} style="left: {hint.x}px; top: {hint.y}px">{hint.text}</div>
{/if}

{#if showSearchIndexHelp}
  <!-- Option off: same choice as the first-launch popup (Activer / Non merci).
       Option on: explanations only (Fermer). -->
  <SearchIndexModal
    mode={form.searchIndexEnabled ? 'info' : 'intro'}
    onEnable={enableSearchIndexFromHelp}
    onDecline={() => showSearchIndexHelp = false}
    onClose={() => showSearchIndexHelp = false}
  />
{/if}

{#if showColorPicker}
  <ColorPicker
    value={form.accentColor || '#5B8AF5'}
    gradient={!!form.accentGradient}
    value2={form.accentColor2 || '#C15BF5'}
    angle={gradientAngle(form.accentGradientAngle)}
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
  gap: 26px;
  /* darker than the cards so each section stands out as a block */
  background: var(--bg-primary);
}

h3 {
  font-size: 11.5px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: var(--accent);
  margin: 0 0 10px 4px;
}

/* Section title with a "?" hint next to it */
.section-title {
  display: flex;
  align-items: center;
  gap: 2px;
  margin: 0 0 10px 4px;
}
.section-title h3 { margin: 0; }

/* One framed card per section, rows separated inside it */
.card {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
}

.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-height: 52px;
  padding: 10px 16px;
  box-sizing: border-box;
  border-top: 1px solid var(--border);
}
.card > .setting-row:first-child { border-top: none; }

.setting-row label:first-child {
  font-size: 13px;
  font-weight: 600;
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
  background: var(--accent-bg) border-box;
  border-color: transparent;
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
.sw.on { background: var(--accent-bg); }
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
  align-items: stretch;
  height: 32px;
  box-sizing: border-box;
  border: 1px solid var(--border);
  border-radius: 4px;
  overflow: hidden;
  background: var(--bg-input);
}
.num-input:focus-within { border-color: var(--accent); }

/* .num-input input[type="number"] beats the generic input[type="number"] rule further down
   (same specificity before, so its own border, radius and padding won and
   the field sat as a smaller box inside the stepper). */
.num-input input[type="number"] {
  width: 60px;
  height: auto;
  box-sizing: border-box;
  text-align: center;
  background: var(--bg-input);
  border: none;
  border-left: 1px solid var(--border);
  border-right: 1px solid var(--border);
  border-radius: 0;
  color: var(--text-primary);
  padding: 0 4px;
  margin: 0;
  font-size: 13px;
  outline: none;
  -moz-appearance: textfield;
  -webkit-appearance: none;
}
.num-input input[type="number"]:focus { border-color: var(--border); }
.num-input input::-webkit-inner-spin-button,
.num-input input::-webkit-outer-spin-button { -webkit-appearance: none; margin: 0; }

.num-btn {
  background: var(--bg-button);
  border: none;
  color: var(--text-secondary);
  padding: 0 10px;
  height: auto;
  font-size: 16px;
  cursor: pointer;
  transition: background 0.1s, color 0.1s;
  user-select: none;
}
.num-btn:hover { background: var(--bg-button-hover); color: var(--text-primary); }
.num-btn:active { background: var(--accent-bg); color: white; }

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

.help-btn {
  width: 18px;
  height: 18px;
  border-radius: 50%;
  border: 1px solid var(--border);
  background: var(--bg-button);
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  padding: 0;
  margin-left: 4px;
  cursor: pointer;
  vertical-align: middle;
}
.help-btn:hover, .help-btn:focus-visible { color: var(--accent); border-color: var(--accent); outline: none; }
.hint-btn { cursor: help; }
/* "Tester le bandeau" (Mises à jour, eye) and the search index book (opens its window
   on click, no hover text): same round button as "?", with an icon */
.test-banner-btn, .icon-help-btn { display: inline-flex; align-items: center; justify-content: center; }
.test-banner-btn svg, .icon-help-btn svg { width: 11px; height: 11px; }
.test-banner-btn:disabled { opacity: 0.5; cursor: default; }
.hint-tip {
  position: fixed;
  z-index: 600;
  max-width: 320px;
  background: var(--bg-secondary);
  border: 1px solid var(--accent);
  border-radius: 6px;
  box-shadow: 0 6px 24px rgba(0,0,0,0.35);
  padding: 8px 12px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-primary);
  white-space: pre-line;
  pointer-events: none;
}
.hint-tip.above { transform: translateY(-100%); }
.cache-size { color: var(--text-muted); font-size: 12px; }
.browse-btn:disabled { opacity: 0.5; cursor: default; }

.current-size-row { display: flex; align-items: center; gap: 10px; }
.browse-btn:hover:not(:disabled) { background: var(--bg-button-hover); color: var(--accent); }

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
.sound-test-btn:hover:not(:disabled) { background: var(--bg-button-hover); color: var(--accent); }
.sound-test-btn:disabled { opacity: 0.5; cursor: default; }
.sound-remove-btn:hover:not(:disabled) { color: var(--danger); }
.sound-test-btn svg { width: 15px; height: 15px; }

/* ── Date format ── */
.date-select {
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  padding: 5px 8px;
  font-size: 13px;
  outline: none;
  min-width: 200px;
}
.date-select:focus { border-color: var(--accent); }

.date-custom { display: flex; align-items: center; gap: 8px; }
.date-custom input { width: 200px; font-family: monospace; }

.info-btn {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: 1px solid var(--border);
  background: var(--bg-button);
  color: var(--text-secondary);
  font: italic 700 12px/1 Georgia, serif;
  cursor: help;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  flex-shrink: 0;
}
.info-btn:hover, .info-btn:focus-visible { color: var(--accent); border-color: var(--accent); outline: none; }

.date-preview {
  font-size: 12px;
  color: var(--text-muted);
  text-align: right;
  padding: 0 16px 10px;
  margin-top: -4px;
}
.date-preview strong { color: var(--accent); font-weight: 600; }

.date-tip {
  position: fixed;
  transform: translateX(-100%);
  z-index: 600;
  background: var(--bg-secondary);
  border: 1px solid var(--accent);
  border-radius: 6px;
  box-shadow: 0 6px 24px rgba(0,0,0,0.35);
  padding: 10px 12px;
  font-size: 12px;
  color: var(--text-primary);
  pointer-events: none;
  min-width: 260px;
}
.date-tip.above { transform: translate(-100%, -100%); }
.date-tip-title { font-weight: 700; color: var(--accent); margin-bottom: 8px; }
.date-tip-grid {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 3px 14px;
}
.date-tip code { font-family: monospace; color: var(--accent); }
.date-tip-example {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--border-subtle);
  color: var(--text-secondary);
}


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

.version-badge { font-size: 12px; color: var(--accent); background: none; border: none; padding: 0; font-family: inherit; cursor: pointer; }
.version-badge:hover:not(:disabled) { text-decoration: underline; }

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

/* ── Light theme warning ── */
.flash-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 700;
}
.flash-box {
  background: linear-gradient(var(--bg-secondary), var(--bg-secondary)) padding-box,
              var(--accent-gradient) border-box;
  border: 2px solid transparent;
  border-radius: 10px;
  padding: 22px 26px;
  width: 400px;
  max-width: 95vw;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  text-align: center;
  box-shadow: 0 16px 48px rgba(0,0,0,0.5);
}
.flash-icon { font-size: 34px; line-height: 1; }
.flash-title { font-size: 16px; font-weight: 700; color: var(--text-primary); }
.flash-msg { font-size: 13px; color: var(--text-secondary); white-space: pre-line; }
.flash-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  margin-top: 6px;
}
.flash-yes, .flash-no {
  width: 100%;
  border-radius: 6px;
  padding: 8px 14px;
  font-size: 13px;
  cursor: pointer;
}
.flash-no {
  background: var(--accent-bg) border-box;
  border: 1px solid transparent;
  color: white;
  font-weight: 600;
}
.flash-no:hover { background: var(--accent-hover-bg) border-box; }
.flash-yes {
  background: var(--bg-button);
  border: 1px solid var(--border);
  color: var(--text-secondary);
}
.flash-yes:hover { background: var(--bg-button-hover); color: var(--text-primary); }

.btn-primary {
  background: var(--accent-bg);
  border: none;
  border-radius: 5px;
  color: white;
  padding: 8px 20px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
}
.btn-primary:hover { background: var(--accent-hover-bg); }

/* ── Accent gradient mode ── */
:global(html[data-accent-gradient]) h3,
:global(html[data-accent-gradient]) .version-badge,
:global(html[data-accent-gradient]) .date-preview strong,
:global(html[data-accent-gradient]) .date-tip-title,
:global(html[data-accent-gradient]) .date-tip code {
  background: var(--accent-gradient);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
/* block elements: shrink to the text so the whole gradient shows on it */
:global(html[data-accent-gradient]) h3,
:global(html[data-accent-gradient]) .date-tip-title { width: fit-content; }
:global(html[data-accent-gradient]) .date-tip {
  border-color: transparent;
  background: linear-gradient(var(--bg-secondary), var(--bg-secondary)) padding-box, var(--accent-gradient) border-box;
}
</style>
