# GlideFTP

![glide-ftp](./_images/glide-ftp.png)  

---

## 🇬🇧 English

**GlideFTP** is a free, open-source desktop FTP/SFTP client built with [Go](https://go.dev/) and [Wails v2](https://wails.io/), featuring a modern [Svelte](https://svelte.dev/) interface. It is designed to be fast, lightweight, and compatible with both **Windows** and **Linux**.

### Features

- **FTP & SFTP support** — connect to any FTP or SFTP server
- **SSH key authentication** — supports key files, interactive auth, and SSH agent
- **Dark & Light themes** — dark by default, switchable in settings
- **Custom accent color** — full RGB/HEX color picker to personalize the interface
- **English & French interface** — English by default, switchable in settings
- **Dual-panel file browser** — local files on the left, remote files on the right
- **Transfer queue** — with 3 tabs: pending, failed, and successful transfers; cancel in-progress transfers
- **Multi-file operations** — select multiple files with Ctrl+click, Shift+click, or rubber-band drag; transfer or delete the whole selection at once
- **Site manager** — save, edit and quickly reconnect to your favorite servers; add notes to each site; export/import sites as JSON
- **Ask-password auth** — password is never saved; prompted at connect time
- **SFTP auto-coupling** — selecting SFTP automatically sets authentication to Interactive and vice versa
- **Path autocomplete** — dropdown suggestions while typing in the path bar
- **Full settings panel** — passive mode, timeout, concurrent transfers, speed limit, hidden files, and more
- **Encryption support** — None, TLS (implicit), FTPES (explicit)

### Screenshots

|Main|Sites|
|----|-----|
| ![](./_images/main-en-d.png) | ![](./_images/sites-fr-d.png) |

|SFTP|Saved sites|
|----|-----------|
| ![](./_images/sftp-fr-w.png) | ![](./_images/saved-sites-en-d.png) |

|Connected|Settings|
|---------|--------|
| ![](./_images/connected-fr-w.png) | ![](./_images/settings-en-d.png) |

### Installation

#### Prerequisites

- [Go](https://go.dev/dl/) 1.21+
- [Node.js](https://nodejs.org/) 18+
- [Wails v2](https://wails.io/docs/gettingstarted/installation)

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
export PATH="$PATH:$(go env GOPATH)/bin"
```

On **Linux**, the following system dependency is required:

```bash
# Arch / Manjaro
sudo pacman -S webkit2gtk-4.1

# Ubuntu / Debian
sudo apt install libwebkit2gtk-4.1-dev
```

#### Build from source

```bash
git clone https://github.com/Quirky1869/GlideFTP.git
cd GlideFTP

# Linux
wails build -tags webkit2_41

# Windows
wails build
```

The binary will be generated in `build/bin/`.

#### Run

```bash
./build/bin/GlideFTP
```

### Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go — FTP (`jlaffaye/ftp`), SFTP (`pkg/sftp`, `x/crypto/ssh`) |
| UI Framework | Wails v2 |
| Frontend | Svelte + Vite |
| Config storage | JSON files in OS user config directory |

---

## 🇫🇷 Français

**GlideFTP** est un client FTP/SFTP de bureau gratuit et open-source, développé avec [Go](https://go.dev/) et [Wails v2](https://wails.io/), doté d'une interface moderne en [Svelte](https://svelte.dev/). Il est conçu pour être rapide, léger, et compatible avec **Windows** et **Linux**.  

### Fonctionnalités

- **Support FTP & SFTP** — connexion à n'importe quel serveur FTP ou SFTP
- **Authentification par clé SSH** — fichiers de clé, auth interactive, et SSH agent
- **Thèmes sombre & clair** — sombre par défaut, modifiable dans les paramètres
- **Couleur d'accentuation personnalisable** — sélecteur de couleur RGB/HEX pour personnaliser l'interface
- **Interface en anglais & français** — anglais par défaut, modifiable dans les paramètres
- **Explorateur double panneau** — fichiers locaux à gauche, fichiers distants à droite
- **File de transfert** — avec 3 onglets : en attente, échoués et réussis ; annulation des transferts en cours
- **Opérations multi-fichiers** — sélection multiple avec Ctrl+clic, Shift+clic ou sélection à la souris ; transfert ou suppression de toute la sélection en une fois
- **Gestionnaire de sites** — enregistrez, modifiez et reconnectez-vous rapidement à vos serveurs favoris ; ajoutez des notes à chaque site ; exportez/importez les sites en JSON
- **Auth demande de mot de passe** — le mot de passe n'est jamais enregistré ; saisi au moment de la connexion
- **Couplage automatique SFTP** — sélectionner SFTP active automatiquement l'authentification Interactive et inversement
- **Autocomplétion de chemin** — suggestions dans la barre de chemin lors de la saisie
- **Panneau de paramètres complet** — mode passif, délai de connexion, transferts simultanés, limite de vitesse, fichiers cachés, et plus encore
- **Support du chiffrement** — Aucun, TLS (implicite), FTPES (explicite)

### Screenshots

|Main|Sites|
|:----:|:-----:|
| ![](./_images/main-en-d.png) | ![](./_images/sites-fr-d.png) |

|SFTP|Sites Sauvegardés|
|:----:|:-----------:|
| ![](./_images/sftp-fr-w.png) | ![](./_images/saved-sites-en-d.png) |

|Connecté|Paramètres|
|:---------:|:--------:|
| ![](./_images/connected-fr-w.png) | ![](./_images/settings-en-d.png) |

### Installation

#### Prérequis

- [Go](https://go.dev/dl/) 1.21+
- [Node.js](https://nodejs.org/) 18+
- [Wails v2](https://wails.io/docs/gettingstarted/installation)

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
export PATH="$PATH:$(go env GOPATH)/bin"
```

Sur **Linux**, la dépendance système suivante est nécessaire :

```bash
# Arch / Manjaro
sudo pacman -S webkit2gtk-4.1

# Ubuntu / Debian
sudo apt install libwebkit2gtk-4.1-dev
```

#### Compiler depuis les sources

```bash
git clone https://github.com/Quirky1869/GlideFTP.git
cd GlideFTP

# Linux
wails build -tags webkit2_41

# Windows
wails build
```

Le binaire sera généré dans `build/bin/`.  

#### Lancer

```bash
./build/bin/GlideFTP
```

### Stack technique

| Couche | Technologie |
|---|---|
| Backend | Go — FTP (`jlaffaye/ftp`), SFTP (`pkg/sftp`, `x/crypto/ssh`) |
| Framework UI | Wails v2 |
| Frontend | Svelte + Vite |
| Stockage config | Fichiers JSON dans le répertoire de config utilisateur |

---

Developed with ❤️ by [Quirky](https://github.com/Quirky1869)
