import { writable } from 'svelte/store';
import { GetReleaseNotes } from '../../wailsjs/go/main/App.js';

// { version, loading, md, error } while the release notes window is open.
export const releaseNotes = writable(null);

let seq = 0;

export async function openReleaseNotes(version) {
  const mine = ++seq;
  releaseNotes.set({ version, loading: true, md: '', error: '' });
  try {
    const md = await GetReleaseNotes(version);
    if (mine === seq) releaseNotes.set({ version, loading: false, md, error: '' });
  } catch (e) {
    if (mine === seq) releaseNotes.set({ version, loading: false, md: '', error: e?.message || String(e) });
  }
}

export function closeReleaseNotes() {
  seq++;
  releaseNotes.set(null);
}
