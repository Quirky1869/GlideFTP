import { writable } from 'svelte/store';
import {
  Connect, Disconnect, GetConnectionStatus,
  RemoteListDir, RemoteMkDir, RemoteDelete, RemoteRename,
  LocalListDir, LocalMkDir, LocalDelete, LocalRename,
  GetLocalHome, GetLocalParent, GetLocalRoots,
} from '../../wailsjs/go/main/App.js';

// 'disconnected' | 'connecting' | 'connected'
export const connectionStatus = writable('disconnected');
export const connectionError = writable('');

export const localPath = writable('');
export const localEntries = writable([]);
export const localSelected = writable([]);

export const remotePath = writable('/');
export const remoteEntries = writable([]);
export const remoteSelected = writable([]);

export async function initLocalDir() {
  const home = await GetLocalHome();
  localPath.set(home);
  await refreshLocal(home);
}

export async function refreshLocal(path) {
  try {
    const entries = await LocalListDir(path);
    localPath.set(path);
    localEntries.set(entries || []);
    localSelected.set([]);
  } catch (e) {
    console.error('Local list error', e);
  }
}

export async function navigateLocalUp(currentPath) {
  const parent = await GetLocalParent(currentPath);
  if (parent && parent !== currentPath) {
    await refreshLocal(parent);
  }
}

export async function refreshRemote(path) {
  try {
    const entries = await RemoteListDir(path);
    remotePath.set(path);
    remoteEntries.set(entries || []);
    remoteSelected.set([]);
  } catch (e) {
    console.error('Remote list error', e);
  }
}

export async function connect(cfg) {
  connectionStatus.set('connecting');
  connectionError.set('');
  try {
    await Connect(cfg);
    connectionStatus.set('connected');
    await refreshRemote('/');
  } catch (e) {
    connectionStatus.set('disconnected');
    connectionError.set(e?.toString() || 'Connection failed');
    throw e;
  }
}

export async function disconnect() {
  await Disconnect();
  connectionStatus.set('disconnected');
  remoteEntries.set([]);
  remotePath.set('/');
  remoteSelected.set([]);
}

export async function localMkDir(path) {
  await LocalMkDir(path);
}
export async function localDelete(path) {
  await LocalDelete(path);
}
export async function localRename(oldPath, newPath) {
  await LocalRename(oldPath, newPath);
}
export async function remoteMkDir(path) {
  await RemoteMkDir(path);
}
export async function remoteDelete(path) {
  await RemoteDelete(path);
}
export async function remoteRename(oldPath, newPath) {
  await RemoteRename(oldPath, newPath);
}
