# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GlideFTP is a desktop FTP/SFTP client built with Go + Wails v2 + Svelte. Fully implemented. Target platforms: Windows and Linux.

Design spec (French) in `prompt-glideftp`. UI reference sketch in `_images/exemple.png`.

## Build & Run

```bash
# Recommended — use the build script at project root
./build.sh            # builds Linux binary + Windows exe + AppImage
./build.sh linux      # Linux only      → build/bin/linux/GlideFTP
./build.sh windows    # Windows only    → build/bin/windows/GlideFTP.exe
./build.sh appimage   # AppImage only   → build/bin/linux/GlideFTP-x86_64.AppImage
# AppImage: downloads linuxdeploy to tools/ on first run (gitignored); requires imagemagick
# Key env vars set internally: NO_STRIP=1 (linuxdeploy's strip too old for .relr.dyn on Arch),
# icon resized to 256x256 (linuxdeploy requires standard resolution)

# Create distribution archives (requires built binaries first)
./create-archive.sh 1.7.0                       # all 6 archives (Linux+Windows+AppImage × gz+tar)
./create-archive.sh -p linux 1.7.0              # Linux binary archives only (includes README.md)
./create-archive.sh -p appimage 1.7.0           # AppImage archives only (no README.md)
./create-archive.sh -p windows -t gz 1.7.0      # Windows .tar.gz only
# Version must be X.Y.Z (3 numbers) — script refuses anything else
# Linux binary archives include README.md with libwebkit2gtk-4.1 install instructions
# AppImage archives contain only the .AppImage (self-contained, no deps needed)

# Manual — Linux (system has webkit2gtk-4.1, the tag is mandatory)
wails build -tags webkit2_41        # → build/bin/GlideFTP (then move to build/bin/linux/)
wails dev   -tags webkit2_41        # dev mode with hot reload

# Manual — Windows cross-compile from Linux (requires mingw-w64-gcc)
# Install: sudo pacman -S mingw-w64-gcc
CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows wails build -platform windows/amd64

# Frontend only
cd frontend && npm install && npm run build

# Run the compiled binary
./build/bin/linux/GlideFTP
```

> **Note:** `wails` must be on PATH — install with `go install github.com/wailsapp/wails/v2/cmd/wails@latest` then `export PATH="$PATH:$(go env GOPATH)/bin"`.
> Windows builds use WebView2 (built into Windows 10/11) — do NOT add `-tags webkit2_41` for Windows.

### Icon files — two separate assets

| File | Used by |
|---|---|
| `build/appicon.png` | Window taskbar icon (Linux/macOS) |
| `build/windows/icon.ico` | Icon **embedded inside the .exe** (Windows resource compiler) |

**Replacing only `appicon.png` does NOT update the Windows .exe icon.** `build/windows/icon.ico` must be regenerated too:
```bash
magick build/appicon.png -define icon:auto-resize="256,128,64,48,32,16" build/windows/icon.ico
```
`build.sh` does this automatically before each Windows build when `appicon.png` is newer than `icon.ico` (requires `imagemagick` — `sudo pacman -S imagemagick`).

## Architecture

```
GlideFTP/
├── main.go                        # Wails entry point (1280×800)
├── app.go                         # All Go→JS bindings (the only Wails-bound struct)
├── internal/
│   ├── connection/
│   │   ├── types.go               # Shared types: Config, ConnInfo, RemoteFileEntry, Client interface
│   │   ├── manager.go             # Thread-safe multi-connection manager (Connect/ConnectNew/CloseOne/SwitchTo…)
│   │   ├── ftp.go                 # FTP client (github.com/jlaffaye/ftp)
│   │   ├── sftp.go                # SFTP client (github.com/pkg/sftp + golang.org/x/crypto/ssh)
│   │   └── ppk.go                 # PuTTY .ppk key parser (v2/v3, RSA + Ed25519, unencrypted)
│   ├── transfer/
│   │   └── queue.go               # Worker-pool transfer queue; emits Wails events for progress
│   ├── sites/
│   │   └── sites.go               # Saved sites — persisted to ~/.config/GlideFTP/sites.json
│   ├── settings/
│   │   └── settings.go            # App settings — persisted to ~/.config/GlideFTP/settings.json (includes MaxConnections)
│   └── fs/
│       └── local.go               # Local filesystem helpers (ListDir, MkDir, Delete, Rename)
└── frontend/src/
    ├── App.svelte                  # Root: disconnected (centered form) vs connected (tabs strip + dual panel)
    ├── style.css                   # Global CSS vars (themes: dark/light), html/body/app layout
    ├── i18n/{en,fr,index}.js       # EN/FR i18n via Svelte derived store
    ├── stores/
    │   ├── settings.js             # Loads/saves settings; loadSettings() returns the settings object
    │   ├── connection.js           # Connection state, local+remote path/entries stores
    │   └── transfers.js            # Transfer list store; completedTransfer store; Wails event subs
    └── components/
        ├── ConnectionBar.svelte    # Host/user/pass/port/protocol inputs + connect button
        ├── FileBrowser.svelte      # Single panel: nav, sort, multi-select, drag-drop, rename, delete
        ├── TransferQueue.svelte    # Bottom panel, resizable, 3 tabs: pending/failed/done
        ├── SettingsPanel.svelte    # Sliding panel (75% width from right); footer shows version badge (accent color, bottom-left) — update hardcoded "v1.7.0" string on each release
        ├── SiteManager.svelte      # Centered modal: create/edit/delete/connect saved sites
        └── ColorPicker.svelte      # Sliding overlay (z-index 500): HSV canvas + hue slider + RGB/HEX inputs
```

