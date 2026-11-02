<script>
  // v1.8.0 points 2 + 8 - "N transfers in progress" before closing
  // connections or quitting (state in stores/transferGuard.js).
  import { t } from '../i18n/index.js';
  import { trapFocus } from '../utils/focusTrap.js';
  import { transferGuard, answerTransferGuard } from '../stores/transferGuard.js';

  function focusOnOpen(node) {
    requestAnimationFrame(() => node.focus());
  }

  $: g = $transferGuard;
  $: quit = g?.kind === 'quit';
  $: title = !g ? '' : quit
    ? (g.count === 1 ? $t('guardQuitTitleOne') : $t('guardQuitTitleMany').replace('{n}', g.count))
    : (g.count === 1 ? $t('guardTitleOne') : $t('guardTitleMany').replace('{n}', g.count)).replace('{host}', g.hosts.join(', '));
</script>

{#if g}
  <div class="tg-overlay" on:click|self={() => answerTransferGuard('cancel')}>
    <div class="tg-box" use:trapFocus role="dialog" aria-modal="true">
      <div class="tg-title">{title}</div>
      <div class="tg-msg">{$t('guardQuestion')}</div>

      <button class="tg-choice primary" use:focusOnOpen on:click={() => answerTransferGuard('pause')}>
        <span class="tg-label">{quit ? $t('guardQuitPause') : $t('guardPause')}</span>
        <span class="tg-hint">{quit ? $t('guardQuitPauseHint') : $t('guardPauseHint')}</span>
      </button>
      <button class="tg-choice danger" on:click={() => answerTransferGuard('stop')}>
        <span class="tg-label">{quit ? $t('guardQuitStop') : $t('guardStop')}</span>
        <span class="tg-hint">{$t('guardStopHint')}</span>
      </button>

      <div class="tg-actions">
        <button class="tg-cancel" on:click={() => answerTransferGuard('cancel')}>{$t('cancel')}</button>
      </div>
    </div>
  </div>
{/if}

<style>
.tg-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 4800;
}
.tg-box {
  background: linear-gradient(var(--bg-secondary), var(--bg-secondary)) padding-box,
              var(--accent-gradient) border-box;
  border: 2px solid transparent;
  border-radius: 10px;
  padding: 22px 26px;
  width: 440px;
  max-width: 95vw;
  display: flex;
  flex-direction: column;
  gap: 10px;
  box-shadow: 0 16px 48px rgba(0,0,0,0.5);
}
.tg-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}
.tg-msg {
  font-size: 13px;
  color: var(--text-secondary);
  margin-bottom: 4px;
}
.tg-choice {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 3px;
  text-align: left;
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 9px 12px;
  cursor: pointer;
  color: var(--text-primary);
}
.tg-choice:hover { background: var(--bg-button-hover); }
.tg-choice.primary {
  background: var(--accent-bg) border-box;
  border-color: transparent;
  color: white;
}
.tg-choice.primary:hover { background: var(--accent-hover-bg) border-box; }
.tg-choice.primary .tg-hint { color: rgba(255,255,255,0.85); }
.tg-choice.danger .tg-label { color: var(--danger); }
.tg-label { font-size: 13px; font-weight: 600; }
.tg-hint { font-size: 12px; color: var(--text-muted); }
.tg-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 4px;
}
.tg-cancel {
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-secondary);
  padding: 7px 16px;
  font-size: 13px;
  cursor: pointer;
}
.tg-cancel:hover { background: var(--bg-button-hover); }
</style>
