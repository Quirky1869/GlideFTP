# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GlideFTP is a desktop FTP/SFTP client built with Go + Wails v2 + Svelte. Fully implemented. Target platforms: Windows and Linux.

Design spec (French) in `prompt-glideftp`. UI reference sketch in `_images/exemple.png`.

## Build & Run

```bash
# Linux — system has webkit2gtk-4.1 (not 4.0), the tag is mandatory
wails build -tags webkit2_41        # → build/bin/GlideFTP
wails dev   -tags webkit2_41        # dev mode with hot reload

# Frontend only
cd frontend && npm install && npm run build

# Run the compiled binary
./build/bin/GlideFTP
```

> **Note:** `wails` must be on PATH — install with `go install github.com/wailsapp/wails/v2/cmd/wails@latest` then `export PATH="$PATH:$(go env GOPATH)/bin"`.

## Architecture

```
GlideFTP/
├── main.go                        # Wails entry point (1280×800)
├── app.go                         # All Go→JS bindings (the only Wails-bound struct)
├── internal/
│   ├── connection/
│   │   ├── types.go               # Shared types: Config, RemoteFileEntry, Client interface
│   │   ├── manager.go             # Thread-safe connection manager (Connect/Disconnect/ListDir…)
│   │   ├── ftp.go                 # FTP client (github.com/jlaffaye/ftp)
│   │   └── sftp.go                # SFTP client (github.com/pkg/sftp + golang.org/x/crypto/ssh)
│   ├── transfer/
│   │   └── queue.go               # Worker-pool transfer queue; emits Wails events for progress
│   ├── sites/
│   │   └── sites.go               # Saved sites — persisted to ~/.config/GlideFTP/sites.json
│   ├── settings/
│   │   └── settings.go            # App settings — persisted to ~/.config/GlideFTP/settings.json
│   └── fs/
│       └── local.go               # Local filesystem helpers (ListDir, MkDir, Delete, Rename)
└── frontend/src/
    ├── App.svelte                  # Root: disconnected (centered form) vs connected (dual panel)
    ├── i18n/{en,fr,index}.js       # EN/FR i18n via Svelte derived store
    ├── stores/
    │   ├── settings.js             # Loads/saves settings, applies theme to <html data-theme>
    │   ├── connection.js           # Connection state, local+remote path/entries stores
    │   └── transfers.js            # Transfer list store; subscribes to Wails events
    └── components/
        ├── ConnectionBar.svelte    # Host/user/pass/port/protocol inputs + connect button
        ├── FileBrowser.svelte      # Single panel (local or remote): nav, rename, delete, transfer
        ├── TransferQueue.svelte    # Bottom panel, 3 tabs: pending/failed/done
        ├── SettingsPanel.svelte    # Sliding panel (75% width from right)
        └── SiteManager.svelte      # Centered modal: create/edit/delete/connect saved sites
```

## Key Design Decisions

- **One `App` struct** in `app.go` is the single Wails binding — all methods on it are exposed to JS automatically.
- **Transfer progress** uses `runtime.EventsEmit` from Go → frontend subscribes with `EventsOn('transfer:progress', ...)`.
- **Theme** is applied via `document.documentElement.setAttribute('data-theme', 'dark'|'light')` — CSS vars defined in `style.css`.
- **i18n** is a Svelte `derived` store — `$t('key')` reactively switches language with no page reload.
- **Config files** are stored in the OS user config dir (`os.UserConfigDir()`): cross-platform without hardcoding paths.
- **SFTP auth** supports password, SSH key file (with optional passphrase), interactive keyboard, and SSH agent (`SSH_AUTH_SOCK`).
- **FTP passive mode** is the default (configurable in settings).
