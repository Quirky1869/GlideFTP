<script>
  // UI of the "Open with default application" feature (stores/openfile.js):
  // download-in-progress box, "file modified - send back?" / conflict prompts,
  // "no application for this file type" dialog, and a discreet toast.
  // Mounted once in App.svelte.
  import { t } from '../i18n/index.js';
  import { trapFocus } from '../utils/focusTrap.js';
  import {
    openingFile, openPrompts, noAppPrompt, openToast,
    cancelOpening, answerPrompt, openContainingFolder, chooseAppAndOpen, downloadFromNoApp,
  } from '../stores/openfile.js';

  $: prompt = $openPrompts[0];

  const fill = (text, vars) => Object.entries(vars).reduce((s, [k, v]) => s.replace(`{${k}}`, v), text);

  function onPromptKeydown(e) {
    if (e.key === 'Escape') answerPrompt(prompt.mode === 'conflict' ? 'cancel' : 'no');
  }
  function onNoAppKeydown(e) {
    if (e.key === 'Escape') noAppPrompt.set(null);
  }
</script>

{#if $openingFile}
  <div class="opening-box">
    <svg class="spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
    <span class="opening-text">{fill($t('openingFile'), { name: $openingFile.name })}</span>
    <button class="btn-secondary" on:click={cancelOpening}>{$t('cancel')}</button>
  </div>
{/if}

{#if prompt}
  <div class="of-overlay">
    <div class="of-box" use:trapFocus on:keydown={onPromptKeydown} role="dialog">
      {#if prompt.mode === 'conflict'}
        <div class="of-title of-title-warn">{$t('remoteFileChangedTitle')}</div>
        <div class="of-message">{fill($t('remoteFileChanged'), { name: prompt.file.name, host: prompt.file.host })}</div>
        <div class="of-actions">
          <button class="btn-primary" disabled={prompt.busy} on:click={() => answerPrompt('overwrite')} autofocus>
            {#if prompt.busy}<svg class="spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>{/if}
            {$t('overwrite')}
          </button>
          <button class="btn-secondary" disabled={prompt.busy} on:click={() => answerPrompt('cancel')}>{$t('cancel')}</button>
        </div>
      {:else}
        <div class="of-title">{$t('fileModifiedTitle')}</div>
        <div class="of-message">{fill($t('fileModifiedUpload'), { name: prompt.file.name, host: prompt.file.host })}</div>
        <div class="of-path" title={prompt.file.remotePath}>{prompt.file.remotePath}</div>
        <div class="of-actions">
          <button class="btn-primary" disabled={prompt.busy} on:click={() => answerPrompt('yes')} autofocus>
            {#if prompt.busy}<svg class="spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>{/if}
            {$t('yes')}
          </button>
          <button class="btn-secondary" disabled={prompt.busy} on:click={() => answerPrompt('no')}>{$t('no')}</button>
          <button class="btn-secondary" disabled={prompt.busy} on:click={() => answerPrompt('always')}>{$t('uploadAlways')}</button>
        </div>
      {/if}
      {#if $openPrompts.length > 1}
        <div class="of-more">{fill($t('openFileMorePending'), { n: $openPrompts.length - 1 })}</div>
      {/if}
    </div>
  </div>
{/if}

{#if $noAppPrompt}
  <div class="of-overlay" on:click|self={() => noAppPrompt.set(null)}>
    <div class="of-box" use:trapFocus on:keydown={onNoAppKeydown} role="dialog">
      <div class="of-title">{$t('noDefaultApp')}</div>
      <div class="of-message">{$noAppPrompt.name}</div>
      <div class="of-actions">
        <button class="btn-primary" on:click={chooseAppAndOpen} autofocus>{$t('chooseApp')}</button>
        {#if $noAppPrompt.remote}
          <button class="btn-secondary" on:click={downloadFromNoApp}>{$t('downloadFile')}</button>
        {/if}
        <button class="btn-secondary" on:click={openContainingFolder}>{$t('openFolder')}</button>
        <button class="btn-secondary" on:click={() => noAppPrompt.set(null)}>{$t('close')}</button>
      </div>
    </div>
  </div>
{/if}

{#if $openToast}
  <div class="of-toast">✓ {$openToast.text}</div>
{/if}

<style>
.of-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.65);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 4800; /* under NotifyModal (5000), so an error notification shows on top */
}

.of-box {
  background: var(--bg-secondary);
  border: 2px solid var(--accent);
  border-radius: 12px;
  padding: 24px 24px 20px;
  width: 440px;
  max-width: 90vw;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  text-align: center;
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.5), 0 0 0 4px var(--accent-subtle);
}

.of-title { font-size: 17px; font-weight: 700; color: var(--text-primary); }
.of-title-warn { color: var(--danger); }
.of-message {
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.45;
  white-space: pre-line;
  word-break: break-word;
}
.of-path {
  font-family: monospace;
  font-size: 12px;
  color: var(--text-muted);
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.of-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
  margin-top: 6px;
}
.of-more { font-size: 12px; color: var(--text-muted); }

.btn-primary, .btn-secondary {
  display: flex;
  align-items: center;
  gap: 6px;
  border-radius: 6px;
  padding: 8px 16px;
  font-size: 13px;
  cursor: pointer;
}
.btn-primary {
  background: var(--accent-bg);
  border: none;
  color: white;
  font-weight: 500;
}
.btn-primary:hover:not(:disabled) { background: var(--accent-hover-bg); }
.btn-secondary {
  background: var(--bg-button);
  border: 1px solid var(--border);
  color: var(--text-secondary);
}
.btn-secondary:hover:not(:disabled) { background: var(--bg-button-hover); color: var(--text-primary); }
.btn-primary:disabled, .btn-secondary:disabled { opacity: 0.6; cursor: default; }

.spinner {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  animation: of-spin 0.75s linear infinite;
}
@keyframes of-spin { to { transform: rotate(360deg); } }

/* Download in progress: small box, not modal - the rest of the UI stays usable */
.opening-box {
  position: fixed;
  left: 50%;
  bottom: 24px;
  transform: translateX(-50%);
  z-index: 4700;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px 10px 16px;
  background: var(--bg-secondary);
  border: 1px solid var(--accent);
  border-radius: 10px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.45);
  color: var(--text-primary);
  font-size: 13px;
  max-width: 80vw;
}
.opening-box .spinner { color: var(--accent); width: 16px; height: 16px; }
.opening-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.opening-box .btn-secondary { padding: 5px 12px; font-size: 12px; }

.of-toast {
  position: fixed;
  right: 20px;
  bottom: 20px;
  z-index: 4700;
  padding: 9px 14px;
  border-radius: 8px;
  background: var(--bg-secondary);
  border: 1px solid var(--success);
  color: var(--success);
  font-size: 13px;
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.35);
  pointer-events: none;
}

/* ── Accent gradient mode (CLAUDE.md accent rules) ── */
:global(html[data-accent-gradient]) .of-box {
  border-color: transparent;
  background: linear-gradient(var(--bg-secondary), var(--bg-secondary)) padding-box, var(--accent-gradient) border-box;
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.5), -3px 0 0 4px var(--accent-subtle), 3px 0 0 4px var(--accent-subtle-2);
}
:global(html[data-accent-gradient]) .opening-box {
  border-color: transparent;
  background: linear-gradient(var(--bg-secondary), var(--bg-secondary)) padding-box, var(--accent-gradient) border-box;
}
</style>
