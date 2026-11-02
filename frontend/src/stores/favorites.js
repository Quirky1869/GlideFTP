import { writable, get } from 'svelte/store';
import { GetFavorites, SetFavorites } from '../../wailsjs/go/main/App.js';

// key → [{ path, isDir }]. "local" is shared by every server; a remote list
// belongs to one server account (see favKey).
export const favorites = writable({});

export async function loadFavorites() {
  try {
    favorites.set((await GetFavorites()) || {});
  } catch {
    favorites.set({});
  }
}

// null while no server is connected (no remote list to show).
export function favKey(side, cfg) {
  if (side === 'local') return 'local';
  if (!cfg?.host) return null;
  return `${cfg.protocol}://${cfg.user || ''}@${cfg.host.toLowerCase()}:${cfg.port}`;
}

async function setList(key, list) {
  favorites.update(f => ({ ...f, [key]: list }));
  await SetFavorites(key, list);
}

export function isFavorite(favs, key, path) {
  return !!key && (favs[key] || []).some(f => f.path === path);
}

export async function addFavorites(key, entries) {
  if (!key) return;
  const list = [...(get(favorites)[key] || [])];
  for (const e of entries) {
    if (!list.some(f => f.path === e.path)) list.push({ path: e.path, isDir: !!e.isDir });
  }
  await setList(key, list);
}

export async function removeFavorites(key, paths) {
  if (!key) return;
  const list = get(favorites)[key] || [];
  const next = list.filter(f => !paths.includes(f.path));
  if (next.length !== list.length) await setList(key, next);
}

// The path itself or anything inside it ("/a" covers "/a/b" and "C:\a\b").
const within = (p, root) => p === root || p.startsWith(root + '/') || p.startsWith(root + '\\');

// Called after a rename or move done in GlideFTP: favourites follow the new path.
export async function favoritesRenamed(key, oldPath, newPath) {
  if (!key) return;
  const list = get(favorites)[key] || [];
  if (!list.some(f => within(f.path, oldPath))) return;
  await setList(key, list.map(f => within(f.path, oldPath) ? { ...f, path: newPath + f.path.slice(oldPath.length) } : f));
}

// Called after a delete done in GlideFTP: the favourite and those inside it go.
export async function favoritesDeleted(key, path) {
  if (!key) return;
  const list = get(favorites)[key] || [];
  const next = list.filter(f => !within(f.path, path));
  if (next.length !== list.length) await setList(key, next);
}
