import { writable } from 'svelte/store';
import { GetSettings, SaveSettings } from '../../wailsjs/go/main/App.js';
import { locale } from '../i18n/index.js';

export const settings = writable(null);
export const theme = writable('dark');

export async function loadSettings() {
  try {
    const s = await GetSettings();
    settings.set(s);
    theme.set(s.theme || 'dark');
    locale.set(s.language || 'en');
    applyTheme(s.theme || 'dark');
    return s;
  } catch (e) {
    console.error('Failed to load settings', e);
    return null;
  }
}

export async function saveSettings(s) {
  await SaveSettings(s);
  settings.set(s);
  theme.set(s.theme);
  locale.set(s.language);
  applyTheme(s.theme);
}

export function applyTheme(t) {
  document.documentElement.setAttribute('data-theme', t);
}
