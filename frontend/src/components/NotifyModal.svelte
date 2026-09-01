<script>
  import { t } from '../i18n/index.js';
  import { notification, closeNotify } from '../stores/notify.js';
  import { trapFocus } from '../utils/focusTrap.js';
</script>

{#if $notification}
  <div class="notify-overlay" on:click|self={closeNotify}>
    <div class="notify-box" class:is-error={$notification.type === 'error'} use:trapFocus>
      <div class="notify-message">{$notification.message}</div>
      <button class="notify-close" on:click={closeNotify} autofocus>{$t('close')}</button>
    </div>
  </div>
{/if}

<style>
  .notify-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.65);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 5000;
  }

  .notify-box {
    background: var(--bg-secondary);
    border: 2px solid var(--accent);
    border-radius: 12px;
    padding: 28px 26px 22px;
    width: 380px;
    max-width: 90vw;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 14px;
    text-align: center;
    box-shadow: 0 16px 48px rgba(0, 0, 0, 0.5), 0 0 0 4px var(--accent-subtle);
  }

  .notify-message {
    font-size: 19px;
    font-weight: 700;
    color: var(--text-primary);
    line-height: 1.4;
    white-space: pre-line;
    word-break: break-word;
  }
  .notify-box.is-error .notify-message { color: var(--danger); }

  .notify-close {
    background: var(--accent);
    border: none;
    border-radius: 6px;
    color: white;
    padding: 8px 26px;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
  }
  .notify-close:hover { background: var(--accent-hover); }
</style>
