// v1.8.0 point 5 - panel comparison, with FileZilla's colours:
//   'only'  (yellow) = no entry of that name (and kind) on the other side
//   'newer' (green)  = newer than the other side's file (criterion "date")
//   'size'  (red)    = not the same size as the other side's file (criterion "size")
// Identical entries (and folders present on both sides) get no mark.

// Dates closer than this are equal: FTP listings are often minute-precise.
const DATE_TOLERANCE_MS = 60 * 1000;

const atMidnight = (d) => d.getHours() === 0 && d.getMinutes() === 0 && d.getSeconds() === 0;
const dayOf = (d) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();

// a newer than b? Old files are often listed by FTP servers with a date but
// no time (midnight): those are compared by day only.
function isNewer(a, b) {
  const da = new Date(a || 0), db = new Date(b || 0);
  if (isNaN(da) || isNaN(db)) return false;
  if (atMidnight(da) || atMidnight(db)) return dayOf(da) > dayOf(db);
  return da - db > DATE_TOLERANCE_MS;
}

// Returns a Map path → { kind, here, there } for the entries of this panel
// that differ from the other panel's. by: 'size' | 'date'. caseInsensitive:
// one side is a Windows folder.
export function compareEntries(entries, otherEntries, by, caseInsensitive) {
  const key = (n) => (caseInsensitive ? n.toLowerCase() : n);
  const other = new Map((otherEntries || []).map(e => [key(e.name), e]));
  const marks = new Map();
  for (const e of entries || []) {
    const o = other.get(key(e.name));
    if (!o || o.isDir !== e.isDir) {
      marks.set(e.path, { kind: 'only' });
    } else if (e.isDir) {
      continue;
    } else if (by === 'date') {
      if (isNewer(e.modTime, o.modTime)) marks.set(e.path, { kind: 'newer', here: e.modTime, there: o.modTime });
    } else if ((e.size || 0) !== (o.size || 0)) {
      marks.set(e.path, { kind: 'size', here: e.size || 0, there: o.size || 0 });
    }
  }
  return marks;
}

// "Align the rows of both panels" (setting compareAlign, like FileZilla):
// the rows of this panel = every name of both panels, folders first then
// files, each group sorted with cmpName; a name missing here becomes a gap
// { gap: true, key, isDir } so both panels show the same names on the same
// lines. Both panels build the same order. hide ("hide identical files"):
// a file is kept when it is marked on either side (marksHere / marksThere),
// so both panels keep the same rows.
export function alignEntries(entries, otherEntries, marksHere, marksThere, hide, caseInsensitive, cmpName) {
  const key = (e) => (e.isDir ? 'd:' : 'f:') + (caseInsensitive ? e.name.toLowerCase() : e.name);
  const here = new Map((entries || []).map(e => [key(e), e]));
  const there = new Map((otherEntries || []).map(e => [key(e), e]));
  const rows = [];
  for (const k of new Set([...here.keys(), ...there.keys()])) {
    const h = here.get(k), o = there.get(k);
    const any = h || o;
    if (hide && !any.isDir && !(h && marksHere?.has(h.path)) && !(o && marksThere?.has(o.path))) continue;
    rows.push(h || { gap: true, key: k, isDir: o.isDir, name: o.name });
  }
  return rows.sort((a, b) => (b.isDir - a.isDir) || cmpName(a.name, b.name));
}
