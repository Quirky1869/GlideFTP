<script>
  import { onMount } from 'svelte';
  import { t } from '../i18n/index.js';
  import { gradientAngle, DEFAULT_GRADIENT_ANGLE } from '../stores/settings.js';

  export let value = '#5B8AF5';
  export let gradient = false;      // accent gradient on/off
  export let value2 = '#C15BF5';    // gradient end color
  export let angle = DEFAULT_GRADIENT_ANGLE; // gradient angle in degrees
  export let onClose = () => {};
  export let onApply = (_accent) => {}; // ({ color, gradient, color2, angle })

  const DEFAULT_COLOR = '#5B8AF5';
  const DEFAULT_COLOR_2 = '#C15BF5';
  const HISTORY_KEY = 'glideftp_color_history';
  const GRADIENT_HISTORY_KEY = 'glideftp_gradient_history'; // [{ c1, c2, a }]

  // Two tabs: "Uni" (single color = color 1, hue wheel) and "Dégradé"
  // (color 1 → color 2, angle). The tab shown when saving is the mode applied.
  let tab = gradient ? 'gradient' : 'solid';
  let solidMode = 'hex'; // 'hex' | 'rgb' inputs under the wheel
  let sel = 0;           // gradient point being edited (0 = start, 1 = end)

  // cols[i] = { h, s, v (0-100), r, g, b, hex, hexInput }
  let cols = [fromHex(value) || fromHex(DEFAULT_COLOR), fromHex(value2) || fromHex(DEFAULT_COLOR_2)];
  $: deg = gradientAngle(angle);

  let colorHistory = [];
  let gradientHistory = []; // recent color pairs, one click sets both colors + angle

  let wheelCanvas;
  let svCanvas;

  function loadHistory(key) {
    try {
      const stored = localStorage.getItem(key);
      if (stored) return JSON.parse(stored);
    } catch {}
    return [];
  }

  onMount(() => {
    colorHistory = loadHistory(HISTORY_KEY).filter(c => hexToRgb(c || ''));
    gradientHistory = loadHistory(GRADIENT_HISTORY_KEY)
      .filter(p => p && hexToRgb(p.c1 || '') && hexToRgb(p.c2 || ''));
  });

  // ── Color state ────────────────────────────────────────────────────────────

  function fromHex(h) {
    const rgb = hexToRgb(h || '');
    if (!rgb) return null;
    const hsv = rgbToHsv(rgb.r, rgb.g, rgb.b);
    const hex = rgbToHex(rgb.r, rgb.g, rgb.b);
    return { h: hsv.h, s: hsv.s * 100, v: hsv.v * 100, ...rgb, hex, hexInput: hex };
  }

  function setHsv(i, h, s, v) {
    const rgb = hsvToRgb(h, s, v);
    const hex = rgbToHex(rgb.r, rgb.g, rgb.b);
    cols[i] = { h, s, v, ...rgb, hex, hexInput: hex };
  }

  function setHex(i, h) {
    const c = fromHex(h);
    if (!c) return;
    // keep the hue of greys/black so the wheel handle doesn't jump to red
    if (c.s === 0 || c.v === 0) c.h = cols[i].h;
    cols[i] = c;
  }

  function onHexChange(i) {
    let v = (cols[i].hexInput || '').trim();
    if (!v.startsWith('#')) v = '#' + v;
    if (/^#[0-9a-fA-F]{6}$/.test(v)) setHex(i, v);
    else cols[i].hexInput = cols[i].hex; // invalid: back to the current color
  }

  function onRgbChange(i) {
    const k = (x) => Math.max(0, Math.min(255, parseInt(x) || 0));
    setHex(i, rgbToHex(k(cols[i].r), k(cols[i].g), k(cols[i].b)));
  }

  function onHueSlider(i) {
    setHsv(i, Number(cols[i].h), cols[i].s, cols[i].v);
  }

  function swapColors() {
    cols = [cols[1], cols[0]];
  }

  function onAngleChange() {
    angle = deg;
  }

  // ── Hue wheel (Uni tab): hue ring + saturation/brightness square ─────────

  const WHEEL = 220;               // CSS px
  const R_OUT = WHEEL / 2 - 4;     // room for the ring handle
  const RING = 20;
  const R_IN = R_OUT - RING;
  const SQ = 120;                  // inner square side
  const SQ0 = (WHEEL - SQ) / 2;

  $: drawWheel(wheelCanvas, cols[0]);
  $: drawSV(svCanvas, cols[sel]);

  // Hue 0 (red) at the top, increasing clockwise.
  function drawWheel(cv, col) {
    if (!cv || !col) return;
    const ctx = cv.getContext('2d');
    const k = cv.width / WHEEL;
    ctx.setTransform(k, 0, 0, k, 0, 0);
    ctx.clearRect(0, 0, WHEEL, WHEEL);
    const c = WHEEL / 2;

    // ring: one thin sector per degree (no createConicGradient, old WebKitGTK)
    for (let d = 0; d < 360; d++) {
      const a0 = (d - 90 - 0.5) * Math.PI / 180;
      const a1 = (d - 90 + 1.5) * Math.PI / 180;
      ctx.beginPath();
      ctx.arc(c, c, R_OUT, a0, a1);
      ctx.arc(c, c, R_IN, a1, a0, true);
      ctx.closePath();
      ctx.fillStyle = `hsl(${d}, 100%, 50%)`;
      ctx.fill();
    }

    // square
    ctx.save();
    roundedRect(ctx, SQ0, SQ0, SQ, SQ, 8);
    ctx.clip();
    paintSVRect(ctx, SQ0, SQ0, SQ, SQ, col.h);
    ctx.restore();

    // ring handle
    const ha = (col.h - 90) * Math.PI / 180;
    const rm = (R_OUT + R_IN) / 2;
    const hue = hsvToRgb(col.h % 360, 100, 100);
    drawHandle(ctx, c + rm * Math.cos(ha), c + rm * Math.sin(ha), RING / 2 - 1, `rgb(${hue.r},${hue.g},${hue.b})`);

    // square handle
    drawHandle(ctx, SQ0 + (col.s / 100) * SQ, SQ0 + (1 - col.v / 100) * SQ, 7, col.hex);
  }

  function drawSV(cv, col) {
    if (!cv || !col) return;
    const ctx = cv.getContext('2d');
    const w = cv.width, h = cv.height;
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    paintSVRect(ctx, 0, 0, w, h, col.h);
    const cssW = cv.getBoundingClientRect().width;
    const k = cssW > 0 ? w / cssW : 2;
    drawHandle(ctx, (col.s / 100) * w, (1 - col.v / 100) * h, 8 * k, col.hex, k);
  }

  // Saturation/brightness area of a hue.
  function paintSVRect(ctx, x, y, w, h, hue) {
    const rgb = hsvToRgb(hue % 360, 100, 100);
    ctx.fillStyle = `rgb(${rgb.r},${rgb.g},${rgb.b})`;
    ctx.fillRect(x, y, w, h);
    const white = ctx.createLinearGradient(x, 0, x + w, 0);
    white.addColorStop(0, 'rgba(255,255,255,1)');
    white.addColorStop(1, 'rgba(255,255,255,0)');
    ctx.fillStyle = white;
    ctx.fillRect(x, y, w, h);
    const black = ctx.createLinearGradient(0, y + h, 0, y);
    black.addColorStop(0, 'rgba(0,0,0,1)');
    black.addColorStop(1, 'rgba(0,0,0,0)');
    ctx.fillStyle = black;
    ctx.fillRect(x, y, w, h);
  }

  // White ring handle filled with the color, thin dark outline for light areas.
  function drawHandle(ctx, x, y, r, fill, k = 1) {
    ctx.beginPath();
    ctx.arc(x, y, r, 0, Math.PI * 2);
    ctx.fillStyle = fill;
    ctx.fill();
    ctx.lineWidth = 3 * k;
    ctx.strokeStyle = 'white';
    ctx.stroke();
    ctx.beginPath();
    ctx.arc(x, y, r + 1.5 * k, 0, Math.PI * 2);
    ctx.lineWidth = 1 * k;
    ctx.strokeStyle = 'rgba(0,0,0,0.45)';
    ctx.stroke();
  }

  function roundedRect(ctx, x, y, w, h, r) {
    ctx.beginPath();
    ctx.moveTo(x + r, y);
    ctx.arcTo(x + w, y, x + w, y + h, r);
    ctx.arcTo(x + w, y + h, x, y + h, r);
    ctx.arcTo(x, y + h, x, y, r);
    ctx.arcTo(x, y, x + w, y, r);
    ctx.closePath();
  }

  // Drag helper: onPick(ev) on press and while moving, until mouseup.
  function drag(e, onPick) {
    onPick(e);
    const onMove = (ev) => onPick(ev);
    const onUp = () => {
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
    e.preventDefault();
  }

  const clamp01 = (x) => Math.max(0, Math.min(1, x));

  function wheelDown(e) {
    const pos = (ev) => {
      const rect = wheelCanvas.getBoundingClientRect();
      const k = rect.width / WHEEL;
      return { x: (ev.clientX - rect.left) / k, y: (ev.clientY - rect.top) / k };
    };
    const p = pos(e);
    const c = WHEEL / 2;
    const dist = Math.hypot(p.x - c, p.y - c);
    let target = null;
    if (dist >= R_IN - 4 && dist <= R_OUT + 4) target = 'ring';
    else if (Math.abs(p.x - c) <= SQ / 2 + 6 && Math.abs(p.y - c) <= SQ / 2 + 6) target = 'square';
    if (!target) return;
    drag(e, (ev) => {
      const q = pos(ev);
      const col = cols[0];
      if (target === 'ring') {
        const h = (Math.atan2(q.y - c, q.x - c) * 180 / Math.PI + 90 + 360) % 360;
        setHsv(0, h, col.s, col.v);
      } else {
        setHsv(0, col.h, clamp01((q.x - SQ0) / SQ) * 100, (1 - clamp01((q.y - SQ0) / SQ)) * 100);
      }
    });
  }

  function svDown(e) {
    drag(e, (ev) => {
      const rect = svCanvas.getBoundingClientRect();
      const col = cols[sel];
      setHsv(sel, col.h,
        clamp01((ev.clientX - rect.left) / rect.width) * 100,
        (1 - clamp01((ev.clientY - rect.top) / rect.height)) * 100);
    });
  }

  // ── Footer ─────────────────────────────────────────────────────────────────

  // Back to factory accent: default colors, solid, 135°.
  function resetToDefault() {
    cols = [fromHex(DEFAULT_COLOR), fromHex(DEFAULT_COLOR_2)];
    angle = DEFAULT_GRADIENT_ANGLE;
    tab = 'solid';
    sel = 0;
  }

  function apply() {
    const isGradient = tab === 'gradient';
    const [c1, c2] = [cols[0].hex, cols[1].hex];
    if (isGradient) {
      gradientHistory = [{ c1, c2, a: deg }, ...gradientHistory.filter(p => p.c1 !== c1 || p.c2 !== c2 || gradientAngle(p.a) !== deg)].slice(0, 8);
      try { localStorage.setItem(GRADIENT_HISTORY_KEY, JSON.stringify(gradientHistory)); } catch {}
    } else {
      colorHistory = [c1, ...colorHistory.filter(c => c !== c1)].slice(0, 8);
      try { localStorage.setItem(HISTORY_KEY, JSON.stringify(colorHistory)); } catch {}
    }
    onApply({ color: c1, gradient: isGradient, color2: c2, angle: deg });
  }

  function pickRecentGradient(p) {
    cols = [fromHex(p.c1), fromHex(p.c2)];
    angle = gradientAngle(p.a);
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

    <!-- Uni / Dégradé: the tab shown when saving is the mode applied -->
    <div class="seg tabs" role="tablist">
      <button type="button" role="tab" class="seg-btn" class:active={tab === 'solid'} aria-selected={tab === 'solid'} on:click={() => tab = 'solid'}>
        <span class="tab-dot" style="background: {cols[0].hex}"></span>{$t('colorTabSolid')}
      </button>
      <button type="button" role="tab" class="seg-btn" class:active={tab === 'gradient'} aria-selected={tab === 'gradient'} on:click={() => tab = 'gradient'}>
        <span class="tab-dot" style="background: linear-gradient({deg}deg, {cols[0].hex}, {cols[1].hex})"></span>{$t('accentGradient')}
      </button>
    </div>

    {#if tab === 'solid'}
      <div class="wheel-wrap">
        <canvas
          bind:this={wheelCanvas}
          class="wheel"
          width={WHEEL * 2}
          height={WHEEL * 2}
          on:mousedown={wheelDown}
        ></canvas>
      </div>

      <div class="seg small" style="--cur: {cols[0].hex}; --cur-subtle: rgba({cols[0].r}, {cols[0].g}, {cols[0].b}, 0.18)">
        <button type="button" class="seg-btn" class:active={solidMode === 'hex'} on:click={() => solidMode = 'hex'}>HEX</button>
        <button type="button" class="seg-btn" class:active={solidMode === 'rgb'} on:click={() => solidMode = 'rgb'}>RGB</button>
      </div>

      {#if solidMode === 'hex'}
        <input
          class="hex-input big"
          type="text"
          bind:value={cols[0].hexInput}
          on:change={() => onHexChange(0)}
          maxlength="7"
          spellcheck="false"
          aria-label="HEX"
        />
      {:else}
        <div class="rgb-row">
          <label class="field"><span class="input-label">R</span><input class="num" type="number" min="0" max="255" bind:value={cols[0].r} on:change={() => onRgbChange(0)} /></label>
          <label class="field"><span class="input-label">G</span><input class="num" type="number" min="0" max="255" bind:value={cols[0].g} on:change={() => onRgbChange(0)} /></label>
          <label class="field"><span class="input-label">B</span><input class="num" type="number" min="0" max="255" bind:value={cols[0].b} on:change={() => onRgbChange(0)} /></label>
        </div>
      {/if}

      <div class="divider"></div>

      <div class="history-section">
        <span class="input-label">{$t('colorHistory')}</span>
        <div class="history-swatches">
          {#each colorHistory as c}
            <button class="history-swatch round" style="background: {c}" title={c} on:click={() => setHex(0, c)}></button>
          {/each}
          {#each { length: 8 - colorHistory.length } as _}
            <div class="history-swatch-empty round"></div>
          {/each}
        </div>
      </div>

    {:else}
      <!-- Preview; the CSS value only shows on hover -->
      <div class="grad-preview" style="background: linear-gradient({deg}deg, {cols[0].hex}, {cols[1].hex})">
        <span class="grad-code">linear-gradient({deg}°)</span>
      </div>

      <!-- The two points: click one to edit it below -->
      <div class="stops">
        <div class="stop-track" style="background: linear-gradient(to right, {cols[0].hex}, {cols[1].hex})">
          <button type="button" class="stop-handle start" class:selected={sel === 0} style="background: {cols[0].hex}" title={$t('gradientStopStart')} aria-label={$t('gradientStopStart')} on:click={() => sel = 0}></button>
          <button type="button" class="stop-handle end" class:selected={sel === 1} style="background: {cols[1].hex}" title={$t('gradientStopEnd')} aria-label={$t('gradientStopEnd')} on:click={() => sel = 1}></button>
        </div>
        <div class="stop-labels">
          <span>0 % {cols[0].hex}</span>
          <span>100 % {cols[1].hex}</span>
        </div>
      </div>

      <div class="angle-row">
        <span class="input-label">{$t('gradientAngle')}</span>
        <input type="range" class="angle-slider" min="0" max="360" bind:value={angle} style="--pct: {deg / 3.6}%" />
        <div class="angle-box">
          <input class="num" type="number" min="0" max="360" bind:value={angle} on:change={onAngleChange} aria-label={$t('gradientAngle')} />
          <span>°</span>
        </div>
      </div>

      <div class="divider"></div>

      <div class="section-title">{$t('gradientResult')}</div>
      <div class="result-row">
        <button type="button" class="chip" class:selected={sel === 0} on:click={() => sel = 0} title={$t('gradientStopStart')}>
          <span class="chip-dot" style="background: {cols[0].hex}"></span>{cols[0].hex}
        </button>
        <button type="button" class="swap-btn" on:click={swapColors} title={$t('gradientSwap')} aria-label={$t('gradientSwap')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M7 4 3 8l4 4"/><path d="M3 8h14"/><path d="m17 20 4-4-4-4"/><path d="M21 16H7"/></svg>
        </button>
        <button type="button" class="chip" class:selected={sel === 1} on:click={() => sel = 1} title={$t('gradientStopEnd')}>
          <span class="chip-dot" style="background: {cols[1].hex}"></span>{cols[1].hex}
        </button>
      </div>

      <div class="section-title"><span class="tab-dot" style="background: {cols[sel].hex}"></span>{$t('gradientSelectedPoint')}</div>

      <canvas
        bind:this={svCanvas}
        class="sv-canvas"
        width="560"
        height="320"
        on:mousedown={svDown}
      ></canvas>

      <!-- Preview of the point being edited + the gradient it gives, next to
           the hue slider so it stays in view while picking -->
      <div class="hue-row">
        <div class="point-preview" style="background: {cols[sel].hex}" title={cols[sel].hex}></div>
        <div class="hue-col">
          <input
            type="range"
            class="hue-slider"
            min="0" max="360"
            bind:value={cols[sel].h}
            on:input={() => onHueSlider(sel)}
          />
          <div class="mini-gradient" style="background: linear-gradient(to right, {cols[0].hex}, {cols[1].hex})"></div>
        </div>
      </div>

      <div class="point-row">
        <label class="field hex-field">
          <span class="input-label">HEX</span>
          <input class="hex-input" type="text" bind:value={cols[sel].hexInput} on:change={() => onHexChange(sel)} maxlength="7" spellcheck="false" />
        </label>
        <label class="field"><span class="input-label">R</span><input class="num" type="number" min="0" max="255" bind:value={cols[sel].r} on:change={() => onRgbChange(sel)} /></label>
        <label class="field"><span class="input-label">G</span><input class="num" type="number" min="0" max="255" bind:value={cols[sel].g} on:change={() => onRgbChange(sel)} /></label>
        <label class="field"><span class="input-label">B</span><input class="num" type="number" min="0" max="255" bind:value={cols[sel].b} on:change={() => onRgbChange(sel)} /></label>
      </div>

      <div class="divider"></div>

      <div class="history-section">
        <span class="input-label">{$t('recentGradients')}</span>
        <div class="history-swatches">
          {#each gradientHistory as p}
            <button
              class="history-swatch"
              style="background: linear-gradient({gradientAngle(p.a)}deg, {p.c1}, {p.c2})"
              title="{p.c1} → {p.c2} ({gradientAngle(p.a)}°)"
              on:click={() => pickRecentGradient(p)}
            ></button>
          {/each}
          {#each { length: 8 - gradientHistory.length } as _}
            <div class="history-swatch-empty"></div>
          {/each}
        </div>
      </div>
    {/if}

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

/* ── Segmented buttons (tabs, HEX/RGB) ── */
.seg {
  display: flex;
  gap: 4px;
  padding: 4px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--bg-primary);
  flex-shrink: 0;
}
.seg-btn {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 9px 8px;
  border: 1px solid transparent;
  border-radius: 7px;
  background: transparent;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}
.seg-btn:hover { color: var(--text-primary); }
.seg-btn.active {
  background: var(--bg-hover);
  border-color: var(--border);
  color: var(--text-primary);
}
.seg.small { padding: 0; border: none; background: none; gap: 8px; }
.seg.small .seg-btn {
  border-color: var(--border);
  font-size: 12px;
  padding: 8px;
}
/* colored with the color being edited, not the saved accent */
.seg.small .seg-btn.active {
  background: var(--cur-subtle);
  border-color: var(--cur);
}
.tab-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  flex-shrink: 0;
}

/* ── Uni: hue wheel ── */
.wheel-wrap {
  display: flex;
  justify-content: center;
  padding: 4px 0;
}
.wheel {
  width: 220px;
  height: 220px;
  cursor: crosshair;
  display: block;
}

.divider {
  height: 1px;
  background: var(--border);
  flex-shrink: 0;
}

/* ── Inputs ── */
.input-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.hex-input, .num {
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-primary);
  padding: 7px 8px;
  font-size: 13px;
  outline: none;
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
}
.hex-input { font-family: monospace; }
.hex-input.big { padding: 10px 12px; font-size: 14px; }
.num {
  text-align: center;
  -moz-appearance: textfield;
}
.num::-webkit-inner-spin-button,
.num::-webkit-outer-spin-button { -webkit-appearance: none; margin: 0; }
.hex-input:focus, .num:focus { border-color: var(--accent); }

.rgb-row, .point-row {
  display: flex;
  gap: 8px;
}
.field {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.point-row .hex-field { flex: 1.9; }

/* ── Dégradé ── */
.grad-preview {
  position: relative;
  height: 90px;
  border-radius: 10px;
  border: 1px solid var(--border);
  flex-shrink: 0;
}
.grad-code {
  position: absolute;
  top: 50%;
  left: 12px;
  transform: translateY(-50%);
  padding: 5px 9px;
  border-radius: 6px;
  background: rgba(10, 12, 20, 0.75);
  color: #fff;
  font-family: monospace;
  font-size: 12px;
  opacity: 0;
  transition: opacity 0.15s;
  pointer-events: none;
}
.grad-preview:hover .grad-code { opacity: 1; }

.stops { display: flex; flex-direction: column; gap: 8px; padding: 0 2px; }
.stop-track {
  position: relative;
  height: 16px;
  border-radius: 8px;
  margin: 6px 11px;
}
.stop-handle {
  position: absolute;
  top: 50%;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  border: 3px solid #fff;
  box-shadow: 0 0 0 1px rgba(0,0,0,0.45), 0 1px 4px rgba(0,0,0,0.4);
  padding: 0;
  cursor: pointer;
  transform: translate(-50%, -50%);
  transition: transform 0.12s, box-shadow 0.12s;
}
.stop-handle.start { left: 0; }
.stop-handle.end { left: 100%; }
.stop-handle.selected {
  transform: translate(-50%, -50%) scale(1.15);
  box-shadow: 0 0 0 1px rgba(0,0,0,0.45), 0 0 0 4px var(--accent), 0 1px 4px rgba(0,0,0,0.4);
}
.stop-labels {
  display: flex;
  justify-content: space-between;
  font-family: monospace;
  font-size: 11px;
  color: var(--text-muted);
}

.angle-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.angle-slider {
  flex: 1;
  -webkit-appearance: none;
  height: 4px;
  border-radius: 2px;
  background: linear-gradient(to right, var(--accent) var(--pct), var(--border) var(--pct));
  outline: none;
  cursor: pointer;
}
.angle-slider::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: #fff;
  border: 2px solid rgba(0,0,0,0.3);
  box-shadow: 0 1px 4px rgba(0,0,0,0.3);
  cursor: pointer;
}
.angle-box {
  display: flex;
  align-items: center;
  gap: 4px;
  width: 72px;
  flex-shrink: 0;
  color: var(--text-muted);
  font-size: 13px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
}

.result-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: -4px;
}
.chip {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-primary);
  color: var(--text-primary);
  font-family: monospace;
  font-size: 13px;
  cursor: pointer;
}
.chip:hover { background: var(--bg-hover); }
.chip.selected { border-color: var(--accent); }
.chip-dot {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 1px solid rgba(255,255,255,0.25);
  flex-shrink: 0;
}
.swap-btn {
  width: 38px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-primary);
  color: var(--text-secondary);
  cursor: pointer;
  flex-shrink: 0;
  padding: 0;
}
.swap-btn:hover { color: var(--accent); border-color: var(--accent); }
.swap-btn svg { width: 16px; height: 16px; }

.sv-canvas {
  width: 100%;
  height: 160px;
  border-radius: 8px;
  cursor: crosshair;
  display: block;
  border: 1px solid var(--border);
  flex-shrink: 0;
}

.hue-slider {
  -webkit-appearance: none;
  height: 14px;
  border-radius: 7px;
  background: linear-gradient(to right,
    hsl(0,100%,50%), hsl(30,100%,50%), hsl(60,100%,50%), hsl(90,100%,50%),
    hsl(120,100%,50%), hsl(150,100%,50%), hsl(180,100%,50%), hsl(210,100%,50%),
    hsl(240,100%,50%), hsl(270,100%,50%), hsl(300,100%,50%), hsl(330,100%,50%), hsl(360,100%,50%)
  );
  cursor: pointer;
  outline: none;
  border: none;
  flex-shrink: 0;
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

.hue-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.hue-col {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.hue-col .hue-slider { width: 100%; margin: 0; }
.point-preview {
  width: 46px;
  height: 46px;
  border-radius: 50%;
  border: 3px solid var(--bg-primary);
  box-shadow: 0 0 0 1px var(--border);
  flex-shrink: 0;
}
.mini-gradient {
  height: 8px;
  border-radius: 4px;
  border: 1px solid var(--border);
}

/* ── Recent colors (round) / recent gradients (square) ── */
.history-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.history-swatches {
  display: flex;
  gap: 7px;
  flex-wrap: wrap;
}
.history-swatch {
  width: 30px;
  height: 30px;
  border-radius: 6px;
  border: 2px solid var(--border);
  cursor: pointer;
  padding: 0;
  transition: border-color 0.1s, transform 0.1s;
  flex-shrink: 0;
}
.history-swatch:hover {
  border-color: var(--accent);
  transform: scale(1.12);
}
.history-swatch-empty {
  width: 30px;
  height: 30px;
  border-radius: 6px;
  border: 2px dashed var(--border);
  flex-shrink: 0;
  opacity: 0.4;
  box-sizing: border-box;
}
.history-swatch.round, .history-swatch-empty.round { border-radius: 50%; }

/* ── Footer ── */
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
  background: var(--accent-bg);
  border: none;
  border-radius: 5px;
  color: white;
  padding: 7px 16px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
}
.btn-apply:hover { background: var(--accent-hover-bg); }
</style>
