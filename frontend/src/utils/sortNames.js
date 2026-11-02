// File name order of the panels. Default: alphabetical, upper and lower case
// mixed, accents ignored (like a file explorer). strict (setting
// "caseSensitiveSort", like FileZilla): character by character - names
// starting with a dot, then UPPER CASE, then lower case.
export function compareNames(a, b, strict) {
  if (strict) return a < b ? -1 : a > b ? 1 : 0;
  return a.localeCompare(b, undefined, { sensitivity: 'base' });
}
