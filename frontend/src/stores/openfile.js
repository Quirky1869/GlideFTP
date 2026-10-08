// "Open" files with the system's default application (backend: app_openfile.go).
// Remote files are downloaded to a temp folder and watched; each time one is
// saved, the backend emits openfile:modified and we ask whether to send it back.
import { writable, get } from 'svelte/store';
import { EventsOn } from '../../wailsjs/runtime/runtime.js';
import {
  OpenLocalFile, OpenRemoteFile, CancelOpenRemoteFile, UploadOpenedFile,
  IgnoreOpenedFileChange, SetOpenedFileAutoUpload, ChooseAppAndOpen, OpenContainingFolder,
} from '../../wailsjs/go/main/App.js';
import { t } from '../i18n/index.js';
import { notify } from './notify.js';
import { activeConnectionId, remotePath, refreshRemote, localPath, localEntries, refreshLocal, localCopy } from './connection.js';

export const openingFile = writable(null); // { name } while a remote file downloads
export const openPrompts = writable([]);   // queue of { mode: 'modified'|'conflict', file, busy }
export const noAppPrompt = writable(null); // { path, name, remote } - no application for this type
export const openToast = writable(null);   // { text } - discreet confirmation

const tr = (key) => get(t)(key);
// Wails rejects with the Go error's string; localfs.ErrNoDefaultApp is translated.
const errText = (e) => {
  const msg = typeof e === 'string' ? e : e?.message || String(e);
  return msg === 'no application associated with this file type' ? tr('noDefaultApp') : msg;
};

let toastTimer = null;
function showToast(text) {
  openToast.set({ text });
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => openToast.set(null), 3000);
}

// One prompt per file in the queue: a newer event for the same file replaces
// the queued one (but never the prompt currently shown and busy uploading).
function enqueue(mode, file) {
  openPrompts.update(list => {
    const i = list.findIndex(p => p.file.id === file.id);
    const prompt = { mode, file, busy: false };
    if (i === -1) return [...list, prompt];
    if (list[i].busy) return list;
    const next = [...list];
    next[i] = prompt;
    return next;
  });
}

function shiftPrompt() {
  openPrompts.update(list => list.slice(1));
}

function remoteParent(p) {
  const i = p.lastIndexOf('/');
  return i <= 0 ? '/' : p.slice(0, i);
}

// Refresh the remote panel if it currently shows the uploaded file's folder.
function refreshIfVisible(file) {
  if (get(activeConnectionId) === file.connId && remoteParent(file.remotePath) === get(remotePath)) {
    refreshRemote(get(remotePath));
  }
}

function uploadErrorText(file, msg) {
  if (msg === 'connection_closed') return tr('openFileConnectionClosed').replace('{host}', file.host);
  return tr('openFileUploadError').replace('{name}', file.name) + '\n' + msg;
}

export function initOpenFile() {
  EventsOn('openfile:modified', (f) => enqueue(f.conflict ? 'conflict' : 'modified', f));
  EventsOn('openfile:uploaded', (f) => {
    refreshIfVisible(f);
    showToast(tr('openFileUploaded').replace('{name}', f.name));
  });
  EventsOn('openfile:error', (f) => notify(uploadErrorText(f, f.error), 'error'));
  EventsOn('openfile:orphaned', ({ host, names }) => {
    notify(tr('openFileOrphaned').replace('{host}', host).replace('{names}', (names || []).join(', ')), 'error');
  });
}

// Opens a file entry of the local or remote panel.
export async function openEntry(side, entry) {
  if (!entry || entry.isDir) return;
  if (side === 'local') {
    try {
      const r = await OpenLocalFile(entry.path);
      if (r?.noDefaultApp) noAppPrompt.set({ path: r.path, name: entry.name });
    } catch (e) {
      notify(tr('openFileError').replace('{name}', entry.name) + '\n' + errText(e), 'error');
    }
    return;
  }
  if (get(openingFile)) return; // one download at a time
  openingFile.set({ name: entry.name });
  try {
    const r = await OpenRemoteFile(entry.path);
    if (r?.noDefaultApp) noAppPrompt.set({ path: r.path, name: entry.name, remote: true });
  } catch (e) {
    const msg = errText(e);
    if (msg !== 'cancelled') notify(tr('openFileError').replace('{name}', entry.name) + '\n' + msg, 'error');
  } finally {
    openingFile.set(null);
  }
}

export function cancelOpening() {
  CancelOpenRemoteFile();
}

// Answer to the prompt at the head of the queue:
// 'yes' | 'no' | 'always' (modified mode), 'overwrite' | 'cancel' (conflict mode).
export async function answerPrompt(choice) {
  const head = get(openPrompts)[0];
  if (!head || head.busy) return;
  const file = head.file;

  if (choice === 'no' || choice === 'cancel') {
    await IgnoreOpenedFileChange(file.id);
    shiftPrompt();
    return;
  }
  if (choice === 'always') await SetOpenedFileAutoUpload(file.id, true);

  openPrompts.update(list => [{ ...list[0], busy: true }, ...list.slice(1)]);
  try {
    await UploadOpenedFile(file.id, choice === 'overwrite');
    shiftPrompt();
    refreshIfVisible(file);
    showToast(tr('openFileUploaded').replace('{name}', file.name));
  } catch (e) {
    const msg = errText(e);
    if (msg === 'remote_changed') {
      // Same file, now asking whether to overwrite the server's version.
      openPrompts.update(list => [{ mode: 'conflict', file, busy: false }, ...list.slice(1)]);
      return;
    }
    shiftPrompt();
    notify(uploadErrorText(file, msg), 'error');
  }
}

export async function openContainingFolder() {
  const p = get(noAppPrompt);
  noAppPrompt.set(null);
  if (!p) return;
  try { await OpenContainingFolder(p.path); } catch (e) { notify(errText(e), 'error'); }
}

// "Download": copies the already downloaded temp file into the local panel's
// folder (instant, no second download, independent of the connection). Never
// overwrites: "name (1).ext", "name (2).ext"... if the name is taken.
export async function downloadFromNoApp() {
  const p = get(noAppPrompt);
  noAppPrompt.set(null);
  if (!p) return;
  const dir = get(localPath);
  const sep = dir.includes('\\') && !dir.includes('/') ? '\\' : '/';
  const taken = new Set(get(localEntries).map(e => e.name));
  const dot = p.name.lastIndexOf('.');
  const base = dot > 0 ? p.name.slice(0, dot) : p.name;
  const ext = dot > 0 ? p.name.slice(dot) : '';
  let name = p.name;
  for (let i = 1; taken.has(name); i++) name = `${base} (${i})${ext}`;
  try {
    await localCopy(p.path, dir.replace(/[\\/]+$/, '') + sep + name);
    await refreshLocal(dir);
    showToast(tr('openFileDownloaded').replace('{name}', name));
  } catch (e) {
    notify(errText(e), 'error');
  }
}

export async function chooseAppAndOpen() {
  const p = get(noAppPrompt);
  noAppPrompt.set(null);
  if (!p) return;
  try { await ChooseAppAndOpen(p.path); } catch (e) { notify(errText(e), 'error'); }
}
