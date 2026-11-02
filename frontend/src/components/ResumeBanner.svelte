<script>
  // v1.8.0 point 8 - "N paused transfers for host - Resume?" once connected
  // to a server that has paused transfers (resume later, lost connection,
  // app closed or crashed). Mounted next to UpdateBanner in App.svelte.
  import { t } from '../i18n/index.js';
  import { transfers, resumePausedTransfers, accountKey, queueVisible, resumeBannerDismissed } from '../stores/transfers.js';
  import { connections } from '../stores/connection.js';
  import { notify } from '../stores/notify.js';

  let busy = false;

  function findOffer(conns, jobs, dismissed) {
    for (const c of conns) {
      if (dismissed.has(c.id)) continue;
      const key = accountKey(c);
      const count = jobs.reduce((n, j) => n + (j.status === 'paused' && j.account === key ? 1 : 0), 0);
      if (count > 0) return { connId: c.id, host: c.host, count };
    }
    return null;
  }

  // "Later" hides it for that connection until a paused transfer is resumed
  // by hand (TransferQueue clears resumeBannerDismissed).
  $: offer = findOffer($connections, $transfers, $resumeBannerDismissed);

  async function resume() {
    if (!offer || busy) return;
    busy = true;
    try {
      await resumePausedTransfers(offer.connId);
      queueVisible.set(true);
    } catch (e) {
      notify(e?.toString() || $t('unknownError'), 'error');
    } finally {
      busy = false;
    }
  }

  function later() {
    if (!offer) return;
    resumeBannerDismissed.update(d => new Set([...d, offer.connId]));
  }
</script>

{#if offer}
  <div class="resume-banner" role="status">
    <svg class="resume-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="6 4 20 12 6 20 6 4"/></svg>
    <span class="resume-text">
      {(offer.count === 1 ? $t('resumeBannerOne') : $t('resumeBannerMany').replace('{n}', offer.count)).replace('{host}', offer.host)}
    </span>
    <div class="resume-actions">
      <button class="resume-btn primary" disabled={busy} on:click={resume}>{$t('resumeTransfers')}</button>
      <button class="resume-btn" on:click={later}>{$t('updateLater')}</button>
    </div>
  </div>
{/if}

<style>
.resume-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  flex-shrink: 0;
  padding: 10px 16px;
  background: linear-gradient(var(--bg-secondary), var(--bg-secondary)) padding-box,
              var(--accent-gradient) border-box;
  border-top: 2px solid transparent;
  color: var(--text-primary);
  font-size: 13px;
  animation: resume-in 0.25s ease;
}
@keyframes resume-in {
  from { transform: translateY(100%); opacity: 0; }
  to   { transform: translateY(0);    opacity: 1; }
}
.resume-icon {
  width: 16px;
  height: 16px;
  color: var(--accent);
  flex-shrink: 0;
}
.resume-text { flex: 1; min-width: 200px; }
.resume-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.resume-btn {
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-secondary);
  padding: 5px 12px;
  font-size: 13px;
  cursor: pointer;
  white-space: nowrap;
}
.resume-btn:hover { background: var(--bg-button-hover); color: var(--text-primary); }
.resume-btn.primary {
  background: var(--accent-bg) border-box;
  border-color: transparent;
  color: white;
}
.resume-btn.primary:hover:not(:disabled) { background: var(--accent-hover-bg) border-box; }
.resume-btn:disabled { opacity: 0.6; cursor: default; }
</style>