## Key Design Decisions

- **One `App` struct** in `app.go` is the single Wails binding — all methods on it are exposed to JS automatically.
- **Transfer progress** uses `runtime.EventsEmit` from Go → frontend subscribes with `EventsOn('transfer:progress', ...)`. Removal emits `transfer:removed`.
- **Theme** is applied via `document.documentElement.setAttribute('data-theme', 'dark'|'light')` — CSS vars defined in `style.css`.
- **Accent color** is applied via `applyAccentColor(hex)` in `settings.js` which sets `--accent`, `--accent-hover`, `--accent-subtle` CSS vars on `document.documentElement`.
- **i18n** is a Svelte `derived` store — `$t('key')` reactively switches language with no page reload.
- **Config files** are stored in the OS user config dir (`os.UserConfigDir()`): cross-platform without hardcoding paths.
- **SFTP auth** supports password, SSH key file (with optional passphrase), interactive keyboard, and SSH agent (`SSH_AUTH_SOCK`). Auth type `key` (= `AuthSSHKey`) handles both OpenSSH PEM keys and PuTTY `.ppk` format (v2/v3, RSA & Ed25519) — detection is automatic via `isPPKFile()` in `ppk.go`; encrypted PPK keys return a clear error asking to convert with PuTTYgen. Selecting SFTP auto-sets authType to `interactive` (preserves `key` if already set); selecting `interactive` or `key` auto-sets protocol to `sftp` (coupled in `SiteManager.svelte` via `setProtocol`/`setAuthType`).
- **FTP passive mode** is the default (configurable in settings).
- **FTP thread-safety**: `FTPClient` has a `sync.Mutex` — all methods lock it. The `jlaffaye/ftp` library is not thread-safe; without the mutex, concurrent queue jobs corrupt the connection.
- **Transfer cancellation**: each `Job` holds a `cancelFn context.CancelFunc` set in `queue.run()`. `progressReader.Read()` and `progressWriter.Write()` check `ctx.Err()` before each chunk — calling `cancelFn()` interrupts an in-progress transfer. `Cancel(id)` handles both `StatusPending` and `StatusRunning` jobs.
- **Reconnection**: `manager.Connect()` disconnects an existing connection before reconnecting — no "already connected" error.
- **Multi-connection tabs**: `manager.ConnectNew()` adds a connection alongside existing ones. The frontend tracks open connections in the `connections` writable store (`[{id, name, host, protocol, port, user, remotePath}]`). Tabs appear in `App.svelte` between the topbar and dual-browser only when `$connections.length > 1`. `switchTab(id)` saves the current remotePath before switching. `closeTab(id)` cleans up and auto-activates the next tab. When the disconnect button is clicked with 2+ open connections, a confirmation overlay asks to close all. `MaxConnections` (1–5, default 3) is a setting that controls how many can be kept open; `SiteManager` checks this before offering the "keep and open new" option.
- **Duplicate connection guard**: `doKeepAndAdd()` in `SiteManager.svelte` checks whether any entry in `$connections` already has the same `host`, `port`, `protocol`, and `user` before calling `addConnection()`. If a duplicate is detected → calls `connectBySite()` instead (reconnect, no new tab). For `ask_password` mode the `promptIsAdd` flag is set to `false` so the password prompt routes to `connectBySiteWithPassword()` rather than `addConnection()`.
- **DefaultLocalDir**: `initLocalDir(startDir?)` in connection.js uses the setting on startup; `loadSettings()` returns the settings object so `App.svelte` can pass it immediately.
- **ListDir timeout**: `manager.ListDir` wraps the blocking client call in a goroutine with a `time.After` timeout; on timeout it forces disconnect and returns an error so the UI doesn't freeze.

