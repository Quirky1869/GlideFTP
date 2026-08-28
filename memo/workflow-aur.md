# Workflow de publication - GitHub Release + AUR (glideftp-bin)

## A. Mise en place initiale (une seule fois)

### 1. Générer une clé SSH dédiée à l'AUR
```bash
ssh-keygen -t ed25519 -C "aur-glideftp" -f ~/.ssh/aur_glideftp -N ""
```
→ crée `~/.ssh/aur_glideftp` (privée) et `~/.ssh/aur_glideftp.pub` (publique).

### 2. Dire à SSH d'utiliser cette clé pour l'AUR
```bash
cat >> ~/.ssh/config << 'EOF'

Host aur.archlinux.org
    IdentityFile ~/.ssh/aur_glideftp
    User aur
EOF
chmod 600 ~/.ssh/config
```

### 3. Ajouter la clé publique sur le compte AUR
```bash
cat ~/.ssh/aur_glideftp.pub
```
→ copier le résultat, aller sur https://aur.archlinux.org/account/ → **My Account** → champ **SSH Public Key** → coller → **Update Account**.

### 4. Vérifier que ça fonctionne
```bash
ssh aur@aur.archlinux.org
```
→ un message de bienvenue Arch Linux s'affiche (normal : pas de shell interactif, l'AUR n'en fournit pas).

### 5. Cloner le dépôt du paquet (emplacement permanent, pas `/tmp`)
```bash
git clone ssh://aur@aur.archlinux.org/glideftp-bin.git /home/$USER/les-git-clones/AUR/glideftp-bin
```
→ premier `git push` dans ce dossier = création réelle du paquet sur l'AUR.
Ce dépôt ne contient **que** `PKGBUILD` et `.SRCINFO` - jamais le binaire, le `.desktop` ou l'icône (déjà dans l'archive `.tar.gz` téléchargée depuis GitHub par le `PKGBUILD` lui-même).

---

## B. À chaque nouvelle version (ex: 1.7.7)

### 1. Mettre à jour les fichiers de suivi du projet
- `v1.7.7.md` (notes de version EN/FR, format des releases précédentes)
- `issues-to-github.txt` / `prompt-glideftp` si des issues ont été corrigées
- badge de version dans `SettingsPanel.svelte` (footer)

Commit + push sur `main` (commandes à taper toi-même) :
```bash
git add -A
git commit -m "v1.7.7 release notes"
git push origin main
```

### 2. Tagger et pousser le tag → déclenche la CI GitHub Actions
```bash
git tag v1.7.7
git push origin v1.7.7
```
→ `release.yml` se lance : build Linux + Windows + AppImage Debian + `.deb` + `.rpm`, crée les archives, crée une **draft release** intitulée `V1.7.7` avec le texte de `v1.7.7.md` déjà en description.

### 3. Suivre la CI
Onglet **Actions** du repo → attendre que le run passe au vert.
(Si besoin de relancer manuellement : Actions → release.yml → **Run workflow** → taper `1.7.7`.)

### 4. Builder l'AppImage Arch en local (jamais construite par la CI)
```bash
cd /home/$USER/les-git-clones/GlideFTP
./make.sh appimage-arch 1.7.7
./create-archive.sh -p appimage-arch 1.7.7
```
→ produit `GlideFTP-Linux-Arch-AppImage-v1.7.7.tar.gz` à la racine.

### 5. Compléter la draft release sur GitHub
- Onglet **Releases** → ouvrir la draft `V1.7.7`
- Glisser-déposer `GlideFTP-Linux-Arch-AppImage-v1.7.7.tar.gz`
- Vérifier que le texte correspond bien à `v1.7.7.md` et que le titre est `V1.7.7`
- Cliquer **Publish release** (indispensable : tant que c'est en draft, les liens de téléchargement renvoient 404, l'AUR ne pourra rien récupérer)

### 6. Récupérer le hash réel du fichier publié
Ne pas se fier au sha256 calculé en local par `create-archive.sh` pendant les tests - le binaire buildé par la CI n'est pas forcément identique bit à bit à un build local. Toujours vérifier sur le fichier **réellement en ligne** :
```bash
curl -L -o /tmp/glideftp-1.7.7.tar.gz \
  https://github.com/Quirky1869/GlideFTP/releases/download/v1.7.7/GlideFTP-Linux-v1.7.7.tar.gz
sha256sum /tmp/glideftp-1.7.7.tar.gz
```

