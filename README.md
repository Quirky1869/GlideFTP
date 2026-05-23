# GlideFTP

![glide-ftp](./_images/glide-ftp.png)  

---

## 🇬🇧 English

**GlideFTP** is a free, open-source desktop FTP/SFTP client built with [Go](https://go.dev/) and [Wails v2](https://wails.io/), featuring a modern [Svelte](https://svelte.dev/) interface. It is designed to be fast, lightweight, and compatible with both **Windows** and **Linux**.

### Features

- **FTP & SFTP support** - connect to any FTP or SFTP server
- **SSH key authentication** - supports OpenSSH key files, PuTTY `.ppk` format (v2/v3, RSA & Ed25519), interactive auth, and SSH agent
- **Dark & Light themes** - dark by default, switchable in settings
- **Custom accent color** - full RGB/HEX color picker to personalize the interface
- **English & French interface** - English by default, switchable in settings
- **Dual-panel file browser** - local files on the left, remote files on the right
- **Transfer queue** - with 3 tabs: pending, failed, and successful transfers; cancel in-progress transfers
- **Multi-file operations** - select multiple files with Ctrl+click, Shift+click, or rubber-band drag; transfer or delete the whole selection at once
- **Multi-connection tabs** - open several servers simultaneously; browser-style tabs appear between the toolbar and file panels; configurable limit (1–5) in settings
- **Site manager** - save, edit and quickly reconnect to your favorite servers; add notes to each site; export/import sites as JSON
- **Ask-password auth** - password is never saved; prompted at connect time
- **SFTP auto-coupling** - selecting SFTP automatically sets authentication to Interactive (or SSH Key) and vice versa
- **Path autocomplete** - dropdown suggestions while typing in the path bar
- **Full settings panel** - passive mode, timeout, concurrent transfers, speed limit, hidden files, and more
- **Encryption support** - None, TLS (implicit), FTPES (explicit)

### Screenshots

|Main|Saved sites|
|----|-----|
| ![](./_images/main-en-d.png) | ![](./_images/saved-sites-en-d.png) |

|SFTP|Sites|
|----|-----------|
| ![](./_images/sftp-fr-w.png) | ![](./_images/sites-fr-d.png) |

|Connected|Settings|
|---------|--------|
| ![](./_images/connected-fr-w.png) | ![](./_images/settings-en-d.png) |

### Download pre-built binaries

The latest release provides three ready-to-run files:

| File | Platform | Notes |
|---|---|---|
| `GlideFTP-Windows-vX.Y.Z.tar.gz` / `.tar` | Windows | Requires WebView2 (built into Windows 10/11) |
| `GlideFTP-Linux-vX.Y.Z.tar.gz` / `.tar` | Linux binary | Requires `libwebkit2gtk-4.1` — see below |
| `GlideFTP-x86_64.AppImage` | Linux AppImage | **Self-contained — no dependency to install** |

#### Linux — pre-built binary

The raw Linux binary dynamically links against `libwebkit2gtk-4.1`. If it is not installed on your system, the binary will refuse to start with a *"cannot open shared object file"* error. Install it first:

```bash
# Ubuntu / Debian (22.04+)
sudo apt install libwebkit2gtk-4.1-0

# Fedora
sudo dnf install webkit2gtk4.1

# Arch Linux
sudo pacman -S webkit2gtk-4.1
```

Then make it executable and run it:

```bash
chmod +x GlideFTP
./GlideFTP
```

#### Linux — AppImage (recommended)

The AppImage bundles all dependencies (including `libwebkit2gtk-4.1` and the full GTK3 stack) inside a single file. No system library needs to be installed — just download, make executable, and run:

```bash
chmod +x GlideFTP-x86_64.AppImage
./GlideFTP-x86_64.AppImage
```

### Build from source

#### Prerequisites

- [Go](https://go.dev/dl/) 1.21+
- [Node.js](https://nodejs.org/) 18+
- [Wails v2](https://wails.io/docs/gettingstarted/installation)

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
export PATH="$PATH:$(go env GOPATH)/bin"
```

On **Linux**, the WebKit2GTK **development** package is required to compile:

```bash
# Arch Linux
sudo pacman -S webkit2gtk-4.1

# Ubuntu / Debian (22.04+)
sudo apt install libwebkit2gtk-4.1-dev

# Fedora
sudo dnf install webkit2gtk4.1-devel
```

#### Build

```bash
git clone https://github.com/Quirky1869/GlideFTP.git
cd GlideFTP
```

**Using the build script (recommended):**

```bash
./build.sh            # Linux binary + Windows exe + AppImage
./build.sh linux      # Linux binary only  → build/bin/linux/GlideFTP
./build.sh windows    # Windows exe only   → build/bin/windows/GlideFTP.exe
./build.sh appimage   # AppImage only      → build/bin/linux/GlideFTP-x86_64.AppImage
```

> The `appimage` target requires `curl` or `wget` (to download `linuxdeploy` on first run — cached in `tools/`).

**Manual commands:**

```bash
# Linux (requires webkit2gtk-4.1)
wails build -tags webkit2_41

# Windows cross-compile from Linux (requires mingw-w64-gcc)
# Install on Arch : sudo pacman -S mingw-w64-gcc
CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows wails build -platform windows/amd64
```

> **Note:** Windows uses WebView2 (built into Windows 10/11), so the `-tags webkit2_41` flag is not needed for Windows builds.

#### Run

```bash
# Linux
./build/bin/linux/GlideFTP

# Windows
build\bin\windows\GlideFTP.exe
```

### Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go - FTP (`jlaffaye/ftp`), SFTP (`pkg/sftp`, `x/crypto/ssh`) |
| UI Framework | Wails v2 |
| Frontend | Svelte + Vite |
| Config storage | JSON files in OS user config directory |

---

## 🇫🇷 Français

**GlideFTP** est un client FTP/SFTP de bureau gratuit et open-source, développé avec [Go](https://go.dev/) et [Wails v2](https://wails.io/), doté d'une interface moderne en [Svelte](https://svelte.dev/). Il est conçu pour être rapide, léger, et compatible avec **Windows** et **Linux**.  

### Fonctionnalités

- **Support FTP & SFTP** - connexion à n'importe quel serveur FTP ou SFTP
- **Authentification par clé SSH** - fichiers OpenSSH, format PuTTY `.ppk` (v2/v3, RSA & Ed25519), auth interactive, et SSH agent
- **Thèmes sombre & clair** - sombre par défaut, modifiable dans les paramètres
- **Couleur d'accentuation personnalisable** - sélecteur de couleur RGB/HEX pour personnaliser l'interface
- **Interface en anglais & français** - anglais par défaut, modifiable dans les paramètres
- **Explorateur double panneau** - fichiers locaux à gauche, fichiers distants à droite
- **File de transfert** - avec 3 onglets : en attente, échoués et réussis ; annulation des transferts en cours
- **Opérations multi-fichiers** - sélection multiple avec Ctrl+clic, Shift+clic ou sélection à la souris ; transfert ou suppression de toute la sélection en une fois
- **Onglets multi-connexion** - ouvrez plusieurs serveurs simultanément ; des onglets style navigateur apparaissent entre la barre d'outils et les panneaux de fichiers ; limite configurable (1 à 5) dans les paramètres
- **Gestionnaire de sites** - enregistrez, modifiez et reconnectez-vous rapidement à vos serveurs favoris ; ajoutez des notes à chaque site ; exportez/importez les sites en JSON
- **Auth demande de mot de passe** - le mot de passe n'est jamais enregistré ; saisi au moment de la connexion
- **Couplage automatique SFTP** - sélectionner SFTP active automatiquement l'authentification Interactive (ou Clé SSH) et inversement
- **Autocomplétion de chemin** - suggestions dans la barre de chemin lors de la saisie
- **Panneau de paramètres complet** - mode passif, délai de connexion, transferts simultanés, limite de vitesse, fichiers cachés, et plus encore
- **Support du chiffrement** - Aucun, TLS (implicite), FTPES (explicite)

### Screenshots

|Main|Sites Sauvegardés|
|:----:|:-----:|
| ![](./_images/main-en-d.png) | ![](./_images/saved-sites-en-d.png) |

|SFTP|Sites|
|:----:|:-----------:|
| ![](./_images/sftp-fr-w.png) | ![](./_images/sites-fr-d.png) |

|Connecté|Paramètres|
|:---------:|:--------:|
| ![](./_images/connected-fr-w.png) | ![](./_images/settings-en-d.png) |

### Télécharger les binaires pré-compilés

La dernière release propose trois fichiers prêts à l'emploi :

| Fichier | Plateforme | Notes |
|---|---|---|
| `GlideFTP-Windows-vX.Y.Z.tar.gz` / `.tar` | Windows | Nécessite WebView2 (intégré à Windows 10/11) |
| `GlideFTP-Linux-vX.Y.Z.tar.gz` / `.tar` | Binaire Linux | Nécessite `libwebkit2gtk-4.1` — voir ci-dessous |
| `GlideFTP-x86_64.AppImage` | Linux AppImage | **Autonome — aucune dépendance à installer** |

#### Linux — binaire pré-compilé

Le binaire Linux est lié dynamiquement à `libwebkit2gtk-4.1`. Si cette bibliothèque n'est pas installée sur votre système, le binaire refusera de démarrer avec une erreur *"cannot open shared object file"*. Installez-la d'abord :

```bash
# Ubuntu / Debian (22.04+)
sudo apt install libwebkit2gtk-4.1-0

# Fedora
sudo dnf install webkit2gtk4.1

# Arch Linux
sudo pacman -S webkit2gtk-4.1
```

Rendez ensuite le binaire exécutable et lancez-le :

```bash
chmod +x GlideFTP
./GlideFTP
```

#### Linux — AppImage (recommandé)

L'AppImage embarque toutes les dépendances (dont `libwebkit2gtk-4.1` et la pile GTK3 complète) dans un seul fichier. Aucune bibliothèque système à installer — téléchargez, rendez exécutable, et lancez :

```bash
chmod +x GlideFTP-x86_64.AppImage
./GlideFTP-x86_64.AppImage
```

### Compiler depuis les sources

#### Prérequis

- [Go](https://go.dev/dl/) 1.21+
- [Node.js](https://nodejs.org/) 18+
- [Wails v2](https://wails.io/docs/gettingstarted/installation)

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
export PATH="$PATH:$(go env GOPATH)/bin"
```

Sur **Linux**, le paquet de **développement** WebKit2GTK est nécessaire pour compiler :

```bash
# Arch Linux
sudo pacman -S webkit2gtk-4.1

# Ubuntu / Debian (22.04+)
sudo apt install libwebkit2gtk-4.1-dev

# Fedora
sudo dnf install webkit2gtk4.1-devel
```

#### Compiler

```bash
git clone https://github.com/Quirky1869/GlideFTP.git
cd GlideFTP
```

**Via le script de build (recommandé) :**  

```bash
./build.sh            # binaire Linux + exe Windows + AppImage
./build.sh linux      # Linux seulement     → build/bin/linux/GlideFTP
./build.sh windows    # Windows seulement   → build/bin/windows/GlideFTP.exe
./build.sh appimage   # AppImage seulement  → build/bin/linux/GlideFTP-x86_64.AppImage
```

> La cible `appimage` nécessite `curl` ou `wget` (pour télécharger `linuxdeploy` à la première exécution — mis en cache dans `tools/`).

**Commandes manuelles :**  

```bash
# Linux (nécessite webkit2gtk-4.1)
wails build -tags webkit2_41

# Cross-compilation Windows depuis Linux (nécessite mingw-w64-gcc)
# Installation sur Arch : sudo pacman -S mingw-w64-gcc
CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows wails build -platform windows/amd64
```

> **Note :** Windows utilise WebView2 (intégré à Windows 10/11), le flag `-tags webkit2_41` n'est donc pas nécessaire pour les builds Windows.

#### Lancer

```bash
# Linux
./build/bin/linux/GlideFTP

# Windows
build\bin\windows\GlideFTP.exe
```

### Stack technique

| Couche | Technologie |
|---|---|
| Backend | Go - FTP (`jlaffaye/ftp`), SFTP (`pkg/sftp`, `x/crypto/ssh`) |
| Framework UI | Wails v2 |
| Frontend | Svelte + Vite |
| Stockage config | Fichiers JSON dans le répertoire de config utilisateur |

---

Developed with ❤️ by [Quirky](https://github.com/Quirky1869)
