import { writable, get } from 'svelte/store';
import { LocalListDir, RemoteListDir } from '../../wailsjs/go/main/App.js';
import {
  localPath, remotePath, localEntries, remoteEntries,
  navigateLocal, refreshRemote, localMkDir, remoteMkDir,
  activeConnectionId, connectionStatus,
} from './connection.js';
import { settings, saveSettings } from './settings.js';

// v1.8.0 point 6 - synchronized browsing (Ctrl+Y, header button of both
// panels). While on, entering a folder (or going up) in one panel makes the
// other one do the same step: down into the same sub-folders, or up as many
// levels. A jump with no clear match on the other side (typed path,
// favourite, search result, unrelated folder) turns it off with a message.
// A folder missing on the other side opens a dialog (create it / create it
// and don't ask again / turn off sync, or cancel the whole navigation),
// unless the syncCreateMissing setting says to create it without asking.
export const syncBrowsing = writable(false);

// Missing-folder dialog: { side, path, resolve } while shown.
// side = the panel where the folder is missing.
export const syncPrompt = writable(null);

// Another server (tab switch, new connection) or no server: the two panels
// no longer show matching folders.
activeConnectionId.subscribe(() => syncBrowsing.set(false));
connectionStatus.subscribe(s => { if (s !== 'connected') syncBrowsing.set(false); });

export function toggleSyncBrowsing() {
  if (get(connectionStatus) !== 'connected') return;
  syncBrowsing.update(v => !v);
}

// choice: 'cancel' | 'create' | 'createAlways' | 'off'
export function answerSyncPrompt(choice) {
  const p = get(syncPrompt);
  syncPrompt.set(null);
  if (p) p.resolve(choice);
}

// ── Paths: remote = "/a/b"; local = "/a/b" (Linux) or "C:\a\b" (Windows) ──

const isWinPath = (p) => /^[A-Za-z]:/.test(p);
const segsOf = (p) => (p || '').split(/[/\\]+/).filter(Boolean);

function pathOf(segs, win) {
  if (win) return segs[0] + '\\' + segs.slice(1).join('\\');
  return '/' + segs.join('/');
}

// Windows names are case-insensitive, Linux and servers are not.
const sameName = (a, b, win) => (win ? a.toLowerCase() === b.toLowerCase() : a === b);

function startsWith(segs, prefix, win) {
  return prefix.length <= segs.length && prefix.every((s, i) => sameName(s, segs[i], win));
}

const other = (side) => (side === 'local' ? 'remote' : 'local');
const pathStore = (side) => (side === 'local' ? localPath : remotePath);
const entriesStore = (side) => (side === 'local' ? localEntries : remoteEntries);

async function listDir(side, p) {
  return (side === 'local' ? await LocalListDir(p) : await RemoteListDir(p)) || [];
}

// Index of the first segment of segs (after the base) that has no folder of
// that name, or -1 when they all exist. The first level is read from the
// panel's own list, deeper ones are listed.
async function firstMissing(side, baseSegs, rel, win) {
  for (let i = 0; i < rel.length; i++) {
    const parentSegs = [...baseSegs, ...rel.slice(0, i)];
    let list;
    if (i === 0) {
      list = get(entriesStore(side));
    } else {
      try { list = await listDir(side, pathOf(parentSegs, win)); } catch { return i; }
    }
    if (!list.some(e => e.isDir && sameName(e.name, rel[i], win))) return i;
  }
  return -1;
}

// Called by a panel before it moves from oldPath to newPath (up = levels for
// ".."; jump = typed path / favourite / search result). Returns:
//   null                         → cancelled: the panel must not move
//   { follow?, notice? }         → move; then await follow() (the other
//                                  panel's step) and show notice if set.
export async function planSync(side, oldPath, newPath, { up = 0, jump = false } = {}) {
  if (!get(syncBrowsing)) return {};
  const offNotice = () => { syncBrowsing.set(false); return { notice: 'syncOffJump' }; };
  if (jump) return offNotice();

  const win = side === 'local' && isWinPath(oldPath);
  const oldSegs = segsOf(oldPath);
  let down = [];
  if (!up) {
    const newSegs = segsOf(newPath);
    if (startsWith(newSegs, oldSegs, win) && newSegs.length > oldSegs.length) {
      down = newSegs.slice(oldSegs.length);
    } else if (startsWith(oldSegs, newSegs, win) && newSegs.length < oldSegs.length) {
      up = oldSegs.length - newSegs.length;
    } else if (newSegs.length === oldSegs.length && startsWith(newSegs, oldSegs, win)) {
      return {}; // same folder (refresh)
    } else {
      return offNotice();
    }
  }

  // ".." at the root (or drive) doesn't move this panel: nor the other one.
  if (up && oldSegs.length - up < (win ? 1 : 0)) return {};

  const otherSide = other(side);
  const otherPath = get(pathStore(otherSide));
  const otherWin = otherSide === 'local' && isWinPath(otherPath);
  const otherSegs = segsOf(otherPath);
  const go = (segs) => {
    const target = pathOf(segs, otherWin);
    return otherSide === 'local' ? navigateLocal(target) : refreshRemote(target);
  };

  if (up) {
    // Can't go above the root (or the drive) on the other side.
    const keep = otherSegs.length - up;
    if (keep < (otherWin ? 1 : 0)) return offNotice();
    return { follow: () => go(otherSegs.slice(0, keep)) };
  }

  const targetSegs = [...otherSegs, ...down];
  const missingAt = await firstMissing(otherSide, otherSegs, down, otherWin);
  if (missingAt < 0) return { follow: () => go(targetSegs) };

  let choice = 'create';
  if (!get(settings)?.syncCreateMissing) {
    choice = await new Promise(resolve =>
      syncPrompt.set({ side: otherSide, path: pathOf(targetSegs, otherWin), resolve }));
  }
  if (choice === 'cancel') return null;
  if (choice === 'off') {
    syncBrowsing.set(false);
    return {};
  }
  if (choice === 'createAlways') {
    await saveSettings({ ...get(settings), syncCreateMissing: true });
  }
  return {
    follow: async () => {
      const mkdir = otherSide === 'local' ? localMkDir : remoteMkDir;
      for (let i = missingAt; i < down.length; i++) {
        await mkdir(pathOf([...otherSegs, ...down.slice(0, i + 1)], otherWin));
      }
      await go(targetSegs);
    },
  };
}
