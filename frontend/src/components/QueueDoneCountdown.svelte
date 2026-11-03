<script>
  // Countdown before shutting the computer down / quitting GlideFTP once the
  // transfer queue is finished (stores/queueDone.js). Nothing is sent to the
  // system before it ends: cancelling just stops it.
  import { t } from '../i18n/index.js';
  import { trapFocus } from '../utils/focusTrap.js';
  import { queueDoneCountdown, cancelCountdown, runCountdownNow } from '../stores/queueDone.js';
  import { queueStatsText } from '../utils/queueStats.js';

  function focusOnOpen(node) {
    requestAnimationFrame(() => node.focus());
  }

  $: c = $queueDoneCountdown;
  $: shutdown = c?.action === 'shutdown';
</script>

{#if c}
  <div class="qc-overlay">
    <div
      class="qc-box"
      role="dialog"
      aria-modal="true"
      use:trapFocus
      on:keydown|stopPropagation={(e) => { if (e.key === 'Escape') cancelCountdown(); }}
    >
      <svg class="qc-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
        {#if shutdown}
          <path d="M18.36 6.64a9 9 0 1 1-12.73 0"/><line x1="12" y1="2" x2="12" y2="12"/>
        {:else}
          <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/>
        {/if}
      </svg>
      <div class="qc-title">{shutdown ? $t('qdShutdownIn') : $t('qdQuitIn')}</div>
      <div class="qc-seconds">{c.left}<span> s</span></div>
      <div class="qc-bar"><div class="qc-fill" style="width: {(c.left / c.total) * 100}%"></div></div>
      <div class="qc-stats">{$t('qdQueueFinished')} : {queueStatsText(c.stats, $t)}</div>
      {#if c.unsent > 0}
        <div class="qc-warn">{$t('qdUnsentFiles').replace('{n}', c.unsent)}</div>
      {/if}
      <div class="qc-actions">
        <button class="qc-cancel" use:focusOnOpen on:click={cancelCountdown}>
          {shutdown ? $t('qdCancelShutdown') : $t('qdCancelQuit')}
        </button>
        <button class="qc-now" on:click={runCountdownNow}>
          {shutdown ? $t('qdShutdownNow') : $t('qdQuitNow')}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
.qc-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 4950;
}
.qc-box {
  background: linear-gradient(var(--bg-secondary), var(--bg-secondary)) padding-box,
              var(--accent-gradient) border-box;
  border: 2px solid transparent;
  border-radius: 12px;
  padding: 26px 32px 24px;
  width: 420px;
  max-width: 95vw;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  text-align: center;
  box-shadow: 0 16px 48px rgba(0,0,0,0.5);
}
.qc-icon { width: 34px; height: 34px; color: var(--accent); }
.qc-title { font-size: 15px; font-weight: 600; color: var(--text-primary); }
.qc-seconds {
  font-size: 46px;
  font-weight: 700;
  line-height: 1;
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
}
.qc-seconds span { font-size: 18px; font-weight: 500; color: var(--text-secondary); }
.qc-bar {
  width: 100%;
  height: 5px;
  border-radius: 3px;
  background: var(--bg-primary);
  overflow: hidden;
}
.qc-fill {
  height: 100%;
  background: var(--accent-bg);
  transition: width 1s linear;
}
.qc-stats { font-size: 12px; color: var(--text-secondary); }
.qc-warn { font-size: 12px; color: #E8A33D; }
.qc-actions {
  display: flex;
  gap: 10px;
  margin-top: 8px;
}
.qc-cancel, .qc-now {
  border-radius: 6px;
  padding: 8px 18px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
}
.qc-cancel {
  background: var(--danger);
  border: 1px solid var(--danger);
  color: white;
}
.qc-cancel:hover { background: var(--danger-hover); }
.qc-now {
  background: var(--bg-button);
  border: 1px solid var(--border);
  color: var(--text-primary);
}
.qc-now:hover { background: var(--bg-button-hover); }
</style>