## WebKit-GTK UI Patterns (Linux)

The Wails WebView on Linux uses WebKit-GTK. These patterns are broken and **must not be used**:

1. **Hidden checkbox toggles** (`<label><input type="checkbox" hidden>`) — checkboxes never fire click events when hidden this way. **Use `<button class="sw" class:on={val} on:click={() => toggle(key)}>` instead.** See `SettingsPanel.svelte` for reference.

2. **Native number input spinners** — unreliable/invisible. **Use custom `−`/`+` buttons with a `step(key, delta, min, max)` helper.** Hide native spinners with `-moz-appearance: textfield` and `-webkit-appearance: none`.

3. **Dynamic `type` on `<input bind:value>`** — Svelte 3 compile error: `'type' attribute cannot be dynamic if input uses two-way binding`. **Use two separate inputs in `{#if}`/`{:else}` blocks** — one `type="text"`, one `type="password"`, both bound to the same variable. See `SiteManager.svelte` password prompt for reference.

4. **Ctrl+Z (undo) in inputs** — WebKit-GTK does not fire native undo in Svelte-bound inputs. Fixed globally in `App.svelte` `handleKeydown`: intercept `Ctrl+Z` on `INPUT`/`TEXTAREA`, call `document.execCommand('undo')`, then dispatch a synthetic `input` event so Svelte re-syncs its variable.

5. **Right-click context menu in inputs** — WebKit-GTK disables the native context menu in the WebView. Implement a custom paste menu via `on:contextmenu` + `navigator.clipboard.readText()`. See `SiteManager.svelte` password prompt for reference.

## FileBrowser Features

`FileBrowser.svelte` receives `side` ('local'|'remote'), `path`, `entries`, `selected`, `otherPath`, and action callbacks.

- **".." entry**: always shown at the top; click/dblclick calls `onNavigateUp`; focused via `parentFocused` state when ArrowUp is pressed from first entry
- **Keyboard navigation**: ArrowDown/ArrowUp moves selection through entries; ArrowUp from first entry sets `parentFocused = true` (highlights ".." row); Enter on dir navigates in; Enter when `parentFocused` calls `onNavigateUp`; `$: if (path) parentFocused = false` resets on navigation
- **Editable path bar**: click the path display to enter edit mode; Enter navigates, Esc cancels; debounced autocomplete dropdown shows matching subdirs
- **Column sort**: click Name/Size/Date headers; dirs always listed first; second click reverses order
- **Multi-select**: Ctrl+click toggles, Shift+click range-selects, rubber-band (click-drag on empty area)
- **F2 rename**: panel div is `tabindex="-1"` and focused on row click; keydown handler triggers rename on F2
- **Delete key**: keydown handler calls `handleDelete(selected)` — deletes the full selection
- **Right-click context menu**: on a file → Rename / Transfer / Delete (deletes full selection if right-clicked item is in selection); on empty area → New Folder
- **Delete confirmation**: `confirmDeleteEntries` (array); popup shows filename (1 item) or "N éléments" (multiple); `doDeleteAll()` iterates and calls `onDelete` for each, single refresh at end
- **Drag & drop**: rows are `draggable`; drag data is `{ entries: [{path, name}], fromSide }` — if the dragged row is in the current selection, all selected entries are included. Drop iterates over `entries` array.

## App.svelte Layout

- **Disconnected**: centered `.connect-card` with `ConnectionBar` and a link to open `SiteManager`
- **Connected**: optional `.conn-tabs` strip (only when `$connections.length > 1`), then dual-panel `FileBrowser` layout with a draggable `.browser-splitter` (20–80% range via `leftWidth` percent)
- **Connection tabs**: tab per open connection, min-width 160px; active tab has accent underline; ✕ button calls `closeTab(id)`; clicking a tab calls `switchTab(id)` which saves the current path and refreshes the remote panel for the new active connection
- **Disconnect-all overlay**: triggered via `onMultiDisconnect` prop on `ConnectionBar`; confirms closing all open connections
- **Auto-refresh**: `$: if ($completedTransfer)` triggers `refreshLocal` + `refreshRemote` after any finished transfer
- **Settings saved**: `handleSettingsSaved()` refreshes file lists so changes (e.g. showHiddenFiles) take effect immediately

