import { writable, get } from 'svelte/store';
import {
  Connect, Disconnect, GetConnectionStatus,
  ConnectToSite, ConnectToSiteAdditional, ConnectWithPassword,
  GetConnections, SwitchConnection, CloseConnection,
  RemoteListDir, RemoteMkDir, RemoteDelete, RemoteRename,
  LocalListDir, LocalMkDir, LocalDelete, LocalRename,
  GetLocalHome, GetLocalParent,
} from '../../wailsjs/go/main/App.js';

// 'disconnected' | 'connecting' | 'connected'
export const connectionStatus = writable('disconnected');
export const connectionError = writable('');
export const activeConnectionConfig = writable(null);

// Multi-connection state.
// Each entry: { id, name, host, protocol, port, user, remotePath }
export const connections = writable([]);
export const activeConnectionId = writable(null);

export const localPath = writable('');
export const localEntries = writable([]);
export const localSelected = writable([]);

export const remotePath = writable('/');
export const remoteEntries = writable([]);
export const remoteSelected = writable([]);

export async function initLocalDir(startDir) {
  const dir = startDir || await GetLocalHome();
  localPath.set(dir);
  await refreshLocal(dir);
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
    // Keep the stored remotePath in sync for the active tab
    const id = get(activeConnectionId);
    if (id) {
      connections.update(cs => cs.map(c => c.id === id ? { ...c, remotePath: path } : c));
    }
  } catch (e) {
    console.error('Remote list error', e);
    try {
      const status = await GetConnectionStatus();
      if (status !== 'connected') {
        connectionStatus.set('disconnected');
        remoteEntries.set([]);
        remotePath.set('/');
      }
    } catch {}
    throw e;
  }
}

// ── Helpers ──────────────────────────────────────────────────────────────────

function entryFromInfo(info) {
  return {
    id:       info.id,
    name:     info.name,
    host:     info.host,
    protocol: info.protocol,
    port:     info.port,
    user:     info.user,
    remotePath: '/',
  };
}

function configFromInfo(info) {
  return { protocol: info.protocol, host: info.host, port: info.port, user: info.user };
}

function clearRemoteState() {
  remoteEntries.set([]);
  remotePath.set('/');
  remoteSelected.set([]);
  activeConnectionConfig.set(null);
}

// ── Direct connect (ConnectionBar) ───────────────────────────────────────────

export async function connect(cfg) {
  connectionStatus.set('connecting');
  connectionError.set('');
  try {
    const info = await Connect(cfg);
    const oldId = get(activeConnectionId);
    connectionStatus.set('connected');
    activeConnectionConfig.set(configFromInfo(info));
    connections.update(cs => {
      const without = cs.filter(c => c.id !== oldId);
      return [...without, entryFromInfo(info)];
    });
    activeConnectionId.set(info.id);
    await refreshRemote('/');
  } catch (e) {
    connectionStatus.set('disconnected');
    connectionError.set(e?.toString() || 'Connection failed');
    throw e;
  }
}

// ── Disconnect all ───────────────────────────────────────────────────────────

export async function disconnect() {
  await Disconnect();
  connectionStatus.set('disconnected');
  connections.set([]);
  activeConnectionId.set(null);
  clearRemoteState();
}

// ── Connect via saved site (replace active) ──────────────────────────────────

export async function connectBySite(id, config = null) {
  connectionStatus.set('connecting');
  connectionError.set('');
  const oldId = get(activeConnectionId);
  try {
    const info = await ConnectToSite(id);
    connectionStatus.set('connected');
    activeConnectionConfig.set(config || configFromInfo(info));
    connections.update(cs => {
      const without = cs.filter(c => c.id !== oldId);
      return [...without, entryFromInfo(info)];
    });
    activeConnectionId.set(info.id);
  } catch (e) {
    connectionStatus.set('disconnected');
    connectionError.set(e?.toString() || 'Connection failed');
    throw e;
  }
}

export async function connectBySiteWithPassword(id, password, config = null) {
  connectionStatus.set('connecting');
  connectionError.set('');
  const oldId = get(activeConnectionId);
  try {
    const info = await ConnectWithPassword(id, password);
    connectionStatus.set('connected');
    activeConnectionConfig.set(config || configFromInfo(info));
    connections.update(cs => {
      const without = cs.filter(c => c.id !== oldId);
      return [...without, entryFromInfo(info)];
    });
    activeConnectionId.set(info.id);
  } catch (e) {
    connectionStatus.set('disconnected');
    connectionError.set(e?.toString() || 'Connection failed');
    throw e;
  }
}

// ── Connect via saved site (keep existing connections) ───────────────────────

export async function addConnection(siteId, overridePassword = '') {
  connectionStatus.set('connecting');
  connectionError.set('');
  try {
    const info = await ConnectToSiteAdditional(siteId, overridePassword);
    connectionStatus.set('connected');
    activeConnectionConfig.set(configFromInfo(info));
    connections.update(cs => [...cs, entryFromInfo(info)]);
    activeConnectionId.set(info.id);
    await refreshRemote('/');
  } catch (e) {
    // Restore to connected if we still have open connections
    const conns = get(connections);
    connectionStatus.set(conns.length > 0 ? 'connected' : 'disconnected');
    connectionError.set(e?.toString() || 'Connection failed');
    throw e;
  }
}

// ── Switch to a different open connection (tab click) ────────────────────────

export async function switchTab(id) {
  const conns = get(connections);
  const conn = conns.find(c => c.id === id);
  if (!conn) return;

  // Persist current path in the connections store before switching
  const currentId = get(activeConnectionId);
  const currentPath = get(remotePath);
  connections.update(cs => cs.map(c => c.id === currentId ? { ...c, remotePath: currentPath } : c));

  await SwitchConnection(id);
  activeConnectionId.set(id);
  activeConnectionConfig.set(configFromInfo(conn));
  await refreshRemote(conn.remotePath || '/');
}

// ── Close a single connection tab ────────────────────────────────────────────

export async function closeTab(id) {
  const currentId = get(activeConnectionId);
  await CloseConnection(id);
  connections.update(cs => cs.filter(c => c.id !== id));

  const remaining = get(connections);
  if (remaining.length === 0) {
    connectionStatus.set('disconnected');
    activeConnectionId.set(null);
    clearRemoteState();
  } else if (currentId === id) {
    // Was active — switch to the last remaining tab
    const next = remaining[remaining.length - 1];
    activeConnectionId.set(next.id);
    activeConnectionConfig.set(configFromInfo(next));
    await refreshRemote(next.remotePath || '/');
  }
}

// ── Local FS helpers (unchanged) ─────────────────────────────────────────────

export async function localMkDir(path)               { await LocalMkDir(path); }
export async function localDelete(path)               { await LocalDelete(path); }
export async function localRename(oldPath, newPath)   { await LocalRename(oldPath, newPath); }
export async function remoteMkDir(path)               { await RemoteMkDir(path); }
export async function remoteDelete(path)              { await RemoteDelete(path); }
export async function remoteRename(oldPath, newPath)  { await RemoteRename(oldPath, newPath); }
