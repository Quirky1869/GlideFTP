# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GlideFTP is a desktop FTP/SFTP client built with Go + Wails v2 + Svelte. Fully implemented. Target platforms: Windows and Linux.

Design spec (French) in `prompt-glideftp`. UI reference sketch in `_images/exemple.png`.

Issue screenshots are stored in `./_images/issues/v{version}/` where `{version}` is the current app version. Example: for v1.7.5 in progress, screenshots are in `./_images/issues/v1.7.5/`. Always use the versioned subfolder matching the active release when referencing or looking up issue images.

## Build & Run

```bash
# Recommended - use the build script at project root
./make.sh                  # Linux binary + Windows exe + Arch AppImage + Debian AppImage (via Docker)
./make.sh linux            # Linux only             → build/bin/linux/GlideFTP
./make.sh windows          # Windows only           → build/bin/windows/GlideFTP.exe
./make.sh appimage         # Arch AppImage + Debian AppImage → build/bin/linux/GlideFTP-{Arch,Debian}-x86_64.AppImage
./make.sh appimage-arch    # Arch AppImage only    → build/bin/linux/GlideFTP-Arch-x86_64.AppImage
./make.sh appimage-debian  # Debian/Ubuntu AppImage → build/bin/linux/GlideFTP-Debian-x86_64.AppImage
#   appimage-debian uses Docker (requires docker or podman); first run builds Ubuntu 22.04 image (~10 min)
#   Arch AppImage:   linuxdeploy bundles Arch libs; requires GLIBC 2.38+ on target
#   Debian AppImage: built in Ubuntu 22.04 container; bundles Ubuntu libs; requires GLIBC 2.35+ (Ubuntu 22.04+/Debian 12+/Arch)
#   Both AppImages bundle WebKitNetworkProcess + WebKitWebProcess helpers; AppRun sets WEBKIT_EXEC_PATH
#   Key env vars (Arch build): NO_STRIP=1 (linuxdeploy's strip too old for .relr.dyn on Arch),
#   icon resized to 256x256 with magick (Arch) / convert (Debian container, ImageMagick 6)

# Create distribution archives (requires built binaries first)
./create-archive.sh 1.7.1                            # all 8 archives (Linux+Windows+ArchAppImage+DebianAppImage × gz+tar)
./create-archive.sh -p linux 1.7.1                   # Linux binary archives only (includes README.md)
./create-archive.sh -p appimage 1.7.1                # both AppImage variants (Arch + Debian)
./create-archive.sh -p appimage-arch 1.7.1           # Arch AppImage archives only
./create-archive.sh -p appimage-debian 1.7.1         # Debian AppImage archives only
./create-archive.sh -p windows -t gz 1.7.1           # Windows .tar.gz only
# Version must be X.Y.Z (3 numbers) - script refuses anything else
# Linux binary archives include README.md with libwebkit2gtk-4.1 install instructions
# AppImage archives contain only the .AppImage (self-contained, webkit helpers bundled)

# Manual - Linux (system has webkit2gtk-4.1, the tag is mandatory)
wails build -tags webkit2_41        # → build/bin/GlideFTP (then move to build/bin/linux/)
wails dev   -tags webkit2_41        # dev mode with hot reload

# Manual - Windows cross-compile from Linux (requires mingw-w64-gcc)
# Install: sudo pacman -S mingw-w64-gcc
CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows wails build -platform windows/amd64

# Frontend only
cd frontend && npm install && npm run build

# Run the compiled binary
./build/bin/linux/GlideFTP
```

> **Note:** `wails` must be on PATH - install with `go install github.com/wailsapp/wails/v2/cmd/wails@latest` then `export PATH="$PATH:$(go env GOPATH)/bin"`.
> Windows builds use WebView2 (built into Windows 10/11) - do NOT add `-tags webkit2_41` for Windows.

### Icon files - two separate assets

| File | Used by |
|---|---|
| `build/appicon.png` | Window taskbar icon (Linux/macOS) |
| `build/windows/icon.ico` | Icon **embedded inside the .exe** (Windows resource compiler) |

**Replacing only `appicon.png` does NOT update the Windows .exe icon.** `build/windows/icon.ico` must be regenerated too:
```bash
magick build/appicon.png -define icon:auto-resize="256,128,64,48,32,16" build/windows/icon.ico
```
`make.sh` does this automatically before each Windows build when `appicon.png` is newer than `icon.ico` (requires `imagemagick` - `sudo pacman -S imagemagick`).

## Architecture

