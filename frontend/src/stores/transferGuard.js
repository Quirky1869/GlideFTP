import { writable, get } from 'svelte/store';
import { QuitWithTransfers } from '../../wailsjs/go/main/App.js';
import { transfers, stopConnectionTransfers } from './transfers.js';

// v1.8.0 points 2 + 8 - the question asked before closing connections (or
// quitting) while they still have transfers: Cancel / Stop / Resume later.
// { count, hosts, kind: 'disconnect'|'quit', resolve } while it is shown.
export const transferGuard = writable(null);

const isActive = (j) => j.status === 'pending' || j.status === 'running';

function ask(active, kind) {
  const hosts = [...new Set(active.map(j => j.remoteHost).filter(Boolean))];
  return new Promise(resolve => transferGuard.set({ count: active.length, hosts, kind, resolve }));
}

// choice: 'cancel' | 'stop' | 'pause'
export function answerTransferGuard(choice) {
  const g = get(transferGuard);
  transferGuard.set(null);
  if (g) g.resolve(choice);
}

// Call before closing connections connIds (tab ✕, disconnect, replace).
// Resolves true to go on (their transfers were stopped or put aside) or
// false when the user cancelled - then nothing must change.
export async function guardTransfers(connIds) {
  const ids = connIds.filter(Boolean);
  const active = get(transfers).filter(j => isActive(j) && ids.includes(j.connId));
  if (active.length === 0) return true;
  const choice = await ask(active, 'disconnect');
  if (choice === 'cancel') return false;
  await stopConnectionTransfers(ids, choice === 'pause');
  return true;
}

// Answer to the backend's "app:quitWithTransfers" (window closed with
// transfers still pending or running, see beforeClose in app_openfile.go).
export async function askQuitWithTransfers() {
  const active = get(transfers).filter(isActive);
  const choice = await ask(active, 'quit');
  if (choice === 'cancel') return;
  await QuitWithTransfers(choice === 'pause');
}
