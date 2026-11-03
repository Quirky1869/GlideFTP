<script>
  // In-app bubble at the bottom right when the transfer queue is finished
  // (Settings > Notifications, or ticked for one queue in its right-click
  // menu). Framed with the accent colour (or gradient).
  import { t } from '../i18n/index.js';
  import { queueDoneBubble, closeQueueDoneBubble } from '../stores/queueDone.js';
  import { queueVisible } from '../stores/transfers.js';
  import { queueStatsText } from '../utils/queueStats.js';

  $: b = $queueDoneBubble;
  $: problems = b && (b.failed > 0 || b.paused > 0);

  function showQueue() {
    queueVisible.set(true);
    closeQueueDoneBubble();
  }
</script>

{#if b}
  <div class="qb" class:problems role="status">
    <span class="qb-icon">
      {#if problems}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
      {:else}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
      {/if}
    </span>
    <div class="qb-body">
      <div class="qb-title">{$t('qdQueueFinished')}</div>
      <div class="qb-stats">{queueStatsText(b, $t)}</div>
      <button class="qb-link" on:click={showQueue}>{$t('qdShowQueue')}</button>
    </div>
    <button class="qb-close" on:click={closeQueueDoneBubble} title={$t('close')}>✕</button>
  </div>
{/if}

<style>
.qb {
  position: fixed;
  right: 18px;
  bottom: 18px;
  z-index: 4500;
  display: flex;
  align-items: flex-start;
  gap: 12px;
  min-width: 260px;
  max-width: 360px;
  padding: 14px 14px 12px 16px;
  border-radius: 10px;
  background: linear-gradient(var(--bg-secondary), var(--bg-secondary)) padding-box,
              var(--accent-gradient) border-box;
  border: 2px solid transparent;
  box-shadow: 0 10px 30px rgba(0,0,0,0.45), 0 0 14px var(--accent-glow), 0 0 14px var(--accent-glow-2);
  animation: qb-in 0.25s ease-out;
}
@keyframes qb-in {
  from { opacity: 0; transform: translateY(12px); }
  to   { opacity: 1; transform: none; }
}
.qb-icon {
  flex-shrink: 0;
  width: 26px;
  height: 26px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--success);
  color: white;
}
.qb.problems .qb-icon { background: #E8A33D; }
.qb-icon svg { width: 15px; height: 15px; }
.qb-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.qb-title { font-size: 13px; font-weight: 600; color: var(--text-primary); }
.qb-stats { font-size: 12px; color: var(--text-secondary); }
.qb-link {
  align-self: flex-start;
  margin-top: 4px;
  background: none;
  border: none;
  padding: 0;
  color: var(--accent);
  font-size: 12px;
  cursor: pointer;
}
.qb-link:hover { text-decoration: underline; }
.qb-close {
  flex-shrink: 0;
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 12px;
  cursor: pointer;
  padding: 2px 4px;
}
.qb-close:hover { color: var(--text-primary); }

:global(html[data-accent-gradient]) .qb-link {
  background: var(--accent-gradient);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
</style>
