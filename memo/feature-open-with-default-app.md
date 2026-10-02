# Ouvrir un fichier avec l'application par défaut (option 1)

Objectif : pouvoir ouvrir n'importe quel fichier (texte, image, PDF, etc.) directement depuis GlideFTP avec l'application par défaut du système, comme la fonction "Afficher/Éditer" de FileZilla, et renvoyer automatiquement sur le serveur les fichiers distants modifiés.

Option retenue : **ouverture via l'application par défaut du système** (pas de visionneuse intégrée pour l'instant).

---

## 1. Principe général

1. **Fichier local** : ouverture directe à son emplacement, sans copie temporaire ni surveillance.
2. **Fichier distant** :
   1. Téléchargement dans un dossier temporaire dédié.
   2. Ouverture avec l'application par défaut.
   3. Surveillance des **modifications** du fichier temporaire.
   4. Si modifié : proposition de renvoi sur le serveur.

Tous les formats sont gérés (texte, images, PDF, vidéo, Office...) puisque c'est le système qui choisit l'application.

---

## 2. Téléchargement temporaire

- Dossier : `os.TempDir()/GlideFTP/open/<id-unique>/<nom-du-fichier>`
  - Un sous-dossier unique par ouverture pour conserver le nom d'origine (l'application affiche le bon nom, l'extension reste intacte) et éviter les collisions entre deux fichiers de même nom venant de dossiers/serveurs différents.
- Utilise `Client.Download(ctx, remotePath, localPath, progress)` qui existe déjà (FTP et SFTP).
  - En FTP, `Download()` ouvre déjà sa propre connexion dédiée via `dial()` - aucun conflit avec la connexion de contrôle.
- Ne **pas** passer par la file de transferts (`queue`) : appel direct et synchrone, avec un indicateur de chargement côté UI et un bouton Annuler (via `context.WithCancel`).

---

## 3. Ouverture avec l'application par défaut

Implémentation par OS, sur le même modèle que `internal/fs/trash_linux.go` / `trash_windows.go` / `trash_other.go` :

- `internal/fs/open_linux.go`
- `internal/fs/open_windows.go`
- `internal/fs/open_other.go` (fallback : renvoie une erreur "non supporté")

### Linux

- `exec.Command("xdg-open", path)`.
- **Piège AppImage** : l'AppImage injecte des variables d'environnement (`LD_LIBRARY_PATH`, `WEBKIT_EXEC_PATH`, `GDK_PIXBUF_MODULE_FILE`, `GDK_PIXBUF_MODULEDIR`, `GIO_MODULE_DIR`, `GST_PLUGIN_*`, `PYTHONPATH`, etc.). Si `xdg-open` en hérite, l'application externe peut charger les bibliothèques embarquées dans l'AppImage et planter. Il faut lancer la commande avec un environnement nettoyé de ces variables (au minimum quand `APPIMAGE` / `APPDIR` est défini).
- Lancer le processus sans attendre sa fin de façon bloquante pour l'UI (`cmd.Start()` puis `cmd.Wait()` dans une goroutine pour récupérer le code de retour sans laisser de processus zombie).

### Windows

- `ShellExecuteW` via `syscall.NewLazyDLL("shell32.dll")` (pas de CGo, compatible cross-compile mingw, comme `trash_windows.go`).
- Verbe `"open"`.

---

## 4. Aucune application associée au type de fichier

### Windows

- `ShellExecuteW` renvoie un code `<= 32` en cas d'erreur ; `SE_ERR_NOASSOC` (31) = aucune application associée.
- Dans ce cas : lancer `rundll32.exe shell32.dll,OpenAs_RunDLL <fichier>` qui affiche la fenêtre native **"Ouvrir avec..."** de Windows. Solution idéale.

### Linux

