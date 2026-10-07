import { writable, get } from 'svelte/store';
import { EventsOn } from '../../wailsjs/runtime/runtime.js';
import { GetTransfers, CancelTransfer, CancelAllTransfers, RetryTransfer, ClearTransfers, RemoveTransfer } from '../../wailsjs/go/main/App.js';
import { settings } from './settings.js';
import { playNotificationSound } from '../utils/sound.js';

export const transfers = writable([]);
export const queueVisible = writable(false);
export const completedTransfer = writable(null);

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

export async function retryTransfer(id) {
  await RetryTransfer(id);
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
