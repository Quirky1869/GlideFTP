<script>
  // v1.8.0 point 5 - colour legend of the panel comparison, under the
  // panels, only while it is on (FileZilla has none, users had to guess).
  import { t } from '../i18n/index.js';
  import { compareMode, toggleCompare } from '../stores/compare.js';
  import { settings } from '../stores/settings.js';
  import { connectionStatus } from '../stores/connection.js';

  $: byDate = $settings?.compareBy === 'date';
</script>

{#if $compareMode && $connectionStatus === 'connected'}
  <div class="cmp-legend" role="note">
    <span class="cmp-legend-title">{byDate ? $t('cmpLegendByDate') : $t('cmpLegendBySize')}</span>
    <span class="cmp-item"><span class="cmp-swatch only"></span>{$t('cmpLegendOnly')}</span>
    {#if byDate}
      <span class="cmp-item"><span class="cmp-swatch newer"></span>{$t('cmpLegendNewer')}</span>
    {:else}
      <span class="cmp-item"><span class="cmp-swatch size"></span>{$t('cmpLegendSize')}</span>
    {/if}
    {#if $settings?.compareAlign}
      <span class="cmp-hidden">{$t('cmpLegendAligned')}</span>
    {/if}
    {#if $settings?.compareHideIdentical}
      <span class="cmp-hidden">{$t('cmpLegendHidden')}</span>
    {/if}
    <button class="cmp-close" on:click={toggleCompare} title={$t('compareOff') + ' (Ctrl+O)'} aria-label={$t('compareOff')}>✕</button>
  </div>
{/if}

<style>
.cmp-legend {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  flex-shrink: 0;
  padding: 5px 14px;
  background: var(--bg-secondary);
  border-top: 1px solid var(--border);
  font-size: 12px;
  color: var(--text-secondary);
}
.cmp-legend-title { color: var(--text-muted); }
.cmp-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.cmp-swatch {
  width: 14px;
  height: 10px;
  border-radius: 2px;
}
.cmp-swatch.only  { background: var(--cmp-only-bg);  box-shadow: inset 3px 0 0 var(--cmp-only); }
.cmp-swatch.newer { background: var(--cmp-newer-bg); box-shadow: inset 3px 0 0 var(--cmp-newer); }
.cmp-swatch.size  { background: var(--cmp-size-bg);  box-shadow: inset 3px 0 0 var(--cmp-size); }
.cmp-hidden { color: var(--text-muted); font-style: italic; }
.cmp-close {
  margin-left: auto;
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 13px;
  padding: 0 4px;
}
.cmp-close:hover { color: var(--text-primary); }
</style>