- `xdg-open` renvoie normalement un code non nul (3 = aucun outil trouvé, 4 = action échouée), **mais** selon l'environnement de bureau il peut renvoyer 0 et échouer silencieusement - pas fiable à 100 %.
- Pas de fenêtre "Ouvrir avec..." universelle (le portail `xdg-desktop-portal` avec `OpenURI` + option `ask` existe mais n'est pas présent partout).
- Comportement retenu en cas d'erreur détectée : popup `notify` "Aucune application associée à ce type de fichier" avec deux actions :
  - **Ouvrir le dossier** : ouvre le dossier temporaire dans le gestionnaire de fichiers (`xdg-open <dossier>`), qui propose toujours "Ouvrir avec".
  - **Choisir une application...** : sélection d'un exécutable via `OpenFileDialog`, lancé avec le fichier en argument. Optionnel : mémoriser ce choix par extension dans les settings (ex : `OpenWithOverrides map[string]string`).

---

## 5. Renvoi des modifications sur le serveur

### Pourquoi ne PAS attendre la fermeture de l'application

On ne peut pas détecter de façon fiable la fermeture de l'éditeur :

- `xdg-open` et `ShellExecute` rendent la main immédiatement (ils ne sont que des lanceurs).
- Beaucoup d'éditeurs sont **mono-instance** (VS Code, Notepad++, gedit, Kate...) : si l'éditeur est déjà ouvert, le fichier s'ajoute en onglet et le processus lancé se termine aussitôt. On croirait que l'utilisateur a fermé l'éditeur alors qu'il n'a encore rien fait.

C'est pour cette raison que FileZilla surveille la **modification du fichier**, pas la fermeture de l'application. On fait pareil.

### Surveillance des modifications

- Liste des fichiers ouverts côté Go (`openedFiles map[string]*OpenedFile`, protégée par un mutex) :
  - `TempPath` - chemin du fichier temporaire
  - `RemotePath` - chemin distant d'origine
  - `ConnID` - ID de la connexion d'origine (onglet)
  - `Host` - pour l'affichage
  - `LastModTime`, `LastSize` - état local au dernier téléchargement / renvoi
  - `RemoteModTime`, `RemoteSize` - état distant au moment du téléchargement (pour la détection de conflit)
  - `AutoUpload bool` - si l'utilisateur a choisi "Toujours pour ce fichier"
- Une goroutine vérifie toutes les **2 secondes** `ModTime` + `Size` de chaque fichier temporaire (`os.Stat`).
  - Vérification périodique plutôt que `fsnotify` : beaucoup d'éditeurs enregistrent en supprimant/recréant le fichier (écriture atomique par renommage), ce qui casse les surveillances inotify posées sur le fichier lui-même. Le polling sur le chemin est simple et robuste.
- **Anti-rebond** : un changement n'est pris en compte que si l'état est stable depuis 1 à 2 secondes (les éditeurs écrivent parfois en plusieurs fois).
- Changement détecté → événement Wails `openfile:modified` `{ id, name, host, remotePath }`.

### Popup côté frontend

- "**fichier.txt** a été modifié. Renvoyer sur le serveur **host** ?"
- Boutons : **Oui** / **Non** / **Toujours pour ce fichier**
  - Oui → `app.UploadOpenedFile(id)`
  - Non → on met à jour `LastModTime`/`LastSize` pour ne pas redemander tant qu'il n'y a pas de nouvelle modification
  - Toujours → `AutoUpload = true`, les modifications suivantes sont renvoyées sans demander (notification discrète de succès)
- Pas besoin de distinguer les fichiers "modifiables" (txt, etc.) : tout est surveillé. Une image retouchée dans GIMP ou un PDF annoté se renvoie de la même façon. Un fichier jamais modifié ne déclenche jamais la popup.

### Upload

- Upload sur la **connexion d'origine** (`ConnID`), pas forcément l'onglet actif.
  - Si la connexion a été fermée entre-temps → erreur claire ("La connexion à host est fermée, impossible de renvoyer le fichier").
- **Détection de conflit** (recommandé) : avant l'upload, `ListDir` du dossier parent distant et comparaison de `ModTime`/`Size` du fichier avec l'état mémorisé au téléchargement. Si différent → avertissement "Le fichier a été modifié sur le serveur depuis son ouverture. Écraser quand même ?".
- Après upload réussi : mise à jour de `RemoteModTime`/`RemoteSize` et rafraîchissement du panneau distant si le dossier affiché est concerné.

---

## 6. Nettoyage

- À la fermeture de GlideFTP (`app.shutdown`) : suppression de `os.TempDir()/GlideFTP/open/`.
- S'il reste des fichiers modifiés non renvoyés : avertir avant de quitter (via `OnBeforeClose` de Wails) - "N fichiers modifiés n'ont pas été renvoyés sur le serveur. Quitter quand même ?".
- Au démarrage : supprimer d'éventuels restes d'une session précédente (crash).
- Fermeture d'un onglet de connexion : arrêter la surveillance des fichiers liés à cette connexion (en prévenant si des modifications sont en attente).

---

## 7. Fichiers à créer / modifier

### Backend Go

| Fichier | Changement |
|---|---|
| `internal/fs/open_linux.go` | `OpenWithDefault(path) error` via `xdg-open` + environnement nettoyé (AppImage) ; `OpenWithApp(appPath, filePath) error` |
| `internal/fs/open_windows.go` | `OpenWithDefault(path) error` via `ShellExecuteW` ; fallback `OpenAs_RunDLL` si `SE_ERR_NOASSOC` |
| `internal/fs/open_other.go` | Fallback "non supporté" |
| `internal/openfile/watcher.go` (nouveau) | Liste des fichiers ouverts, goroutine de polling, anti-rebond, callback de modification |
| `app.go` | `OpenLocalFile(path)`, `OpenRemoteFile(remotePath)`, `CancelOpenRemoteFile()`, `UploadOpenedFile(id)`, `IgnoreOpenedFileChange(id)`, `SetOpenedFileAutoUpload(id, bool)`, `ChooseAppAndOpen(path)`, `OpenTempFolder(id)` ; câblage de l'événement `openfile:modified` dans `startup()` ; nettoyage dans `shutdown()` |
| `main.go` | `OnBeforeClose` pour l'avertissement de modifications non renvoyées |
| `internal/connection/manager.go` | Accès à un client par ID de connexion (pour uploader sur la connexion d'origine) si pas déjà exposé |
| `internal/settings/settings.go` | (optionnel) `OpenWithOverrides map[string]string` |

### Frontend

| Fichier | Changement |
|---|---|
| `components/FileBrowser.svelte` | Entrée "Ouvrir" dans le menu contextuel des fichiers (liste + arbre) ; éventuellement double-clic sur un fichier (à décider : aujourd'hui le double-clic en vue arbre = transfert) ; indicateur de chargement pendant le téléchargement |
| `components/OpenedFileModal.svelte` (nouveau) | Popup "fichier modifié, renvoyer ?" (Oui / Non / Toujours), avec `use:trapFocus` |
| `App.svelte` | Abonnement à `openfile:modified`, montage de la popup |
| `i18n/en.js` + `i18n/fr.js` | Clés : `openFile`, `openingFile`, `noDefaultApp`, `openFolder`, `chooseApp`, `fileModifiedTitle`, `fileModifiedUpload`, `uploadAlways`, `remoteFileChanged`, `connectionClosedUpload`, `unsavedOpenedFiles`... |

---

## 8. Ordre de réalisation proposé

1. `OpenWithDefault` Linux + Windows (avec nettoyage d'environnement AppImage) + `OpenLocalFile` + entrée "Ouvrir" dans le menu contextuel pour les fichiers locaux.
2. `OpenRemoteFile` : téléchargement temporaire + ouverture + annulation.
3. Gestion "aucune application associée" (Windows `OpenAs_RunDLL`, Linux popup + ouvrir le dossier / choisir une application).
4. Surveillance des modifications + popup de renvoi + upload sur la connexion d'origine.
5. Détection de conflit côté serveur.
6. Nettoyage (fermeture app, fermeture onglet, démarrage) + avertissement de modifications non renvoyées.

---

## 9. Points à tester

- Linux binaire, AppImage Arch, AppImage Debian, .deb, .rpm (surtout le nettoyage d'environnement en AppImage).
- Windows (ShellExecute + fallback "Ouvrir avec").
- Éditeur mono-instance déjà ouvert (VS Code, Kate, Notepad++) : la modification doit bien être détectée.
- Éditeur à écriture atomique (vim, gedit) : la modification doit bien être détectée.
- Type de fichier sans application associée.
- Fichier distant modifié par quelqu'un d'autre pendant l'édition (conflit).
- Onglet de connexion fermé avant le renvoi.
- Fermeture de GlideFTP avec des modifications non renvoyées.
- FTP et SFTP.