```
GlideFTP/
├── main.go                        # Wails entry point (1280×800)
├── app.go                         # All Go→JS bindings (the only Wails-bound struct)
├── docker/
│   ├── Dockerfile.appimage        # Ubuntu 22.04 build environment (Go 1.25, Node 20, webkit2gtk-4.1-dev, linuxdeploy)
│   └── build-appimage.sh          # Script run inside the container: rsync source → wails build → linuxdeploy → AppImage
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
│   │   └── sites.go               # Saved sites - persisted to ~/.config/GlideFTP/sites.json (passwords NEVER written to disk)
│   ├── keyring/
│   │   └── keyring.go             # System keyring abstraction (go-keyring): Set/Get/Delete/IsAvailable - D-Bus/gnome-keyring on Linux, DPAPI on Windows
│   ├── crypto/
│   │   └── crypto.go              # .gfe encrypted export format: Argon2id(passphrase,salt) → AES-256-GCM; Encrypt/Decrypt/IsEncrypted
│   ├── settings/
│   │   └── settings.go            # App settings - persisted to ~/.config/GlideFTP/settings.json (includes MaxConnections, ConnectCardShadow)
│   └── fs/
│       └── local.go               # Local filesystem helpers (ListDir, MkDir, Delete, Rename)
└── frontend/src/
    ├── App.svelte                  # Root: disconnected (centered form) vs connected (tabs strip + dual panel)
    ├── style.css                   # Global CSS vars (themes: dark/light), html/body/app layout
    ├── i18n/{en,fr,index}.js       # EN/FR i18n via Svelte derived store
    ├── stores/
    │   ├── settings.js             # Loads/saves settings; loadSettings() returns the settings object
    │   ├── connection.js           # Connection state, local+remote path/entries stores; clipboard store for intra-panel copy/paste
    │   └── transfers.js            # Transfer list store; completedTransfer store; Wails event subs
    ├── utils/
    │   └── focusTrap.js            # Svelte action `trapFocus` - traps Tab/Shift+Tab inside a popup container
    └── components/
        ├── ConnectionBar.svelte    # Host/user/pass/port/protocol inputs + connect button + quick connect (↑/→ button)
        ├── FileBrowser.svelte      # Single panel: nav, sort, multi-select, drag-drop, rename, delete; tree view toggle (lazy-loaded hierarchy, files + dirs, per-panel state); intra-panel copy/cut/paste (Ctrl+C/X/V + right-click); intra-panel drag-drop onto folders and ".."
        ├── TransferQueue.svelte    # Bottom panel, resizable, 3 tabs: pending/failed/done
        ├── SettingsPanel.svelte    # Sliding panel (75% width from right); footer shows version badge (accent color, bottom-left) - update hardcoded version string on each release; reset buttons (↺) next to each label; window size + maximize settings
        ├── SiteManager.svelte      # Centered modal: create/edit/delete/connect/duplicate saved sites; form inputs have right-click cut/copy/paste menu; password field has eye toggle; detail panel fully centered (align-items center on .site-detail, .site-view, .site-view-header)
        └── ColorPicker.svelte      # Sliding overlay (z-index 500): HSV canvas + hue slider + RGB/HEX inputs
```

## Key Design Decisions

