import { writable, get } from 'svelte/store';
import { EventsOn } from '../../wailsjs/runtime/runtime.js';
import { GetTransfers, CancelTransfer, CancelAllTransfers, RetryTransfer, ClearTransfers, RemoveTransfer, AbandonTransfer, StopConnectionTransfers, ResumePausedTransfers } from '../../wailsjs/go/main/App.js';
import { settings } from './settings.js';
import { playNotificationSound } from '../utils/sound.js';

export const transfers = writable([]);
export const queueVisible = writable(false);
export const completedTransfer = writable(null);
// Connection ids whose "Resume N paused transfers?" bar was put off ("Later").
// Resuming one paused transfer by hand clears it, so the bar comes back for
// the others.
export const resumeBannerDismissed = writable(new Set());

function isActive(job) {
  return job.status === 'pending' || job.status === 'running';
}

// Plays the notification sound once the whole queue has drained (no more
// pending/running jobs) - not after every individual file - so a batch of
// N transfers only makes one sound, right when the last one wraps up.
// A drain caused by "Cancel all" is not a finished batch - no sound then.
let hadActiveJobs = false;
let suppressDrainSound = false;
transfers.subscribe(list => {
  const active = list.some(isActive);
  if (hadActiveJobs && !active) {
    const s = get(settings);
    if (s?.notificationSoundEnabled && !suppressDrainSound) {
      playNotificationSound(s.notificationSound);
    }
    suppressDrainSound = false;
  }
  hadActiveJobs = active;
});

export async function initTransfers() {
  try {
    const list = await GetTransfers();
    transfers.set(list || []);
    // Transfers kept from the last session (queue.json): show them.
    if ((list || []).some(j => j.status === 'paused')) queueVisible.set(true);
  } catch {}

  EventsOn('transfer:added', (job) => {
    transfers.update(list => [...list, job]);
  });

  EventsOn('transfer:update', (job) => {
    transfers.update(list => list.map(j => j.id === job.id ? job : j));
    if (job.status === 'done') {
      completedTransfer.set({ ...job, _ts: Date.now() });
    }
  });

  EventsOn('transfer:progress', (job) => {
    transfers.update(list => list.map(j => j.id === job.id ? { ...j, bytesDone: job.bytesDone, size: job.size } : j));
  });

  // Bulk event from CancelAll: ids of the pending jobs it cancelled, in one
  // store update (one event per job would mean thousands of list rebuilds).
  EventsOn('transfer:cancelledAll', (ids) => {
    if (!ids || ids.length === 0) return;
    const set = new Set(ids);
    const now = new Date().toISOString();
    transfers.update(list => list.map(j => set.has(j.id) ? { ...j, status: 'cancelled', finishedAt: now } : j));
  });

  // Bulk event from the Go queue's stop(): pending jobs of a closed
  // connection fail (error "connection_closed") or are paused, at once.
  EventsOn('transfer:stopped', ({ ids, status, error }) => {
    if (!ids || ids.length === 0) return;
    const set = new Set(ids);
    const now = new Date().toISOString();
    transfers.update(list => list.map(j => set.has(j.id)
      ? { ...j, status, error, ...(status === 'failed' ? { finishedAt: now } : {}) }
      : j));
  });

  // Bulk event from ResumePaused: paused jobs back in the queue on a new connection.
  EventsOn('transfer:requeued', ({ ids, connId, remoteHost }) => {
    if (!ids || ids.length === 0) return;
    const set = new Set(ids);
    transfers.update(list => list.map(j => set.has(j.id) ? { ...j, status: 'pending', error: '', connId, remoteHost } : j));
  });

  EventsOn('transfer:cleared', (status) => {
    transfers.update(list => list.filter(j => j.status !== status));
  });

  EventsOn('transfer:removed', (id) => {
    transfers.update(list => list.filter(j => j.id !== id));
  });
}

export function toggleQueue() {
  queueVisible.update(v => !v);
}

export async function cancelTransfer(id) {
  await CancelTransfer(id);
}

export async function cancelAllTransfers() {
  if (get(transfers).some(isActive)) suppressDrainSound = true;
  await CancelAllTransfers();
}

// resume = continue where the transfer stopped, else start again from zero.
// Rejects with "not_connected" when no connection to its server is open.
export async function retryTransfer(id, resume = false) {
  await RetryTransfer(id, resume);
}

// Drops a failed or paused transfer and deletes its incomplete file.
// Resolves to "remote_kept" when the remote partial file could not be deleted.
export async function abandonTransfer(id) {
  return await AbandonTransfer(id);
}

// Before closing connections: stop their transfers (failed) or put them
// aside (pause = "resume later"). A batch stopped this way is not a
// finished batch - no notification sound.
export async function stopConnectionTransfers(connIds, pause) {
  // Only when it empties the whole queue: transfers of other tabs still
  // running will end with their own sound.
  const active = get(transfers).filter(isActive);
  if (active.length && active.every(j => connIds.includes(j.connId))) suppressDrainSound = true;
  await StopConnectionTransfers(connIds, pause);
}

export async function resumePausedTransfers(connId) {
  return await ResumePausedTransfers(connId);
}

// Same key as favKey / Go's connection.AccountKey: protocol://user@host:port.
export function accountKey(c) {
  return `${c.protocol}://${c.user || ''}@${(c.host || '').toLowerCase()}:${c.port}`;
}

export async function clearTransfers(status) {
  await ClearTransfers(status);
}

export async function removeTransfer(id) {
  await RemoveTransfer(id);
}

export function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

export function progressPct(job) {
  if (!job.size || job.size === 0) return 0;
  return Math.min(100, Math.round((job.bytesDone / job.size) * 100));
}
