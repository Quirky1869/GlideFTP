import { writable } from 'svelte/store';
import { EventsOn } from '../../wailsjs/runtime/runtime.js';
import { GetTransfers, CancelTransfer, RetryTransfer, ClearTransfers } from '../../wailsjs/go/main/App.js';

export const transfers = writable([]);
export const queueVisible = writable(false);

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
  });

  EventsOn('transfer:progress', (job) => {
    transfers.update(list => list.map(j => j.id === job.id ? { ...j, bytesDone: job.bytesDone, size: job.size } : j));
  });

  EventsOn('transfer:cleared', (status) => {
    transfers.update(list => list.filter(j => j.status !== status));
  });
}

export function toggleQueue() {
  queueVisible.update(v => !v);
}

export async function cancelTransfer(id) {
  await CancelTransfer(id);
}

export async function retryTransfer(id) {
  await RetryTransfer(id);
}

export async function clearTransfers(status) {
  await ClearTransfers(status);
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
