import { writable } from 'svelte/store';
import { connectionStatus } from './connection.js';

// v1.8.0 point 5 - panel comparison on/off (Ctrl+O, header button of both
// panels). Session only; the criterion, "hide identical files" and "align
// the rows" are lasting settings (compareBy, compareHideIdentical, compareAlign).
export const compareMode = writable(false);

// Aligned rows: the panel scrolled last ({ side, top }) - the other panel
// follows so the same names stay on the same lines.
export const alignedScroll = writable({ side: null, top: 0 });

connectionStatus.subscribe(s => { if (s !== 'connected') compareMode.set(false); });

export function toggleCompare() {
  compareMode.update(v => !v);
}
