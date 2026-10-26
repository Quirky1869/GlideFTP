import { writable } from 'svelte/store';

// Global notification popup - replaces native alert() with a themed, translated modal.
// { type: 'success' | 'error', message: string, action?: { label, run } } | null
export const notification = writable(null);

// action (optional): extra button next to "Fermer" - closes the popup then calls run()
export function notify(message, type = 'success', action = null) {
  if (!message) return;
  notification.set({ message, type, action });
}

export function closeNotify() {
  notification.set(null);
}
