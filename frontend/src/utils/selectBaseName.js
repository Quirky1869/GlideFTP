// Svelte action for rename inputs: focuses the input and selects the file
// name without its extension ("report" in "report.pdf"), like most file
// managers - so typing replaces the name but keeps the extension.
// Folders and dot-files without another dot (".bashrc") select everything.
// Only the last extension is excluded: "archive.tar.gz" selects "archive.tar".
export function selectBaseName(input, isDir = false) {
  // Deferred one frame so it runs after `autofocus`, which selects the
  // whole value on WebKit-GTK and would otherwise override this selection.
  const raf = requestAnimationFrame(() => {
    input.focus();
    const dot = isDir ? -1 : input.value.lastIndexOf('.');
    input.setSelectionRange(0, dot > 0 ? dot : input.value.length);
  });
  return { destroy: () => cancelAnimationFrame(raf) };
}
