import { writable, get } from 'svelte/store';
import { Quit } from '../../wailsjs/runtime/runtime.js';
import { ShutdownComputer, RunQueueDoneCommand, UnsentOpenedFileCount } from '../../wailsjs/go/main/App.js';
import { settings } from './settings.js';
import { notify } from './notify.js';
import { t } from '../i18n/index.js';
import { playNotificationSound } from '../utils/sound.js';

// What happens once the whole transfer queue is finished (right-click on the
// queue, like FileZilla's "Action after queue completion"). Everything here
// is for the current queue only and never saved: after it fires, it goes
// back to "nothing" - a shutdown still armed at the next launch would be a
// nasty surprise.

// 'none' | 'quit' | 'shutdown' | 'command' (settings.queueDoneCommand)
export const queueDoneAction = writable('none');

// Bubble / sound for this queue only, when they are off in the Settings.
export const queueDoneOnce = writable({ bubble: false, sound: false });

// Countdown before quitting / shutting down: { action, left, total, stats, unsent }.
export const queueDoneCountdown = writable(null);

// Bubble at the bottom right: { done, failed, paused }.
export const queueDoneBubble = writable(null);

const COUNTDOWN_SECONDS = { shutdown: 60, quit: 10 };
const BUBBLE_MS = 8000;

let countdownTimer = null;
let bubbleTimer = null;

// Called by stores/transfers.js when the last pending / running job of the
// queue is over (not after "Cancel all" or transfers stopped on purpose).
// stats = { done, failed, paused } for the jobs of that queue.
export function queueFinished(stats) {
  const s = get(settings) || {};
  const once = get(queueDoneOnce);
  if (s.notificationSoundEnabled || once.sound) playNotificationSound(s.notificationSound);
  if (s.queueDoneBubble || once.bubble) showBubble(stats);
  queueDoneOnce.set({ bubble: false, sound: false });

  const action = get(queueDoneAction);
  queueDoneAction.set('none');
  if (action === 'command') runCommand(s.queueDoneCommand, stats);
  else if (action === 'quit' || action === 'shutdown') startCountdown(action, stats);
}

function showBubble(stats) {
  clearTimeout(bubbleTimer);
  queueDoneBubble.set(stats);
  bubbleTimer = setTimeout(() => queueDoneBubble.set(null), BUBBLE_MS);
}

export function closeQueueDoneBubble() {
  clearTimeout(bubbleTimer);
  queueDoneBubble.set(null);
}

async function runCommand(command, stats) {
  if (!command) return;
  try {
    await RunQueueDoneCommand(command, stats.done, stats.failed);
  } catch (e) {
    notify(get(t)('qdCommandFailed').replace('{error}', e?.message || String(e)), 'error');
  }
}

async function startCountdown(action, stats) {
  let unsent = 0;
  if (action === 'shutdown') {
    try { unsent = await UnsentOpenedFileCount(); } catch {}
  }
  const total = COUNTDOWN_SECONDS[action];
  queueDoneCountdown.set({ action, left: total, total, stats, unsent });
  clearInterval(countdownTimer);
  countdownTimer = setInterval(() => {
    const c = get(queueDoneCountdown);
    if (!c) return clearInterval(countdownTimer);
    if (c.left <= 1) runCountdownNow();
    else queueDoneCountdown.set({ ...c, left: c.left - 1 });
  }, 1000);
}

export function cancelCountdown() {
  clearInterval(countdownTimer);
  queueDoneCountdown.set(null);
}

// End of the countdown, or "Shut down / Quit now".
export async function runCountdownNow() {
  const c = get(queueDoneCountdown);
  cancelCountdown();
  if (!c) return;
  if (c.action === 'quit') {
    Quit(); // beforeClose still asks about opened files with unsent changes
    return;
  }
  try {
    await ShutdownComputer();
  } catch (e) {
    notify(get(t)('qdShutdownFailed').replace('{error}', e?.message || String(e)), 'error');
  }
}