## Key Stores & Functions

| Export | File | Purpose |
|---|---|---|
| `completedTransfer` | transfers.js | Writable; set to `{ ...job, _ts }` when a transfer finishes; used to trigger auto-refresh |
| `removeTransfer(id)` | transfers.js | Calls `RemoveTransfer` Go binding; frontend removes via `transfer:removed` event |
| `connections` | connection.js | Writable array of `{id, name, host, protocol, port, user, remotePath}` for all open connections |
| `activeConnectionId` | connection.js | Writable; UUID of the currently active connection tab |
| `addConnection(siteId, overridePassword?)` | connection.js | Calls `ConnectToSiteAdditional`; adds a connection without closing existing ones |
| `switchTab(id)` | connection.js | Saves current remotePath, calls `SwitchConnection`, refreshes remote panel for the new active connection |
| `closeTab(id)` | connection.js | Calls `CloseConnection`, removes from store; if last tab, clears all state; if was active, switches to last remaining |
| `connectBySite(id, config?)` | connection.js | Sets `connectionStatus` store correctly; optional `config` param populates `activeConnectionConfig` |
| `connectBySiteWithPassword(id, pwd, config?)` | connection.js | Like `connectBySite` but passes runtime password (for `ask_password` auth sites) |
| `activeConnectionConfig` | connection.js | Writable; set on every connect with `{ protocol, host, port, user }`; used by `ConnectionBar` to show real values when connected |
| `initLocalDir(startDir?)` | connection.js | Initializes local panel; pass `defaultLocalDir` from settings on startup |
| `loadSettings()` | settings.js | Returns the loaded settings object (in addition to updating the store) |
| `applyAccentColor(hex)` | settings.js | Sets `--accent`, `--accent-hover`, `--accent-subtle` CSS vars; called on load and save |

## Go Backend Notes

- `queue.RemoveJob(id)` — removes a finished/cancelled/failed job; emits `transfer:removed` event
- `app.RemoveTransfer(id)` — JS-callable wrapper around `queue.RemoveJob`
- `app.ConnectWithPassword(id, password)` — connects to a saved site but overrides its stored password (for `ask_password` sites)
- `app.ConnectToSiteAdditional(siteID, overridePassword)` — adds a new connection alongside existing ones (multi-tab); calls `manager.ConnectNew()`
- `app.GetConnections()` — returns `[]ConnInfo` for all currently open connections
- `app.SwitchConnection(id)` — switches the active connection and calls `queue.SetExecutor` with the new client
- `app.CloseConnection(id)` — closes a specific connection and calls `queue.SetExecutor` with the updated active client
- `app.GetActiveConnectionID()` — returns the UUID of the currently active connection
- `manager.ConnectNew()` — adds a connection without removing existing ones; new connection becomes active
- `manager.CloseOne(id)` — closes specific connection; if it was active, switches to the most-recently-added remaining one
- `manager.SwitchTo(id)` — makes a connection active without reconnecting
- `app.ExportSites()` / `app.ImportSites()` — file-dialog based JSON export/import of all saved sites
- `app.shutdown(ctx)` — registered as `OnShutdown` in `main.go`; calls `connMgr.Disconnect()` for clean teardown on window close
- `manager.Connect()` — disconnects existing active client first; other connections remain open (enables reconnection from SiteManager)
- `Client` interface (`types.go`) — `Upload` and `Download` now take `context.Context` as first arg; both `FTPClient` and `SFTPClient` implement this
- `FTPClient` — all methods are protected by `sync.Mutex`; FTP connections are not thread-safe
- **FTP Download order**: `FileSize` MUST be called BEFORE `Retr` in `ftp.go`. Calling it after opens a command on the control connection mid-transfer, which violates FTP protocol and causes Synology (and others) to return 0 bytes.

## TransferQueue

- Speed is computed in `TransferQueue.svelte` from deltas of `bytesDone` between store updates (250 ms window minimum to avoid noise). Stored in `speeds` map (`id → bytes/sec`), displayed as `KB/s` or `MB/s` in accent color next to the progress label.
- `ColorPicker.svelte` stores last 8 applied colors in `localStorage` key `glideftp_color_history`; displayed as swatches above the footer; click to select.
