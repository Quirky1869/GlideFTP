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
    applyAccentColor(s.accentColor || '#5B8AF5');
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
  applyAccentColor(s.accentColor || '#5B8AF5');
}

export function applyTheme(t) {
  document.documentElement.setAttribute('data-theme', t);
}

export function applyAccentColor(hex) {
  if (!hex || !/^#[0-9a-fA-F]{6}$/.test(hex)) hex = '#5B8AF5';
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  const f = 0.84;
  const hr = Math.round(r * f);
  const hg = Math.round(g * f);
  const hb = Math.round(b * f);
  const hoverHex = '#' + [hr, hg, hb].map(v => v.toString(16).padStart(2, '0')).join('');
  const subtle = `rgba(${r}, ${g}, ${b}, 0.14)`;
  const glow   = `rgba(${r}, ${g}, ${b}, 0.45)`;
  const root = document.documentElement;
  root.style.setProperty('--accent', hex);
  root.style.setProperty('--accent-hover', hoverHex);
  root.style.setProperty('--accent-subtle', subtle);
  root.style.setProperty('--accent-glow', glow);
}
