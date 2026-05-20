import { writable, derived } from 'svelte/store';
import en from './en.js';
import fr from './fr.js';

const catalogs = { en, fr };

export const locale = writable('en');

export const t = derived(locale, ($locale) => {
  const catalog = catalogs[$locale] || catalogs.en;
  return (key) => catalog[key] ?? key;
});
