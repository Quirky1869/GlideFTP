import { writable } from 'svelte/store';

// Global notification popup - replaces native alert() with a themed, translated modal.
// { type: 'success' | 'error', message: string } | null
export const notification = writable(null);

export function notify(message, type = 'success') {
  if (!message) return;
  notification.set({ message, type });
}

export function closeNotify() {
  notification.set(null);
}
