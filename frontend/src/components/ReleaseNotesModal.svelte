<script>
  // v1.8.0 - release notes of a version, in the app's language. Opened from
  // Settings > Updates, the Settings footer version badge and the update
  // banner (stores/releaseNotes.js). Mounted once in App.svelte.
  import { t, locale } from '../i18n/index.js';
  import { releaseNotes, closeReleaseNotes } from '../stores/releaseNotes.js';
  import { notesForLocale, renderNotes } from '../utils/releaseNotes.js';
  import { trapFocus } from '../utils/focusTrap.js';

  $: html = $releaseNotes?.md ? renderNotes(notesForLocale($releaseNotes.md, $locale)) : '';
  $: errorText = !$releaseNotes?.error ? ''
    : $releaseNotes.error === 'not_found'
      ? $t('releaseNotesNotFound').replace('{version}', $releaseNotes.version)
      : $t('releaseNotesError').replace('{error}', $releaseNotes.error);
</script>

{#if $releaseNotes}
  <div class="notes-overlay" on:click|self={closeReleaseNotes}>
    <div class="notes-box" use:trapFocus role="dialog" aria-label={$t('releaseNotes')}>
      <div class="notes-header">
        <h2>{$t('releaseNotesTitle').replace('{version}', $releaseNotes.version)}</h2>
        <button class="notes-x" on:click={closeReleaseNotes} title={$t('close')} aria-label={$t('close')}>✕</button>
      </div>
      <div class="notes-body">
        {#if $releaseNotes.loading}
          <div class="notes-state">{$t('releaseNotesLoading')}</div>
        {:else if errorText}
          <div class="notes-state notes-error">{errorText}</div>
        {:else}
          <!-- renderNotes escapes the text before adding its own tags -->
          {@html html}
        {/if}
      </div>
      <div class="notes-footer">
        <button class="notes-close" on:click={closeReleaseNotes} autofocus>{$t('close')}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .notes-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.65);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 4900; /* above Settings, under NotifyModal (5000) */
  }
  .notes-box {
    background: linear-gradient(var(--bg-secondary), var(--bg-secondary)) padding-box,
                var(--accent-gradient) border-box;
    border: 2px solid transparent;
    border-radius: 12px;
    width: 680px;
    max-width: 92vw;
    max-height: 82vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 16px 48px rgba(0, 0, 0, 0.5), 0 0 0 4px var(--accent-subtle);
  }
  .notes-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 16px 20px 12px;
    border-bottom: 1px solid var(--border);
  }
  .notes-header h2 { flex: 1; margin: 0; font-size: 17px; color: var(--text-primary); }
  .notes-x {
    background: none;
    border: none;
    color: var(--text-muted);
    font-size: 15px;
    cursor: pointer;
    padding: 4px 6px;
    border-radius: 4px;
  }
  .notes-x:hover { color: var(--text-primary); background: var(--bg-hover); }
  .notes-body {
    overflow-y: auto;
    padding: 6px 22px 14px;
    font-size: 13.5px;
    line-height: 1.55;
    color: var(--text-primary);
  }
  .notes-body :global(h3) { font-size: 16px; margin: 18px 0 6px; }
  .notes-body :global(h4) { font-size: 14px; margin: 18px 0 6px; color: var(--accent); text-transform: uppercase; letter-spacing: 0.04em; }
  .notes-body :global(h5) { font-size: 14px; margin: 14px 0 4px; }
  .notes-body :global(p) { margin: 0 0 8px; color: var(--text-secondary); }
  .notes-body :global(ul) { margin: 0 0 8px; padding-left: 20px; }
  .notes-body :global(code) { background: var(--bg-hover); padding: 1px 5px; border-radius: 4px; font-size: 12.5px; }
  .notes-body :global(hr) { border: none; border-top: 1px solid var(--border); margin: 14px 0; }
  .notes-state { padding: 28px 0; text-align: center; color: var(--text-muted); }
  .notes-error { color: var(--danger); }
  .notes-footer {
    display: flex;
    justify-content: flex-end;
    padding: 12px 20px 16px;
    border-top: 1px solid var(--border);
  }
  .notes-close {
    background: var(--accent-bg);
    color: white;
    border: none;
    border-radius: 6px;
    padding: 7px 18px;
    font-size: 13px;
    cursor: pointer;
  }
  .notes-close:hover { background: var(--accent-hover-bg); }
</style>
