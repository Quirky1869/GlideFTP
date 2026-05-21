<script>
  import { onMount, afterUpdate } from 'svelte';
  import { t } from '../i18n/index.js';

  export let value = '#5B8AF5';
  export let onClose = () => {};
  export let onApply = (_hex) => {};

  const DEFAULT_COLOR = '#5B8AF5';

  let canvas;
  let hue = 210;
  let saturation = 63;
  let brightness = 96;
  let r = 91, g = 138, b = 245;
  let hex = '#5B8AF5';
  let hexInput = '#5B8AF5';
  let initialized = false;
  let colorHistory = [];

  onMount(() => {
    initFromHex(value || DEFAULT_COLOR);
    initialized = true;
    drawCanvas();
    try {
      const stored = localStorage.getItem('glideftp_color_history');
      if (stored) colorHistory = JSON.parse(stored);
    } catch {}
  });

  $: if (initialized && canvas) drawCanvas();

  function initFromHex(h) {
    const rgb = hexToRgb(h);
    if (!rgb) return;
    const hsv = rgbToHsv(rgb.r, rgb.g, rgb.b);
    hue = hsv.h;
    saturation = hsv.s * 100;
    brightness = hsv.v * 100;
    r = rgb.r; g = rgb.g; b = rgb.b;
    hex = h.toLowerCase();
    hexInput = hex;
  }

  function drawCanvas() {
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const w = canvas.width;
    const h = canvas.height;

    const hueRgb = hsvToRgb(hue, 100, 100);
    ctx.fillStyle = `rgb(${hueRgb.r},${hueRgb.g},${hueRgb.b})`;
    ctx.fillRect(0, 0, w, h);

    const wGrad = ctx.createLinearGradient(0, 0, w, 0);
    wGrad.addColorStop(0, 'rgba(255,255,255,1)');
    wGrad.addColorStop(1, 'rgba(255,255,255,0)');
    ctx.fillStyle = wGrad;
    ctx.fillRect(0, 0, w, h);

    const bGrad = ctx.createLinearGradient(0, h, 0, 0);
    bGrad.addColorStop(0, 'rgba(0,0,0,1)');
    bGrad.addColorStop(1, 'rgba(0,0,0,0)');
    ctx.fillStyle = bGrad;
    ctx.fillRect(0, 0, w, h);

    // cursor
    const cx = (saturation / 100) * w;
    const cy = (1 - brightness / 100) * h;
    ctx.beginPath();
    ctx.arc(cx, cy, 8, 0, Math.PI * 2);
    ctx.strokeStyle = brightness > 55 ? 'rgba(0,0,0,0.7)' : 'rgba(255,255,255,0.9)';
    ctx.lineWidth = 2;
    ctx.stroke();
    ctx.beginPath();
    ctx.arc(cx, cy, 6, 0, Math.PI * 2);
    ctx.strokeStyle = 'white';
    ctx.lineWidth = 1;
    ctx.stroke();
  }

  function pickFromCanvas(e) {
    const rect = canvas.getBoundingClientRect();
    const x = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
    const y = Math.max(0, Math.min(1, (e.clientY - rect.top) / rect.height));
    saturation = x * 100;
    brightness = (1 - y) * 100;
    updateFromHsv();
    drawCanvas();
  }

  function handleCanvasMouseDown(e) {
    pickFromCanvas(e);
    const onMove = (ev) => pickFromCanvas(ev);
    const onUp = () => {
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
    e.preventDefault();
  }

  function updateFromHsv() {
    const rgb = hsvToRgb(hue, saturation, brightness);
    r = rgb.r; g = rgb.g; b = rgb.b;
    hex = rgbToHex(r, g, b);
    hexInput = hex;
  }

  function onHueInput() {
    updateFromHsv();
    drawCanvas();
  }

  function onHexChange() {
    hexInput = hexInput.startsWith('#') ? hexInput : '#' + hexInput;
    if (/^#[0-9a-fA-F]{6}$/.test(hexInput)) {
      initFromHex(hexInput);
      drawCanvas();
    }
  }

  function onRgbChange() {
    r = Math.max(0, Math.min(255, parseInt(r) || 0));
    g = Math.max(0, Math.min(255, parseInt(g) || 0));
    b = Math.max(0, Math.min(255, parseInt(b) || 0));
    hex = rgbToHex(r, g, b);
    hexInput = hex;
    const hsv = rgbToHsv(r, g, b);
    hue = hsv.h; saturation = hsv.s * 100; brightness = hsv.v * 100;
    drawCanvas();
  }

  function resetToDefault() {
    initFromHex(DEFAULT_COLOR);
    drawCanvas();
  }

  function addToHistory(h) {
    colorHistory = [h, ...colorHistory.filter(c => c !== h)].slice(0, 8);
    try { localStorage.setItem('glideftp_color_history', JSON.stringify(colorHistory)); } catch {}
  }

  function apply() {
    addToHistory(hex);
    onApply(hex);
  }

  // ── Color math ─────────────────────────────────────────────────────────────

  function hsvToRgb(h, s, v) {
    s /= 100; v /= 100;
    const c = v * s;
    const x = c * (1 - Math.abs((h / 60) % 2 - 1));
    const m = v - c;
    let r1, g1, b1;
    if (h < 60)       { r1=c; g1=x; b1=0; }
    else if (h < 120) { r1=x; g1=c; b1=0; }
    else if (h < 180) { r1=0; g1=c; b1=x; }
    else if (h < 240) { r1=0; g1=x; b1=c; }
    else if (h < 300) { r1=x; g1=0; b1=c; }
    else              { r1=c; g1=0; b1=x; }
    return { r: Math.round((r1+m)*255), g: Math.round((g1+m)*255), b: Math.round((b1+m)*255) };
  }

  function rgbToHsv(r, g, b) {
    r /= 255; g /= 255; b /= 255;
    const max = Math.max(r,g,b), min = Math.min(r,g,b), d = max - min;
    let h = 0, s = max === 0 ? 0 : d / max, v = max;
    if (d !== 0) {
      if (max === r) h = ((g - b) / d) % 6;
      else if (max === g) h = (b - r) / d + 2;
      else h = (r - g) / d + 4;
      h = h * 60;
      if (h < 0) h += 360;
    }
    return { h, s, v };
  }

  function hexToRgb(hex) {
    const m = hex.match(/^#?([0-9a-f]{6})$/i);
    if (!m) return null;
    return { r: parseInt(m[1].slice(0,2),16), g: parseInt(m[1].slice(2,4),16), b: parseInt(m[1].slice(4,6),16) };
  }

  function rgbToHex(r, g, b) {
    return '#' + [r,g,b].map(v => Math.round(v).toString(16).padStart(2,'0')).join('');
  }
</script>

<div class="cp-backdrop" on:click|self={onClose}></div>

<div class="cp-panel">
  <div class="cp-header">
    <span class="cp-title">{$t('colorPickerTitle')}</span>
    <button class="close-btn" on:click={onClose}>✕</button>
  </div>

  <div class="cp-body">

    <!-- 2D gradient canvas -->
    <canvas
      bind:this={canvas}
      class="color-canvas"
      width="280"
      height="180"
      on:mousedown={handleCanvasMouseDown}
    ></canvas>

    <!-- Hue slider -->
    <div class="hue-row">
      <div class="preview-swatch" style="background: {hex}"></div>
      <input
        type="range"
        class="hue-slider"
        min="0" max="360"
        bind:value={hue}
        on:input={onHueInput}
      />
    </div>

    <!-- HEX input -->
    <div class="input-row">
      <label class="input-label">HEX</label>
      <input
        class="hex-input"
        type="text"
        bind:value={hexInput}
        on:change={onHexChange}
        maxlength="7"
        spellcheck="false"
      />
    </div>

    <!-- RGB inputs -->
    <div class="rgb-row">
      <div class="rgb-field">
        <label class="input-label">R</label>
        <input type="number" min="0" max="255" bind:value={r} on:change={onRgbChange} />
      </div>
      <div class="rgb-field">
        <label class="input-label">G</label>
        <input type="number" min="0" max="255" bind:value={g} on:change={onRgbChange} />
      </div>
      <div class="rgb-field">
        <label class="input-label">B</label>
        <input type="number" min="0" max="255" bind:value={b} on:change={onRgbChange} />
      </div>
    </div>

    <!-- Color history -->
    <div class="history-section">
      <label class="input-label history-label">{$t('colorHistory')}</label>
      <div class="history-swatches">
        {#each colorHistory as c}
          <button
            class="history-swatch"
            style="background: {c}"
            title={c}
            on:click={() => { initFromHex(c); drawCanvas(); }}
          ></button>
        {/each}
        {#each { length: 8 - colorHistory.length } as _}
          <div class="history-swatch-empty"></div>
        {/each}
      </div>
    </div>

  </div>

  <div class="cp-footer">
    <button class="btn-reset" on:click={resetToDefault}>{$t('resetColor')}</button>
    <button class="btn-cancel" on:click={onClose}>{$t('close')}</button>
    <button class="btn-apply" on:click={apply}>{$t('saveSettings')}</button>
  </div>
</div>

<style>
.cp-backdrop {
  position: fixed;
  inset: 0;
  z-index: 450;
}

.cp-panel {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  width: 340px;
  background: var(--bg-secondary);
  border-left: 1px solid var(--border);
  box-shadow: -8px 0 32px rgba(0,0,0,0.5);
  z-index: 500;
  display: flex;
  flex-direction: column;
  animation: slideIn 0.2s ease;
}

@keyframes slideIn {
  from { transform: translateX(100%); opacity: 0; }
  to   { transform: translateX(0);    opacity: 1; }
}

.cp-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.cp-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.close-btn {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 16px;
}
.close-btn:hover { color: var(--text-primary); }

.cp-body {
  flex: 1;
  overflow-y: auto;
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.color-canvas {
  width: 100%;
  height: 180px;
  border-radius: 6px;
  cursor: crosshair;
  display: block;
  border: 1px solid var(--border);
}

.hue-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.preview-swatch {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  border: 2px solid var(--border);
  flex-shrink: 0;
}

.hue-slider {
  flex: 1;
  -webkit-appearance: none;
  height: 18px;
  border-radius: 9px;
  background: linear-gradient(to right,
    hsl(0,100%,50%), hsl(30,100%,50%), hsl(60,100%,50%), hsl(90,100%,50%),
    hsl(120,100%,50%), hsl(150,100%,50%), hsl(180,100%,50%), hsl(210,100%,50%),
    hsl(240,100%,50%), hsl(270,100%,50%), hsl(300,100%,50%), hsl(330,100%,50%), hsl(360,100%,50%)
  );
  cursor: pointer;
  outline: none;
  border: none;
}

.hue-slider::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: white;
  border: 2px solid rgba(0,0,0,0.4);
  cursor: pointer;
  box-shadow: 0 1px 4px rgba(0,0,0,0.3);
}

.input-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.input-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  width: 28px;
  flex-shrink: 0;
}

.hex-input {
  flex: 1;
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  padding: 5px 8px;
  font-size: 13px;
  font-family: monospace;
  outline: none;
}
.hex-input:focus { border-color: var(--accent); }

.rgb-row {
  display: flex;
  gap: 8px;
}

.rgb-field {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.rgb-field input {
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  padding: 5px 6px;
  font-size: 13px;
  outline: none;
  width: 100%;
  text-align: center;
  -moz-appearance: textfield;
}
.rgb-field input::-webkit-inner-spin-button,
.rgb-field input::-webkit-outer-spin-button { -webkit-appearance: none; margin: 0; }
.rgb-field input:focus { border-color: var(--accent); }

.history-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.history-label {
  display: block;
  width: auto;
}

.history-swatches {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.history-swatch {
  width: 28px;
  height: 28px;
  border-radius: 5px;
  border: 2px solid var(--border);
  cursor: pointer;
  padding: 0;
  transition: border-color 0.1s, transform 0.1s;
  flex-shrink: 0;
}
.history-swatch:hover {
  border-color: var(--accent);
  transform: scale(1.15);
}

.history-swatch-empty {
  width: 28px;
  height: 28px;
  border-radius: 5px;
  border: 2px dashed var(--border);
  flex-shrink: 0;
  opacity: 0.4;
}

.cp-footer {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 14px 18px;
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}

.btn-reset {
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-secondary);
  padding: 7px 12px;
  font-size: 12px;
  cursor: pointer;
  flex: 1;
}
.btn-reset:hover { background: var(--bg-button-hover); }

.btn-cancel {
  background: var(--bg-button);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-secondary);
  padding: 7px 14px;
  font-size: 13px;
  cursor: pointer;
}
.btn-cancel:hover { background: var(--bg-button-hover); }

.btn-apply {
  background: var(--accent);
  border: none;
  border-radius: 5px;
  color: white;
  padding: 7px 16px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
}
.btn-apply:hover { background: var(--accent-hover); }
</style>
