// Sounds are embedded as base64 data: URIs (frontend/src/utils/sounds.generated.js,
// produced by scripts/generate_notification_sounds.py) rather than imported as
// plain asset files. WebKitGTK's <audio> loader only recognizes a fixed set of
// URI schemes (http/https/file/data/blob/...); Wails serves the frontend through
// its own custom `wails://` scheme, which isn't in that list, so a plain asset
// URL fails instantly with a GStreamer-less "FormatError" - in both `wails dev`
// and the compiled binary - before any playback is even attempted. data: URIs
// sidestep the scheme check entirely and work identically in dev and build.
import { SOUND_DATA_URLS } from './sounds.generated.js';
import { GetCustomSound } from '../../wailsjs/go/main/App.js';

// Any sound (custom ones especially) is cut after this, with a short fade-out
export const MAX_SOUND_SECONDS = 5;
const FADE_MS = 400;

// User-supplied sound (Settings > Notifications > Load a sound...), id "custom".
// Stored by the backend in the config dir, fetched once as a data: URI.
export const CUSTOM_SOUND_ID = 'custom';
let customSoundUrl = null; // null = not fetched yet, '' = none installed

// Soft, short notification sounds - offered in Settings > Notifications.
// `label` is a translation key resolved via $t() in the UI.
export const NOTIFICATION_SOUNDS = [
  { id: 'chime', label: 'soundChime', url: SOUND_DATA_URLS.chime },
  { id: 'bell', label: 'soundBell', url: SOUND_DATA_URLS.bell },
  { id: 'pop', label: 'soundPop', url: SOUND_DATA_URLS.pop },
  { id: 'glass', label: 'soundGlass', url: SOUND_DATA_URLS.glass },
  { id: 'siren', label: 'soundSiren', url: SOUND_DATA_URLS.siren },
  { id: 'buzzer', label: 'soundBuzzer', url: SOUND_DATA_URLS.buzzer },
  { id: 'alert', label: 'soundAlert', url: SOUND_DATA_URLS.alert },
  { id: 'alarm', label: 'soundAlarm', url: SOUND_DATA_URLS.alarm },
  { id: 'ring', label: 'soundRing', url: SOUND_DATA_URLS.ring },
  { id: 'trill', label: 'soundTrill', url: SOUND_DATA_URLS.trill },
  { id: 'ping', label: 'soundPing', url: SOUND_DATA_URLS.ping },
  { id: 'beep', label: 'soundBeep', url: SOUND_DATA_URLS.beep },
  { id: 'signal', label: 'soundSignal', url: SOUND_DATA_URLS.signal },
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

// Called after a new custom sound is installed ('' after removal)
export function setCustomSoundUrl(url) {
  customSoundUrl = url || '';
  delete audioCache[CUSTOM_SOUND_ID];
}

async function resolveSound(id) {
  if (id === CUSTOM_SOUND_ID) {
    if (customSoundUrl === null) {
      try { customSoundUrl = (await GetCustomSound()) || ''; } catch { customSoundUrl = ''; }
    }
    if (customSoundUrl) return { id: CUSTOM_SOUND_ID, url: customSoundUrl };
  }
  return NOTIFICATION_SOUNDS.find(s => s.id === id) || NOTIFICATION_SOUNDS[0];
}

// Checks a data: URI decodes as audio. Resolves with its duration in seconds
// (Infinity/NaN for some streams), rejects if WebKit/GStreamer can't read it.
export function probeSound(url) {
  return new Promise((resolve, reject) => {
    const audio = new Audio();
    const timer = setTimeout(() => done(new Error('timeout')), 8000);
    function done(err) {
      clearTimeout(timer);
      audio.removeAttribute('src');
      err ? reject(err) : resolve(audio.duration);
    }
    audio.addEventListener('loadedmetadata', () => done(null), { once: true });
    audio.addEventListener('error', () => done(new Error('unreadable')), { once: true });
    audio.preload = 'metadata';
    audio.src = url;
  });
}

let stopTimers = [];
let playing = null;

function stopCurrent() {
  stopTimers.forEach(clearTimeout);
  stopTimers = [];
  if (playing) {
    playing.pause();
    playing.volume = 1;
    playing = null;
  }
}

// Plays at the Audio element's default volume (1.0) - actual loudness is
// left entirely to the OS/app volume mixer, there is no in-app volume knob.
// Playback stops after MAX_SOUND_SECONDS (fading out over the last FADE_MS).
export async function playNotificationSound(id) {
  const sound = await resolveSound(id);
  try {
    stopCurrent();
    const audio = getAudio(sound);
    audio.volume = 1;
    audio.currentTime = 0;
    playing = audio;
    audio.play().catch(err => console.error('[GlideFTP] notification sound failed to play:', err));
    const fadeStart = MAX_SOUND_SECONDS * 1000 - FADE_MS;
    const steps = 8;
    for (let i = 1; i <= steps; i++) {
      stopTimers.push(setTimeout(() => { audio.volume = Math.max(0, 1 - i / steps); }, fadeStart + (FADE_MS * i) / steps));
    }
    stopTimers.push(setTimeout(stopCurrent, MAX_SOUND_SECONDS * 1000));
  } catch (e) {
    console.error('[GlideFTP] notification sound error:', e);
  }
}
