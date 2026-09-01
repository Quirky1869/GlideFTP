// Sounds are embedded as base64 data: URIs (frontend/src/utils/sounds.generated.js,
// produced by scripts/generate_notification_sounds.py) rather than imported as
// plain asset files. WebKitGTK's <audio> loader only recognizes a fixed set of
// URI schemes (http/https/file/data/blob/...); Wails serves the frontend through
// its own custom `wails://` scheme, which isn't in that list, so a plain asset
// URL fails instantly with a GStreamer-less "FormatError" - in both `wails dev`
// and the compiled binary - before any playback is even attempted. data: URIs
// sidestep the scheme check entirely and work identically in dev and build.
import { SOUND_DATA_URLS } from './sounds.generated.js';

// Soft, short notification sounds - offered in Settings > Notifications.
// `label` is a translation key resolved via $t() in the UI.
export const NOTIFICATION_SOUNDS = [
  { id: 'chime', label: 'soundChime', url: SOUND_DATA_URLS.chime },
  { id: 'bell', label: 'soundBell', url: SOUND_DATA_URLS.bell },
  { id: 'pop', label: 'soundPop', url: SOUND_DATA_URLS.pop },
  { id: 'glass', label: 'soundGlass', url: SOUND_DATA_URLS.glass },
];

// One cached Audio instance per sound, kept alive for the app's lifetime.
// A bare `new Audio(url).play()` with no reference kept can get garbage
// collected by WebKit's JS engine before playback actually starts (silent,
// no error) - caching avoids that and lets rapid re-triggers replay cleanly.
const audioCache = {};

function getAudio(sound) {
  let audio = audioCache[sound.id];
  if (!audio) {
    audio = new Audio(sound.url);
    audioCache[sound.id] = audio;
  }
  return audio;
}

// Plays at the Audio element's default volume (1.0) - actual loudness is
// left entirely to the OS/app volume mixer, there is no in-app volume knob.
export function playNotificationSound(id) {
  const sound = NOTIFICATION_SOUNDS.find(s => s.id === id) || NOTIFICATION_SOUNDS[0];
  try {
    const audio = getAudio(sound);
    audio.currentTime = 0;
    audio.play().catch(err => console.error('[GlideFTP] notification sound failed to play:', err));
  } catch (e) {
    console.error('[GlideFTP] notification sound error:', e);
  }
}
