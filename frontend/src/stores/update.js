import { writable, get } from 'svelte/store';
import { CheckForUpdate, OpenUpdateLink } from '../../wailsjs/go/main/App.js';
import { settings, saveSettings } from './settings.js';

// Newer GitHub release shown in UpdateBanner (App.svelte), null = no banner.
// { current, latest, available, releaseUrl, kind, downloadUrl, downloadName, command }
export const updateInfo = writable(null);

// Startup check (when settings.checkUpdates is on): silent on failure, and a
// version the user chose to ignore shows no banner.
export async function checkForUpdateAtStartup() {
  try {
    const info = await CheckForUpdate();
    if (info?.available && info.latest !== get(settings)?.skippedUpdateVersion) {
      updateInfo.set(info);
    }
  } catch {
    // no network, proxy, GitHub rate limit: try again next launch
  }
}

// "Vérifier maintenant" in Settings: errors are thrown to the caller, and the
// banner shows even for an ignored version (the user asked).
export async function checkForUpdateNow() {
  const info = await CheckForUpdate();
  updateInfo.set(info?.available ? info : null);
  return info;
}

// "Tester le bandeau" in Settings: the real banner with the links of the
// latest published release, even when it is not newer. preview: the text says
// it is a test, and "Ignorer cette version" saves nothing.
export async function previewUpdateBanner() {
  const info = await CheckForUpdate();
  updateInfo.set({ ...info, available: true, preview: true });
}

export async function skipUpdateVersion(version) {
  const preview = get(updateInfo)?.preview;
  updateInfo.set(null);
  if (preview) return;
  await saveSettings({ ...get(settings), skippedUpdateVersion: version });
}

export function openUpdateLink(url) {
  return OpenUpdateLink(url);
}