### 7. Mettre à jour packaging/PKGBUILD si le hash diffère
Ouvrir `packaging/PKGBUILD`, vérifier/corriger :
```
pkgver=1.7.7
sha256sums_x86_64=('<hash obtenu à l'étape 6>')
```
Si tu corriges le fichier, commit + push comme d'habitude :
```bash
git add packaging/PKGBUILD
git commit -m "PKGBUILD: v1.7.7"
git push origin main
```

### 8. Tester le paquet en local avant de publier sur l'AUR (recommandé)
```bash
mkdir -p /tmp/pkgtest && cd /tmp/pkgtest
cp /home/$USER/les-git-clones/GlideFTP/packaging/PKGBUILD .
cp /tmp/glideftp-1.7.7.tar.gz "glideftp-bin-1.7.7.tar.gz"
makepkg -si
```
→ installe réellement `glideftp-bin` sur la machine pour vérifier (icône, `.desktop`, `/usr/bin/glideftp`).
Désinstaller ensuite : `sudo pacman -R glideftp-bin`.

### 9. Publier sur l'AUR
```bash
cp /home/$USER/les-git-clones/GlideFTP/packaging/PKGBUILD /home/$USER/les-git-clones/AUR/glideftp-bin/
cd /home/$USER/les-git-clones/AUR/glideftp-bin
makepkg --printsrcinfo > .SRCINFO
git add PKGBUILD .SRCINFO
git commit -m "Release v1.7.7"
git push
```
Note : `.SRCINFO` n'est pas un fichier à copier depuis le projet - il n'existe nulle part ailleurs. `makepkg --printsrcinfo` le **génère** à partir du `PKGBUILD` qui vient d'être copié dans ce dossier ; il doit toujours être régénéré à cette étape, jamais recopié d'une ancienne version.

⚠️ Si `git commit`/`git add` fonctionnent mais que `git push` échoue (ou plus simplement si `git status` dans ce dossier répond "not a git repository") : le dossier `/home/$USER/les-git-clones/AUR/glideftp-bin` n'est plus un vrai clone du dépôt AUR (par exemple parce qu'il a été recréé à la main). Il faut le re-cloner (étape A.5) et remettre `PKGBUILD`/`.SRCINFO` dedans avant de committer.

C'est tout - le paquet `glideftp-bin` est mis à jour sur l'AUR, `yay -S glideftp-bin` / `yay -Syu` récupérera la nouvelle version.

**Après la toute première publication d'un paquet** (première fois seulement, pas à chaque mise à jour) : la page web du paquet (`aur.archlinux.org/packages/glideftp-bin`) est à jour immédiatement, mais l'API RPC qu'utilisent `yay`/les autres helpers met quelques minutes à s'indexer. Si `yay -S glideftp-bin` répond "target not found" juste après le premier push, ce n'est pas une erreur de ta part - attends quelques minutes et retente.

---

## Résumé express (une fois la partie A déjà faite)

```bash
# 1. doc + version
git add -A && git commit -m "v1.7.7 release notes" && git push origin main

# 2. déclenche la CI
git tag v1.7.7 && git push origin v1.7.7
# → attendre le vert dans Actions

# 3. AppImage Arch (pas fait par la CI)
./make.sh appimage-arch 1.7.7
./create-archive.sh -p appimage-arch 1.7.7
# → uploader le .tar.gz sur la draft release + Publish release

# 4. hash réel + PKGBUILD
curl -L -o /tmp/g.tar.gz https://github.com/Quirky1869/GlideFTP/releases/download/v1.7.7/GlideFTP-Linux-v1.7.7.tar.gz
sha256sum /tmp/g.tar.gz
# → corriger packaging/PKGBUILD si besoin, commit + push

# 5. AUR
cp packaging/PKGBUILD /home/$USER/les-git-clones/AUR/glideftp-bin/
cd /home/$USER/les-git-clones/AUR/glideftp-bin
makepkg --printsrcinfo > .SRCINFO
git add PKGBUILD .SRCINFO && git commit -m "Release v1.7.7" && git push
```