- **One `App` struct** in `app.go` is the single Wails binding - all methods on it are exposed to JS automatically.
- **Transfer progress** uses `runtime.EventsEmit` from Go → frontend subscribes with `EventsOn('transfer:progress', ...)`. Removal emits `transfer:removed`.
- **Theme** is applied via `document.documentElement.setAttribute('data-theme', 'dark'|'light')` - CSS vars defined in `style.css`.
- **Accent color** is applied via `applyAccentColor(hex)` in `settings.js` which sets `--accent`, `--accent-hover`, `--accent-subtle` CSS vars on `document.documentElement`.
- **i18n** is a Svelte `derived` store - `$t('key')` reactively switches language with no page reload.
- **Config files** are stored in the OS user config dir (`os.UserConfigDir()`): cross-platform without hardcoding paths.
- **SFTP auth** supports password, SSH key file (with optional passphrase), interactive keyboard, and SSH agent (`SSH_AUTH_SOCK`). Auth type `key` (= `AuthSSHKey`) handles both OpenSSH PEM keys and PuTTY `.ppk` format (v2/v3, RSA & Ed25519) - detection is automatic via `isPPKFile()` in `ppk.go`; encrypted PPK keys return a clear error asking to convert with PuTTYgen. Selecting SFTP auto-sets authType to `interactive` (preserves `key` if already set); selecting `interactive` or `key` auto-sets protocol to `sftp` (coupled in `SiteManager.svelte` via `setProtocol`/`setAuthType`). The `account` auth type has been removed - auth types are: Normal, Anonymous, Ask password, Interactive, SSH Key.
- **Port stepper in ConnectionBar**: the port field uses a custom `−`/`+` stepper (`.port-stepper` div) with a hidden-spinner number input between two buttons. `stepPort(delta)` skips port 22 when `protocol === 'ftp'` (jumps 21→23 going up, 23→21 going down). A reactive guard `$: if (!isConnected && protocol === 'ftp' && port === 22) port = 21` also handles direct keyboard entry of 22.
- **FTP passive mode** is the default (configurable in settings).
- **FTP concurrent transfers**: `FTPClient.Upload()` and `FTPClient.Download()` open a **dedicated FTP connection** per transfer via `dial()` (same credentials as the primary connection). The primary connection (`c.conn`) is reserved exclusively for control operations (ListDir, MkDir, Delete, Rename) and is protected by `c.mu`. This allows N concurrent transfers = N independent FTP connections, which is the standard approach (FileZilla etc.). The `jlaffaye/ftp` library is not thread-safe on a single connection - never run concurrent operations on the same `*ServerConn`.
- **Transfer cancellation**: each `Job` holds a `cancelFn context.CancelFunc` set in `queue.run()`. `progressReader.Read()` and `progressWriter.Write()` check `ctx.Err()` before each chunk - calling `cancelFn()` interrupts an in-progress transfer. `Cancel(id)` handles both `StatusPending` and `StatusRunning` jobs.
- **Concurrent transfer setting**: `Queue.SetWorkers(n int)` replaces the semaphore channel with a new one of capacity `n`. `run()` snapshots `q.sem` into a local variable before the `select` so acquire and release always use the same channel, even if `SetWorkers` is called mid-flight. `SaveSettings()` in `app.go` calls both `SetWorkers` and `SetSpeedLimit` so changes take effect immediately without restart.
- **Reconnection**: `manager.Connect()` disconnects an existing connection before reconnecting - no "already connected" error.
- **Multi-connection tabs**: `manager.ConnectNew()` adds a connection alongside existing ones. The frontend tracks open connections in the `connections` writable store (`[{id, name, host, protocol, port, user, remotePath}]`). Tabs appear in `App.svelte` between the topbar and dual-browser only when `$connections.length > 1`. `switchTab(id)` saves the current remotePath before switching. `closeTab(id)` cleans up and auto-activates the next tab. When the disconnect button is clicked with 2+ open connections, a confirmation overlay asks to close all. `MaxConnections` (1–5, default 3) is a setting that controls how many can be kept open; `SiteManager` checks this before offering the "keep and open new" option.
- **Duplicate connection guard**: `doKeepAndAdd()` in `SiteManager.svelte` checks whether any entry in `$connections` already has the same `host`, `port`, `protocol`, and `user` before calling `addConnection()`. If a duplicate is detected → calls `connectBySite()` instead (reconnect, no new tab). For `ask_password` mode the `promptIsAdd` flag is set to `false` so the password prompt routes to `connectBySiteWithPassword()` rather than `addConnection()`.
- **DefaultLocalDir**: `initLocalDir(startDir?)` in connection.js uses the setting on startup; `loadSettings()` returns the settings object so `App.svelte` can pass it immediately.
- **ListDir timeout**: `manager.ListDir` wraps the blocking client call in a goroutine with a `time.After` timeout; on timeout it forces disconnect and returns an error so the UI doesn't freeze.
- **Keepalive**: `manager.startKeepalive(entry)` is called after every successful connection. A goroutine ticks every 60 seconds: checks the connection is still in the manager's list (stops if not - normal disconnect), then calls `client.Keepalive()`. FTP sends NOOP via `c.conn.NoOp()` using `TryLock` (skips the tick if a control op holds the mutex). SFTP sends `ssh.SendRequest("keepalive@openssh.com", true, nil)`. On keepalive error: removes the entry from the manager, emits `connection:lost` event via the `onLost` callback. `Manager.SetOnConnectionLost(fn)` registers the callback; `app.go startup()` wires it to `runtime.EventsEmit(ctx, "connection:lost", {id, host})`.
- **Connection lost notification**: `App.svelte onMount()` subscribes to `connection:lost`. Handler calls `closeTab(id)` (clean state removal) and shows a red dismissible banner with the server hostname. Auto-dismisses after 10 seconds. i18n keys: `connectionLost`, `connectionLostDetail`.
- **Folder transfers**: `app.QueueUploadDir(localPath, remotePath)` and `app.QueueDownloadDir(remotePath, localPath)` handle recursive directory transfers. Each runs a goroutine that walks the tree (using `os.ReadDir` for local, `connMgr.ListDir` for remote), creates directories at the destination, and calls `queue.Add()` for each individual file. The frontend's `doQueueTransfer()` checks `entry.isDir` and routes to the Dir variants; drag-and-drop includes `isDir` in the payload and does the same.
- **Operation timeout** (`ConnectionTimeoutSec`, default 60s): controls TCP dial timeout on connect and the ListDir watchdog. It is NOT an idle inactivity timer - the keepalive goroutine handles idle disconnects independently. Label in Settings: "Délai d'opération (secondes)" / "Operation timeout (seconds)".
- **Focus trap in popups**: `use:trapFocus` (from `utils/focusTrap.js`) is applied to every modal/overlay container - `SiteManager` main modal, keep-or-replace overlay, password prompt overlay, `SettingsPanel`, disconnect-all box in `App.svelte`, both confirm boxes in `FileBrowser`, and the quick-connect keep-or-replace dialog in `ConnectionBar`. Tab/Shift+Tab cycle only within the active popup.
- **Duplicate site** (`SiteManager.svelte`): `duplicateSite()` clones the current `form` with `name + ' (copie)'` and calls `CreateSite`. Button rendered in `.form-actions` with `margin-left: auto` (pushed right), only visible when `selectedSite` is set (not on new-site creation). i18n key: `duplicateSite`.
- **SiteManager form right-click menus**: `ctxMenu = { x, y, field, selStart, selEnd, value, pasteOnly }` captures cursor state on `on:contextmenu`. All text inputs and textarea get cut/copy/paste; password input gets paste-only (`pasteOnly = true`). `field === 'port'` result is passed through `parseInt()` to keep `form.port` as a number. i18n keys: `cut`, `copy`, `paste`.
- **SiteManager form password eye toggle**: `showFormPwd` boolean (separate from `showPwd` for the ask-password overlay). Uses the `{#if}/{:else}` two-input pattern (WebKit-GTK pattern #3). Reuses `.pwd-input-wrap`, `.pwd-input`, `.eye-btn` CSS classes.
- **SiteManager action buttons centered**: `.view-actions` has `justify-content: center` and `.site-detail` / `.site-view` / `.site-view-header` have `align-items: center` (+ `text-align: center` on header) - entire detail panel (name, subtitle, note, buttons) is fully centered. Modal width is 780px. All three action button classes (`.btn-primary`, `.btn-secondary`, `.btn-danger-outline`) have `display: flex; align-items: center; gap: 6px` and `svg { width: 14px; height: 14px }` to avoid uncontrolled SVG sizing under WebKit-GTK.
- **SiteManager connection loading state**: `connecting` boolean in `SiteManager.svelte` is set to `true` before each async connect call and reset in `finally`. Covers all four paths: `connectToSite()` (direct), `doKeepAndAdd()` (keep + add), `doReplace()`, `confirmPasswordConnect()`. While `connecting` is true, the Connect button shows `.btn-spinner` (14px SVG arc, `@keyframes btn-spin` 0.75s linear) + `$t('connecting')` text and is `disabled`. The Keep/Replace buttons in the keep-or-replace overlay and the Connect/Cancel buttons in the password prompt overlay are also disabled.
- **build script renamed**: `build.sh` was renamed to `make.sh`. All references in README.md, CLAUDE.md, memo.txt updated accordingly.
- **Connect card shadow box** (`App.svelte`): `.connect-card.shadow-accent` applies a 3-layer neon box-shadow offset bottom-right using `--accent-glow` (rgba 45% opacity) and `--accent-subtle`. Controlled by `settings.connectCardShadow` (bool, default `false`). Toggle in Settings > Interface. `--accent-glow` is set by `applyAccentColor()` in `settings.js` alongside `--accent-subtle`.
- **Tree view** (`FileBrowser.svelte`): toggle button (left of refresh) switches each panel independently between list view and tree view. Tree state: `treeMode`, `treeNodes` (flat display list with depth/isDir/expanded/leaf), `treeLoaded`+`treeChildrenMap` (lazy-load cache), `treeSelected` (selected file path). `enterTreeMode()` loads `/` then calls `autoExpandToPath(path)` to pre-expand ancestors. `expandTreeNode(idx)` / `collapseTreeNode(idx)` insert/remove children inline in the flat array. Dir rows navigate on click; file rows select on single-click (accent highlight), transfer on double-click or arrow button. `treeMode` resets to false on disconnect (component destroy). Tree mutations use `treeRefreshDir(dirPath)` (clears cache + collapses/re-expands one node in-place), `treeRemoveEntries(entries)` (removes nodes + descendants directly), and `treeParentDir(path)` helper - so delete/paste/drag-drop/new-folder all update the tree without a full rebuild. Right-click on a tree row sets `selected=[ent]` and calls `handleFileContextMenu` (same menu as list view). Inline rename in tree: `.tree-name` shows `<input class="tree-rename-input">` when `renamingEntry?.path === node.path`; `doRename` mutates `treeNodes` paths directly instead of calling `onRefresh()`.
- **Quick connect button** (`ConnectionBar.svelte`): button to the left of the disconnect button, only visible when connected. Shows ↑ when idle; clicking enters quick connect mode - all inputs unlock and clear, host input gets focus, button becomes → (accent colored). Clicking → shows a keep-or-replace dialog (`quickConnectDialog = { cfg }`, `position: fixed; top: 48px`) with "Keep and open new" (if under `maxConnections`), "Replace current", and "Cancel". "Replace" calls `addConnectionAdHoc(cfg)` then `closeTab(oldId)` - safe replace that preserves the old connection until the new one succeeds. Clicking outside the connection bar exits quick connect mode and restores old field values. If host is empty when → is clicked, the host input border blinks red 3 times (`@keyframes host-error-blink`, 1.2s) via `class:host-error` - no text, no layout shift. Multi-connection case (2+ tabs): triggers the existing disconnect-all dialog instead of keep-or-replace.
- **Password security - system keyring**: passwords are stored in the OS keyring (gnome-keyring/kwallet on Linux via D-Bus, Windows Credential Manager/DPAPI on Windows). `sites.json` on disk **never contains passwords** - `sites.go save()` always strips them. `app.GetSites()` enriches sites with passwords from keyring before returning to the frontend. `app.CreateSite`/`UpdateSite` extract the password, clear it from the struct, then call `keyringMgr.Set()`. `app.DeleteSite` also calls `keyringMgr.Delete()`. On startup `migratePasswords()` migrates any plaintext passwords still in old `sites.json` to keyring then re-saves the file stripped. `needsKeyring(authType)` returns false for `anonymous`, `ask_password`, `interactive` - those auth types never store a password. If keyring is unavailable `GetKeyringStatus()` returns `"keyring_unavailable"` and `SiteManager.svelte` shows a yellow warning banner; passwords simply won't be saved (user should switch to `ask_password`).
- **Password security - encrypted export (.gfe)**: export dialog offers two choices - "without passwords" (plain JSON) or "with passwords" (encrypted `.gfe`). The `.gfe` binary format: `[4B magic "GFEX"][1B version][32B random salt][12B random nonce][ciphertext+GCM tag]`. Key derivation: `Argon2id(passphrase, salt, time=3, memory=64MB, threads=4)` → 32-byte AES-256 key. Encryption: AES-256-GCM (provides confidentiality + integrity). On import, `IsEncrypted()` checks magic bytes; if encrypted, frontend shows passphrase prompt before calling `DoImportSites(path, passphrase)`. Works cross-platform (Linux↔Windows) because the format is purely binary/Go stdlib - no OS-specific primitives.
- **Intra-panel clipboard** (`connection.js`): `clipboard` writable store `{ entries, operation: 'copy'|'cut', side }`. Shared between both FileBrowser instances. `pasteToFolder(destFolder)` in `FileBrowser.svelte` checks `clipboard.side === side` before pasting. Same-directory paste calls `uniqueCopyName(name, usedNames)` which generates `name (copie).ext` → `name (copie 1).ext` etc.; `usedNames` is pre-seeded from `entries` and updated per-item in the batch to avoid duplicates. Remote copy = `CopyRemote` (download to temp + re-upload, synchronous). Progressive refresh: 1s delay then every 4s during operation, final refresh on completion. Green/orange banner with auto-dismiss 4s.
- **Local trash** (`internal/fs/trash_linux.go` / `trash_windows.go`): `LocalDelete` calls `localfs.Trash()` not `localfs.Delete()`. Linux: XDG spec - move to `~/.local/share/Trash/files/` + `.trashinfo` sidecar, collision avoidance, cross-device fallback. Windows: `SHFileOperationW` via `syscall.NewLazyDLL("shell32.dll")` with `FOF_ALLOWUNDO` - no CGo, mingw compatible. Remote deletes are always permanent (no server-side trash in FTP/SFTP).
- **Release notes**: one `v{version}.md` file per release at project root (EN + FR, features + bugfixes + download table). `SettingsPanel.svelte` footer badge must be updated to match on each release.

## WebKit-GTK UI Patterns (Linux)

The Wails WebView on Linux uses WebKit-GTK. These patterns are broken and **must not be used**:

1. **Hidden checkbox toggles** (`<label><input type="checkbox" hidden>`) - checkboxes never fire click events when hidden this way. **Use `<button class="sw" class:on={val} on:click={() => toggle(key)}>` instead.** See `SettingsPanel.svelte` for reference.

2. **Native number input spinners** - unreliable/invisible. **Use custom `−`/`+` buttons with a `step(key, delta, min, max)` helper.** Hide native spinners with `-moz-appearance: textfield` and `-webkit-appearance: none`. See `SettingsPanel.svelte` and `ConnectionBar.svelte` (port stepper) for reference.

3. **Dynamic `type` on `<input bind:value>`** - Svelte 3 compile error: `'type' attribute cannot be dynamic if input uses two-way binding`. **Use two separate inputs in `{#if}`/`{:else}` blocks** - one `type="text"`, one `type="password"`, both bound to the same variable. See `SiteManager.svelte` password prompt for reference.

4. **Ctrl+Z (undo) in inputs** - WebKit-GTK does not fire native undo in Svelte-bound inputs. Fixed globally in `App.svelte` `handleKeydown`: intercept `Ctrl+Z` on `INPUT`/`TEXTAREA`, call `document.execCommand('undo')`, then dispatch a synthetic `input` event so Svelte re-syncs its variable.

5. **Right-click context menu in inputs** - WebKit-GTK disables the native context menu in the WebView. Implement a custom menu via `on:contextmenu`. For **cut/copy**: snapshot `selectionStart`/`selectionEnd`/`value` at right-click time (try-catch required for `type="number"`), then write selected text with `navigator.clipboard.writeText()`. For **paste**: `navigator.clipboard.readText()` inserted at the saved cursor position. Cut/Copy buttons only shown when `selStart !== selEnd`. See `SiteManager.svelte` `handleCtxMenu` / `ctxCut` / `ctxCopy` / `ctxPaste` for the full pattern; password prompt overlay uses the simpler paste-only `pasteMenu` / `doPaste`.

## FileBrowser Features

`FileBrowser.svelte` receives `side` ('local'|'remote'), `path`, `entries`, `selected`, `otherPath`, `otherEntries`, and action callbacks. `otherEntries` is the entry list of the opposite panel (passed from App.svelte as `$remoteEntries` / `$localEntries`), used for conflict detection.

- **".." entry**: always shown at the top; click/dblclick calls `onNavigateUp`; focused via `parentFocused` state when ArrowUp is pressed from first entry
- **Keyboard navigation**: ArrowDown/ArrowUp moves selection through entries; ArrowUp from first entry sets `parentFocused = true` (highlights ".." row); Enter on dir navigates in; Enter when `parentFocused` calls `onNavigateUp`; `$: if (path) parentFocused = false` resets on navigation
- **Editable path bar**: click the path display to enter edit mode; Enter navigates, Esc cancels; debounced autocomplete dropdown shows matching subdirs
- **Column sort**: click Name/Size/Date headers; dirs always listed first; second click reverses order
- **Multi-select**: Ctrl+click toggles, Shift+click range-selects, rubber-band (click-drag on empty area)
- **F2 rename**: panel div is `tabindex="-1"` and focused on row click; keydown handler triggers rename on F2
- **Delete key**: keydown handler calls `handleDelete(selected)` - deletes the full selection
- **Right-click context menu**: on a file → Rename / Transfer / Delete (deletes full selection if right-clicked item is in selection); on empty area → New Folder
- **Delete confirmation**: `confirmDeleteEntries` (array); popup shows filename (1 item) or "N éléments" (multiple); `doDeleteAll()` iterates and calls `onDelete` for each, single refresh at end
- **Drag & drop**: rows are `draggable`; drag data is `{ entries: [{path, name}], fromSide }` - if the dragged row is in the current selection, all selected entries are included. Drop iterates over `entries` array. Drag-drop from the OS file manager is not supported (WebKit-GTK does not expose OS drag sources to JS).
- **Conflict resolution**: `conflictState` is a two-mode state machine - `null` (no dialog), `{ mode:'choose', conflicts:[] }` (4-button dialog: Replace / Rename on host / Rename on server / Cancel), `{ mode:'rename', entry, remaining:[], inputVal:'', index, total }` (rename input step). `checkConflicts()` compares names against `otherEntries` (case-insensitive). Non-conflicting files in the same selection are queued immediately. For multi-file rename, the wizard advances one file at a time (`advanceRename`); a `1/N` counter is shown and the input re-mounts per file via `{#key conflictState.index}` to trigger `autofocus`. Drag-drop bypasses conflict detection.
- **Refresh animation**: `handleRefresh()` sets `refreshing = true`, awaits `Promise.all([onRefresh(), 500ms])` to guarantee a full spin, then resets. CSS `@keyframes spin-once` rotates the SVG 360° in 0.5s; `class:spinning={refreshing}` applies it. Multiple rapid clicks ignored while refreshing.
- **Tree view**: toggle button to the left of the refresh button switches the panel between list view and tree view (`treeMode` boolean, independent per panel, resets to `false` on disconnect). In tree mode the file-list is replaced by `.tree-list`. State: `treeNodes` (flat array `{path, name, depth, isDir, size, modTime, expanded, loading, leaf}`), `treeLoaded` Set + `treeChildrenMap` object (lazy-load cache keyed by dir path), `treeSelected` (path of selected file). `enterTreeMode()` resets all state, loads `/` via `fetchDirChildren('/')`, then calls `autoExpandToPath(path)` to pre-expand all ancestors of the current path. `expandTreeNode(idx)` inserts child nodes after the parent in the flat array and marks `leaf:true` if no children. `collapseTreeNode(idx)` removes all descendants (nodes with depth > parent depth). Dir rows: click → `onNavigate()`; toggle arrow → expand/collapse. File rows: single click → `treeSelected` (accent highlight + transfer button visible); double-click → `transferTreeFile()` which queues upload/download to `otherPath`. Transfer arrow button visible on hover (opacity 0→1). `fetchDirChildren` returns dirs first then files, both sorted alphabetically.

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
| `addConnectionAdHoc(cfg)` | connection.js | Calls `ConnectAdditional`; adds a direct (non-site) connection alongside existing ones - used by quick connect "Replace" to safely add then close old |
| `switchTab(id)` | connection.js | Saves current remotePath, calls `SwitchConnection`, refreshes remote panel for the new active connection |
| `closeTab(id)` | connection.js | Calls `CloseConnection`, removes from store; if last tab, clears all state; if was active, switches to last remaining |
| `connectBySite(id, config?)` | connection.js | Sets `connectionStatus` store correctly; optional `config` param populates `activeConnectionConfig` |
| `connectBySiteWithPassword(id, pwd, config?)` | connection.js | Like `connectBySite` but passes runtime password (for `ask_password` auth sites) |
| `activeConnectionConfig` | connection.js | Writable; set on every connect with `{ protocol, host, port, user }`; used by `ConnectionBar` to show real values when connected |
| `initLocalDir(startDir?)` | connection.js | Initializes local panel; pass `defaultLocalDir` from settings on startup |
| `loadSettings()` | settings.js | Returns the loaded settings object (in addition to updating the store) |
| `applyAccentColor(hex)` | settings.js | Sets `--accent`, `--accent-hover`, `--accent-subtle`, `--accent-glow` CSS vars; called on load and save |

## Go Backend Notes

- `queue.RemoveJob(id)` - removes a finished/cancelled/failed job; emits `transfer:removed` event
- `app.RemoveTransfer(id)` - JS-callable wrapper around `queue.RemoveJob`
- `app.ConnectWithPassword(id, password)` - connects to a saved site but overrides its stored password (for `ask_password` sites)
- `app.ConnectToSiteAdditional(siteID, overridePassword)` - adds a new connection alongside existing ones (multi-tab); calls `manager.ConnectNew()`
- `app.GetConnections()` - returns `[]ConnInfo` for all currently open connections
- `app.SwitchConnection(id)` - switches the active connection and calls `queue.SetExecutor` with the new client
- `app.CloseConnection(id)` - closes a specific connection and calls `queue.SetExecutor` with the updated active client
- `app.GetActiveConnectionID()` - returns the UUID of the currently active connection
- `manager.ConnectNew()` - adds a connection without removing existing ones; new connection becomes active
- `manager.CloseOne(id)` - closes specific connection; if it was active, switches to the most-recently-added remaining one
- `manager.SwitchTo(id)` - makes a connection active without reconnecting
- `app.ExportSitesPlain()` - file-dialog, exports sites as plain JSON without passwords
- `app.ExportSitesEncrypted(passphrase)` - file-dialog, exports sites as `.gfe` (Argon2id+AES-256-GCM encrypted); fetches passwords from keyring before encrypting
- `app.OpenImportDialog()` - opens file-dialog, reads first bytes, returns `ImportFileInfo{Path, NeedsPassphrase}`
- `app.DoImportSites(path, passphrase)` - imports from plain JSON or `.gfe`; stores passwords in keyring; returns count
- `app.GetKeyringStatus()` - returns `""` if keyring available, `"keyring_unavailable"` otherwise
- `app.shutdown(ctx)` - registered as `OnShutdown` in `main.go`; calls `connMgr.Disconnect()` for clean teardown on window close
- `manager.Connect()` - disconnects existing active client first; other connections remain open (enables reconnection from SiteManager)
- `app.ConnectAdditional(cfg)` - adds a new connection alongside existing ones (no site ID required - for ConnectionBar quick connect); calls `manager.ConnectNew()` and sets the queue executor; returns `ConnInfo`
- `Client` interface (`types.go`) - `Upload`, `Download` take `context.Context` as first arg; `Keepalive() error` also part of the interface; both `FTPClient` and `SFTPClient` implement all methods
- `FTPClient` - `c.mu` protects control-connection ops only (ListDir, MkDir, Delete, Rename, CurrentDir, Keepalive with TryLock); `Upload`/`Download` use per-transfer connections via `dial()`
- **FTP Download order**: `FileSize` MUST be called BEFORE `Retr` in `ftp.go`. Calling it after opens a command on the control connection mid-transfer, which violates FTP protocol and causes Synology (and others) to return 0 bytes.
- `manager.GetActiveHost()` - returns `cfg.Host` of the currently active connection; used by `QueueUpload`/`QueueDownload` to tag each job with `RemoteHost`
- `Job.RemoteHost string` - set at queue time with the active server hostname/IP; serialized as `remoteHost` in JSON events sent to the frontend
- `app.QueueUploadDir(localPath, remotePath)` / `app.QueueDownloadDir(remotePath, localPath)` - async recursive directory transfer; walks tree, creates dest dirs, queues one job per file; exported to frontend via `App.js`
- `queue.SetWorkers(n int)` - replaces the semaphore with a new buffered channel of capacity `n`; called by `SaveSettings()` alongside `SetSpeedLimit()`
- `manager.SetOnConnectionLost(fn func(id, host string))` - registers callback fired when a keepalive detects a dead connection; wired in `app.go startup()` to emit `connection:lost` Wails event
- `app.LocalCopy(srcPath, destPath)` / `localfs.Copy(src, dst)` - recursive local copy (files + dirs); used by intra-panel copy/paste in FileBrowser
- `app.RemoteCopy(srcPath, destPath)` - downloads to temp file then re-uploads; works for FTP and SFTP without server-side COPY command; synchronous so caller can await
- `app.RemoteCopyDir(srcPath, destPath)` - synchronous recursive remote copy (MkDir + recurse + CopyRemote per file)
- `app.LocalDelete(path)` - now calls `localfs.Trash(path)` instead of `localfs.Delete(path)`; sends to OS trash/recycle bin
- **Local trash** (`internal/fs/trash_linux.go` / `trash_windows.go` / `trash_other.go`): Linux = XDG Trash spec (`~/.local/share/Trash/files/` + `.trashinfo` sidecar, cross-device fallback via Copy+RemoveAll); Windows = `SHFileOperationW` via `syscall.NewLazyDLL("shell32.dll")` with `FOF_ALLOWUNDO` flag (no CGo, compatible with mingw cross-compile); other platforms = permanent delete fallback

## TransferQueue

- Speed is computed in `TransferQueue.svelte` from deltas of `bytesDone` between store updates (250 ms window minimum to avoid noise). Stored in `speeds` map (`id → bytes/sec`), displayed as `KB/s` or `MB/s` in accent color next to the progress label.
- **Transfer direction**: each job row shows a `.job-route` line (`"local → host"` for uploads, `"host → local"` for downloads) in all 3 tabs - uses `job.remoteHost` for the server name and `job.direction` for the arrow side.
- **Average speed (done tab only)**: computed in `avgSpeed(job)` as `job.size / (finishedAt - createdAt)` seconds; displayed next to the route with label `avgSuffix` (i18n: `"moy."` FR / `"avg."` EN).
- **Clear button (failed tab)**: the "Échoués" tab groups `status === 'failed'` AND `status === 'cancelled'` jobs. The "Vider" button calls `clearTransfers('failed')` then `clearTransfers('cancelled')` to remove both - calling only `'failed'` left cancelled jobs behind.
- `ColorPicker.svelte` stores last 8 applied colors in `localStorage` key `glideftp_color_history`; displayed as swatches above the footer; click to select.

## Text & Naming Conventions

- **Arch**: always write `Arch` - never `Arch/Manjaro` anywhere (README, release notes, docs, etc.).
- **Dashes**: always use `-` (hyphen). Never use `-` (em dash) in any generated text, release notes, or documentation.
