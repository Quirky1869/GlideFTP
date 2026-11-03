// "12 réussis, 1 échoué, 2 interrompus" for a finished queue
// (stats = { done, failed, paused }, see stores/queueDone.js).
export function queueStatsText(stats, t) {
  const parts = [];
  for (const [key, n] of [['qdDone', stats.done], ['qdFailed', stats.failed], ['qdPaused', stats.paused]]) {
    if (n > 0) parts.push(t(key + (n === 1 ? 'One' : 'Many')).replace('{n}', n));
  }
  return parts.length ? parts.join(', ') : t('qdNothing');
}
