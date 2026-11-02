// Release notes shown in the app (v1.8.0): the body of the GitHub release is
// v{version}.md, with a "## 🇬🇧 English" part and a "## 🇫🇷 Français" part.
// Only the part of the app's language is kept, without its download table,
// and the small Markdown subset used by the notes is turned into HTML.

const LANG_HEADINGS = { en: 'English', fr: 'Français' };
const DOWNLOAD_HEADINGS = ['Downloads', 'Téléchargements'];

// Keeps the "## ..." part of the given language (the whole text when the
// headings aren't found) and drops its downloads section.
export function notesForLocale(md, locale) {
  const lines = (md || '').replace(/\r\n?/g, '\n').split('\n');
  const want = LANG_HEADINGS[locale] || LANG_HEADINGS.en;
  const parts = [];
  let current = null;
  for (const line of lines) {
    if (line.startsWith('## ')) {
      current = { heading: line, lines: [] };
      parts.push(current);
    } else if (current) {
      current.lines.push(line);
    }
  }
  const part = parts.find(p => p.heading.includes(want));
  let body = part ? part.lines : lines;

  const out = [];
  let skipping = false;
  for (const line of body) {
    if (line.startsWith('### ')) skipping = DOWNLOAD_HEADINGS.some(h => line.includes(h));
    if (!skipping) out.push(line);
  }
  // Trailing separators / blank lines left before the removed section
  while (out.length && (!out[out.length - 1].trim() || out[out.length - 1].trim() === '---')) out.pop();
  return out.join('\n');
}

function escapeHtml(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// Inline marks on already-escaped text: `code`, **bold**, *italic*, and
// [text](url) reduced to its text (links can't open from here).
function inline(s) {
  return escapeHtml(s)
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/(^|[^*])\*([^*\s][^*]*)\*/g, '$1<em>$2</em>')
    .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1');
}

// Headings (##, ###, ####), paragraphs, "- " / "* " lists, "---". Everything
// is escaped first, so the notes can never inject HTML.
export function renderNotes(md) {
  const html = [];
  let para = [];
  let list = null;
  const flushPara = () => { if (para.length) html.push('<p>' + para.map(inline).join('<br>') + '</p>'); para = []; };
  const flushList = () => { if (list) html.push('<ul>' + list.map(li => '<li>' + inline(li) + '</li>').join('') + '</ul>'); list = null; };

  for (const raw of md.split('\n')) {
    const line = raw.trimEnd();
    const h = line.match(/^(#{2,4}) (.*)$/);
    if (h) {
      flushPara(); flushList();
      const level = h[1].length + 1; // ## → h3, ### → h4, #### → h5
      html.push(`<h${level}>${inline(h[2])}</h${level}>`);
    } else if (/^\s*[-*] /.test(line)) {
      flushPara();
      if (!list) list = [];
      list.push(line.replace(/^\s*[-*] /, ''));
    } else if (line.trim() === '---') {
      flushPara(); flushList();
      html.push('<hr>');
    } else if (!line.trim() || line.startsWith('|')) {
      flushPara(); flushList();
    } else {
      flushList();
      para.push(line);
    }
  }
  flushPara(); flushList();
  return html.join('\n');
}
