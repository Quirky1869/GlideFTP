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
    applyAccentFromSettings(s);
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
  applyAccentFromSettings(s);
}

export function applyTheme(t) {
  document.documentElement.setAttribute('data-theme', t);
}

export const DEFAULT_ACCENT = '#5B8AF5';
export const DEFAULT_ACCENT_2 = '#C15BF5';

const isHex = (h) => !!h && /^#[0-9a-fA-F]{6}$/.test(h);
const hexRgb = (hex) => [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16));
const darken = (hex, f = 0.84) =>
  '#' + hexRgb(hex).map(v => Math.round(v * f).toString(16).padStart(2, '0')).join('');
const rgba = (hex, a) => `rgba(${hexRgb(hex).join(', ')}, ${a})`;

export function applyAccentFromSettings(s) {
  applyAccentColor(s?.accentColor, !!s?.accentGradient, s?.accentColor2);
}

// Sets the accent CSS vars. With gradient off, every "-2" / "-bg" var equals
// the plain color so components look exactly as before. Components use:
//   --accent                    solid color 1 (icons, thin focus borders, fallback)
//   --accent-bg / -hover-bg     background: plain color or linear-gradient(c1, c2)
//   --accent-subtle-bg          translucent background (selected rows...)
//   --accent-gradient           always an image (gradient borders / gradient text)
//   --accent-2, --accent-glow-2, --accent-subtle-2   color 2 (two-tone shadows)
// html[data-accent-gradient] is set while the gradient is on, for the
// gradient-text / gradient-border overrides that only make sense then.
export function applyAccentColor(hex, gradient = false, hex2 = null) {
  if (!isHex(hex)) hex = DEFAULT_ACCENT;
  if (!isHex(hex2)) hex2 = DEFAULT_ACCENT_2;
  const c2 = gradient ? hex2 : hex;
  const grad = (a, b) => `linear-gradient(135deg, ${a}, ${b})`;
  const root = document.documentElement;
  const set = (k, v) => root.style.setProperty(k, v);

  set('--accent', hex);
  set('--accent-hover', darken(hex));
  set('--accent-subtle', rgba(hex, 0.14));
  set('--accent-glow', rgba(hex, 0.45));
  set('--accent-2', c2);
  set('--accent-subtle-2', rgba(c2, 0.14));
  set('--accent-glow-2', rgba(c2, 0.45));
  set('--accent-gradient', grad(hex, c2));
  set('--accent-bg', gradient ? grad(hex, c2) : hex);
  set('--accent-hover-bg', gradient ? grad(darken(hex), darken(c2)) : darken(hex));
  set('--accent-subtle-bg', gradient ? grad(rgba(hex, 0.14), rgba(c2, 0.14)) : rgba(hex, 0.14));
  if (gradient) root.setAttribute('data-accent-gradient', '');
  else root.removeAttribute('data-accent-gradient');
}
