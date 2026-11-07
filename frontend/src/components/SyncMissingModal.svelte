<script>
  // v1.8.0 point 6 - synchronized browsing: the matching folder is missing
  // on the other side (state in stores/syncBrowse.js). Radio choices are
  // buttons (hidden native inputs don't work on WebKit-GTK).
  import { t } from '../i18n/index.js';
  import { trapFocus } from '../utils/focusTrap.js';
  import { syncPrompt, answerSyncPrompt } from '../stores/syncBrowse.js';

  let choice = 'create';
  let lastPrompt = null;
  // Each new question starts on the default choice.
  $: if ($syncPrompt !== lastPrompt) {
    lastPrompt = $syncPrompt;
    choice = 'create';
  }

  $: local = $syncPrompt?.side === 'local';
  $: choices = [
    { key: 'create', label: local ? $t('syncCreateLocal') : $t('syncCreateRemote') },
    { key: 'createAlways', label: $t('syncCreateAlways') },
    { key: 'off', label: local ? $t('syncTurnOffRemote') : $t('syncTurnOffLocal') },
  ];

  function focusOnOpen(node) {
    requestAnimationFrame(() => node.focus());
  }
</script>

{#if $syncPrompt}
  <div class="sm-overlay" on:click|self={() => answerSyncPrompt('cancel')}>
    <div class="sm-box" use:trapFocus role="dialog" aria-modal="true">
      <div class="sm-title">
        {(local ? $t('syncMissingLocal') : $t('syncMissingRemote')).split('{path}')[0]}<span class="sm-path">{$syncPrompt.path}</span>{(local ? $t('syncMissingLocal') : $t('syncMissingRemote')).split('{path}')[1] || ''}
      </div>

      <div class="sm-choices" role="radiogroup">
        {#each choices as c (c.key)}
          <button
            class="sm-choice"
            class:on={choice === c.key}
            role="radio"
            aria-checked={choice === c.key}
            on:click={() => choice = c.key}
            on:dblclick={() => answerSyncPrompt(c.key)}
          >
            <span class="sm-dot"></span>
            <span>{c.label}</span>
          </button>
        {/each}
      </div>

      <div class="sm-actions">
        <button class="sm-cancel" on:click={() => answerSyncPrompt('cancel')}>{$t('cancel')}</button>
        <button class="sm-ok" use:focusOnOpen on:click={() => answerSyncPrompt(choice)}>{$t('validate')}</button>
      </div>
    </div>
  </div>
{/if}

<style>
.sm-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 4800;
}
.sm-box {
  background: linear-gradient(var(--bg-secondary), var(--bg-secondary)) padding-box,
              var(--accent-gradient) border-box;
  border: 2px solid transparent;
  border-radius: 10px;
  padding: 22px 26px;
  width: 480px;
  max-width: 95vw;
  display: flex;
  flex-direction: column;
  gap: 14px;
  box-shadow: 0 16px 48px rgba(0,0,0,0.5);
}
.sm-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  /* Words of the sentence wrap only between words, never in the middle;
     only the path (one long block) may be broken, when it can't fit. */
  overflow-wrap: break-word;
}
.sm-path { color: var(--accent); overflow-wrap: anywhere; }
.sm-choices {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.sm-choice {
  display: flex;
  align-items: center;
  gap: 10px;
  text-align: left;
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 8px 12px;
  color: var(--text-primary);
  font-size: 13px;
  cursor: pointer;
}
.sm-choice:hover { background: var(--bg-button-hover); }
.sm-choice.on { border-color: var(--accent); }
.sm-dot {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  border-radius: 50%;
  border: 2px solid var(--text-muted);
  box-sizing: border-box;
}
.sm-choice.on .sm-dot {
  border-color: var(--accent);
  background: radial-gradient(var(--accent) 0 3px, transparent 3.5px);
}
.sm-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
.sm-cancel, .sm-ok {
  border-radius: 5px;
  padding: 7px 16px;
  font-size: 13px;
  cursor: pointer;
}
.sm-cancel {
  background: var(--bg-button);
  border: 1px solid var(--border);
  color: var(--text-secondary);
}
.sm-cancel:hover { background: var(--bg-button-hover); }
.sm-ok {
  background: var(--accent-bg) border-box;
  border: 1px solid transparent;
  color: white;
  font-weight: 500;
}
.sm-ok:hover { background: var(--accent-hover-bg) border-box; }

:global(html[data-accent-gradient]) .sm-path {
  background: var(--accent-gradient);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
</style>
