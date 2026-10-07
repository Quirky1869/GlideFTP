// Date formatting for the file lists' "Date" column, driven by the
// `dateFormat` setting. A format is a free string where `%`-prefixed tokens
// are replaced by date parts (e.g. "%dd/%MM/%yyyy %hh:%mm"). Tokens are
// case-sensitive: uppercase M = month, lowercase m = minutes.
// The special value "system" keeps the OS locale's own date/time format.

export const DEFAULT_DATE_FORMAT = '%yyyy-%MM-%dd %hh:%mm';
export const SYSTEM_DATE_FORMAT = 'system';

// Presets offered in the Settings dropdown. Anything else is "advanced".
export const DATE_FORMAT_PRESETS = [
  DEFAULT_DATE_FORMAT,
  '%yyyy-%MM-%dd %hh:%mm:%ss',
  '%dd/%MM/%yyyy %hh:%mm',
  '%dd/%MM/%yy %hh:%mm',
  '%MM/%dd/%yyyy %I:%mm %p',
  '%dd.%MM.%yyyy %hh:%mm',
  '%dd %MMM %yyyy %hh:%mm',
  SYSTEM_DATE_FORMAT,
];

// Shown in the "i" tooltip next to the advanced input (label = i18n key).
export const DATE_FORMAT_TOKENS = [
  { token: '%yyyy', label: 'dateTokYear4' },
  { token: '%yy',   label: 'dateTokYear2' },
  { token: '%MMMM', label: 'dateTokMonthLong' },
  { token: '%MMM',  label: 'dateTokMonthShort' },
  { token: '%MM',   label: 'dateTokMonth2' },
  { token: '%M',    label: 'dateTokMonth' },
  { token: '%dd',   label: 'dateTokDay2' },
  { token: '%d',    label: 'dateTokDay' },
  { token: '%hh',   label: 'dateTokHour2' },
  { token: '%h',    label: 'dateTokHour' },
  { token: '%II',   label: 'dateTokHour12_2' },
  { token: '%I',    label: 'dateTokHour12' },
  { token: '%p',    label: 'dateTokAmPm' },
  { token: '%mm',   label: 'dateTokMinute2' },
  { token: '%m',    label: 'dateTokMinute' },
  { token: '%ss',   label: 'dateTokSecond2' },
  { token: '%s',    label: 'dateTokSecond' },
  { token: '%%',    label: 'dateTokPercent' },
];

// Longest alternatives first so "%MMMM" isn't read as "%MM" + "MM".
const TOKEN_RE = /%(yyyy|yy|MMMM|MMM|MM|M|dd|d|hh|h|II|I|p|mm|m|ss|s|%)/g;

const pad = (n) => String(n).padStart(2, '0');

// Older settings files stored an unused Go layout ("2006-01-02 15:04") -
// anything without a token falls back to the default instead of being
// printed literally.
export function normalizeDateFormat(fmt) {
  if (fmt === SYSTEM_DATE_FORMAT) return fmt;
  if (!fmt || !fmt.includes('%')) return DEFAULT_DATE_FORMAT;
  return fmt;
}

export function formatDateWith(date, fmt, locale = 'en') {
  const d = date instanceof Date ? date : new Date(date);
  if (isNaN(d.getTime())) return '';
  fmt = normalizeDateFormat(fmt);
  if (fmt === SYSTEM_DATE_FORMAT) {
    return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }
  const h = d.getHours();
  const h12 = h % 12 || 12;
  return fmt.replace(TOKEN_RE, (_, tok) => {
    switch (tok) {
      case 'yyyy': return String(d.getFullYear());
      case 'yy':   return pad(d.getFullYear() % 100);
      case 'MMMM': return d.toLocaleString(locale, { month: 'long' });
      case 'MMM':  return d.toLocaleString(locale, { month: 'short' });
      case 'MM':   return pad(d.getMonth() + 1);
      case 'M':    return String(d.getMonth() + 1);
      case 'dd':   return pad(d.getDate());
      case 'd':    return String(d.getDate());
      case 'hh':   return pad(h);
      case 'h':    return String(h);
      case 'II':   return pad(h12);
      case 'I':    return String(h12);
      case 'p':    return h < 12 ? 'AM' : 'PM';
      case 'mm':   return pad(d.getMinutes());
      case 'm':    return String(d.getMinutes());
      case 'ss':   return pad(d.getSeconds());
      case 's':    return String(d.getSeconds());
      case '%':    return '%';
    }
    return tok;
  });
}
