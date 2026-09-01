package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"GlideFTP/internal/connection"
	gfeCrypto "GlideFTP/internal/crypto"
	localfs "GlideFTP/internal/fs"
	"GlideFTP/internal/keyring"
	"GlideFTP/internal/settings"
	"GlideFTP/internal/sites"
	"GlideFTP/internal/transfer"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx         context.Context
	connMgr     *connection.Manager
	queue       *transfer.Queue
	siteMgr     *sites.Manager
	appSettings *settings.Settings
	keyringMgr  *keyring.Manager
}

func NewApp() *App {
	s, _ := settings.Load()
	app := &App{
		connMgr:     connection.NewManager(),
		siteMgr:     sites.NewManager(),
		appSettings: s,
		keyringMgr:  keyring.NewManager(),
	}
	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	emitter := func(name string, data interface{}) {
		runtime.EventsEmit(ctx, name, data)
	}
	a.queue = transfer.NewQueue(a.appSettings.MaxConcurrentTransfers, emitter)
	a.queue.SetSpeedLimit(a.appSettings.MaxTransferSpeedKBps)
	a.connMgr.SetOnConnectionLost(func(id, host string) {
		runtime.EventsEmit(ctx, "connection:lost", map[string]string{"id": id, "host": host})
	})
	a.migratePasswords()
	s := a.appSettings
	if s.StartMaximized {
		runtime.WindowMaximise(ctx)
	} else if s.WindowWidth > 0 && s.WindowHeight > 0 {
		runtime.WindowSetSize(ctx, s.WindowWidth, s.WindowHeight)
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.connMgr.Disconnect()
}

// migratePasswords moves any plaintext passwords still in sites.json into the keyring.
// This handles the one-time migration from the pre-keyring format.
func (a *App) migratePasswords() {
	if !a.keyringMgr.IsAvailable() {
		return
	}
	allSites := a.siteMgr.GetAll()
	migrated := false
	for _, site := range allSites {
		if site.Password != "" && needsKeyring(string(site.AuthType)) {
			if err := a.keyringMgr.Set(site.ID, site.Password); err == nil {
				migrated = true
			}
		}
	}
	if migrated {
		// Re-save sites.json with passwords stripped (save() always strips them).
		_ = a.siteMgr.Persist()
	}
}

// needsKeyring reports whether a given authType should store a password in the keyring.
func needsKeyring(authType string) bool {
	switch authType {
	case "anonymous", "ask_password", "interactive":
		return false
	default:
		return true
	}
}

// ─── Settings ────────────────────────────────────────────────────────────────

func (a *App) GetSettings() *settings.Settings {
	return a.appSettings
}

func (a *App) SaveSettings(s settings.Settings) error {
	a.appSettings = &s
	a.queue.SetWorkers(s.MaxConcurrentTransfers)
	a.queue.SetSpeedLimit(s.MaxTransferSpeedKBps)
	return s.Save()
}

// ExportSettings exports all current app settings (the whole Settings page) as plain JSON.
func (a *App) ExportSettings() error {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export settings",
		DefaultFilename: "glideftp-settings.json",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files", Pattern: "*.json"},
		},
	})
	if err != nil || path == "" {
		return err
	}
	data, err := json.MarshalIndent(a.appSettings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// ImportSettings opens a file picker, loads settings from a previously exported JSON file,
// applies them immediately (transfer workers/speed limit included) and persists them to disk.
// Fields missing from the file (e.g. an older export) fall back to defaults rather than being zeroed.
func (a *App) ImportSettings() (*settings.Settings, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Import settings",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files", Pattern: "*.json"},
			{DisplayName: "All Files", Pattern: "*"},
		},
	})
	if err != nil || path == "" {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := settings.Default()
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("fichier de paramètres invalide : %w", err)
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	a.appSettings = s
	a.queue.SetWorkers(s.MaxConcurrentTransfers)
	a.queue.SetSpeedLimit(s.MaxTransferSpeedKBps)
	return s, nil
}

// ─── Keyring ─────────────────────────────────────────────────────────────────

// GetKeyringStatus returns "" when the system keyring is available, or an error key otherwise.
func (a *App) GetKeyringStatus() string {
	if a.keyringMgr.IsAvailable() {
		return ""
	}
	return "keyring_unavailable"
}

// ─── Sites ───────────────────────────────────────────────────────────────────

// GetSites returns all saved sites, with passwords populated from the keyring.
func (a *App) GetSites() []sites.Site {
	siteList := a.siteMgr.GetAll()
	for i := range siteList {
		if pwd, err := a.keyringMgr.Get(siteList[i].ID); err == nil && pwd != "" {
			siteList[i].Password = pwd
		}
	}
	return siteList
}

func (a *App) CreateSite(s sites.Site) (sites.Site, error) {
	password := s.Password
	s.Password = ""
	created, err := a.siteMgr.Create(s)
	if err != nil {
		return sites.Site{}, err
	}
	if password != "" && needsKeyring(string(s.AuthType)) {
		_ = a.keyringMgr.Set(created.ID, password)
	}
	created.Password = password
	return created, nil
}

func (a *App) UpdateSite(s sites.Site) error {
	password := s.Password
	s.Password = ""
	if err := a.siteMgr.Update(s); err != nil {
		return err
	}
	if !needsKeyring(string(s.AuthType)) || password == "" {
		_ = a.keyringMgr.Delete(s.ID)
	} else {
		_ = a.keyringMgr.Set(s.ID, password)
	}
	return nil
}

func (a *App) DeleteSite(id string) error {
	_ = a.keyringMgr.Delete(id)
	return a.siteMgr.Delete(id)
}

// ReorderSites persists a new site ordering (drag-and-drop reorder in Site Manager).
func (a *App) ReorderSites(orderedIDs []string) error {
	return a.siteMgr.Reorder(orderedIDs)
}

// ─── Export / Import ─────────────────────────────────────────────────────────

// ExportSitesPlain exports all sites as plain JSON without passwords.
func (a *App) ExportSitesPlain() error {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export sites",
		DefaultFilename: "glideftp-sites.json",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files", Pattern: "*.json"},
		},
	})
	if err != nil || path == "" {
		return err
	}
	allSites := a.siteMgr.GetAll()
	for i := range allSites {
		allSites[i].Password = ""
	}
	data, err := json.MarshalIndent(allSites, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// ExportSitesEncrypted exports all sites with passwords, encrypted with Argon2id+AES-256-GCM.
func (a *App) ExportSitesEncrypted(passphrase string) error {
	if passphrase == "" {
		return fmt.Errorf("la passphrase ne peut pas être vide")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export sites (chiffré)",
		DefaultFilename: "glideftp-sites.gfe",
		Filters: []runtime.FileFilter{
			{DisplayName: "GlideFTP Export", Pattern: "*.gfe"},
		},
	})
	if err != nil || path == "" {
		return err
	}
	allSites := a.siteMgr.GetAll()
	for i := range allSites {
		if pwd, err := a.keyringMgr.Get(allSites[i].ID); err == nil && pwd != "" {
			allSites[i].Password = pwd
		}
	}
	plaintext, err := json.MarshalIndent(allSites, "", "  ")
	if err != nil {
		return err
	}
	encrypted, err := gfeCrypto.Encrypt(plaintext, passphrase)
	if err != nil {
		return fmt.Errorf("chiffrement échoué : %w", err)
	}
	return os.WriteFile(path, encrypted, 0600)
}

// ExportSitesPlainSelected exports the given sites (by ID) as plain JSON without passwords.
// If ids is empty all sites are exported.
func (a *App) ExportSitesPlainSelected(ids []string) error {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export sites",
		DefaultFilename: "glideftp-sites.json",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files", Pattern: "*.json"},
		},
	})
	if err != nil || path == "" {
		return err
	}
	selected := filterSitesByIDs(a.siteMgr.GetAll(), ids)
	for i := range selected {
		selected[i].Password = ""
	}
	data, err := json.MarshalIndent(selected, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// ExportSitesEncryptedSelected exports the given sites (by ID) with passwords, encrypted.
// If ids is empty all sites are exported.
func (a *App) ExportSitesEncryptedSelected(passphrase string, ids []string) error {
	if passphrase == "" {
		return fmt.Errorf("la passphrase ne peut pas être vide")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export sites (chiffré)",
		DefaultFilename: "glideftp-sites.gfe",
		Filters: []runtime.FileFilter{
			{DisplayName: "GlideFTP Export", Pattern: "*.gfe"},
		},
	})
	if err != nil || path == "" {
		return err
	}
	selected := filterSitesByIDs(a.siteMgr.GetAll(), ids)
	for i := range selected {
		if pwd, kerr := a.keyringMgr.Get(selected[i].ID); kerr == nil && pwd != "" {
			selected[i].Password = pwd
		}
	}
	plaintext, err := json.MarshalIndent(selected, "", "  ")
	if err != nil {
		return err
	}
	encrypted, err := gfeCrypto.Encrypt(plaintext, passphrase)
	if err != nil {
		return fmt.Errorf("chiffrement échoué : %w", err)
	}
	return os.WriteFile(path, encrypted, 0600)
}

// filterSitesByIDs returns only those sites whose ID appears in ids.
// If ids is empty all sites are returned.
func filterSitesByIDs(all []sites.Site, ids []string) []sites.Site {
	if len(ids) == 0 {
		return all
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	out := make([]sites.Site, 0, len(ids))
	for _, s := range all {
		if set[s.ID] {
			out = append(out, s)
		}
	}
	return out
}

// ImportFileInfo is returned by OpenImportDialog to tell the frontend what kind of file was selected.
type ImportFileInfo struct {
	Path            string `json:"path"`
	NeedsPassphrase bool   `json:"needsPassphrase"`
}

// OpenImportDialog opens a file picker and reports whether the selected file is encrypted.
func (a *App) OpenImportDialog() (ImportFileInfo, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Import sites",
		Filters: []runtime.FileFilter{
			{DisplayName: "GlideFTP Export", Pattern: "*.gfe;*.json"},
			{DisplayName: "All Files", Pattern: "*"},
		},
	})
	if err != nil || path == "" {
		return ImportFileInfo{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ImportFileInfo{}, err
	}
	return ImportFileInfo{
		Path:            path,
		NeedsPassphrase: gfeCrypto.IsEncrypted(data),
	}, nil
}

// DoImportSites imports sites from a file (plain JSON or .gfe encrypted).
// Pass an empty passphrase for plain JSON files.
func (a *App) DoImportSites(path, passphrase string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var jsonData []byte
	if gfeCrypto.IsEncrypted(data) {
		if passphrase == "" {
			return 0, fmt.Errorf("passphrase requise pour ce fichier")
		}
		jsonData, err = gfeCrypto.Decrypt(data, passphrase)
		if err != nil {
			return 0, err
		}
	} else {
		jsonData = data
	}
	var imported []sites.Site
	if err := json.Unmarshal(jsonData, &imported); err != nil {
		return 0, fmt.Errorf("format de fichier invalide : %w", err)
	}
	count := 0
	for _, s := range imported {
		password := s.Password
		s.ID = ""
		s.Password = ""
		created, err := a.siteMgr.Create(s)
		if err != nil {
			continue
		}
		if password != "" && needsKeyring(string(s.AuthType)) {
			_ = a.keyringMgr.Set(created.ID, password)
		}
		count++
	}
	return count, nil
}

// ─── Connection ───────────────────────────────────────────────────────────────

func (a *App) buildSiteConfig(site sites.Site, password string) connection.Config {
	if password == "" {
		if site.Password != "" {
			// Password already populated (e.g. during migration before keyring save)
			password = site.Password
		} else {
			password, _ = a.keyringMgr.Get(site.ID)
		}
	}
	return connection.Config{
		Protocol:   connection.Protocol(site.Protocol),
		Host:       site.Host,
		Port:       site.Port,
		User:       site.User,
		Password:   password,
		Encryption: connection.EncryptionType(site.Encryption),
		AuthType:   connection.AuthType(site.AuthType),
		SSHKeyPath: site.SSHKeyPath,
		TimeoutSec: a.appSettings.ConnectionTimeoutSec,
		Passive:    a.appSettings.PassiveMode,
	}
}

func (a *App) connInfoFrom(cfg connection.Config, id, name string) connection.ConnInfo {
	return connection.ConnInfo{
		ID:       id,
		Name:     name,
		Host:     cfg.Host,
		Protocol: string(cfg.Protocol),
		Port:     cfg.Port,
		User:     cfg.User,
	}
}

// Connect is called by the ConnectionBar (direct connect, not via a saved site).
func (a *App) Connect(cfg connection.Config) (connection.ConnInfo, error) {
	if cfg.TimeoutSec == 0 {
		cfg.TimeoutSec = a.appSettings.ConnectionTimeoutSec
	}
	name := cfg.Host
	id, err := a.connMgr.Connect(cfg, name)
	if err != nil {
		return connection.ConnInfo{}, err
	}
	a.queue.SetExecutor(a.connMgr.GetClient())
	return a.connInfoFrom(cfg, id, name), nil
}

// ConnectToSite connects to a saved site, replacing the currently active connection.
func (a *App) ConnectToSite(siteID string) (connection.ConnInfo, error) {
	site, ok := a.siteMgr.GetByID(siteID)
	if !ok {
		return connection.ConnInfo{}, fmt.Errorf("site not found")
	}
	name := site.Name
	if name == "" {
		name = site.Host
	}
	cfg := a.buildSiteConfig(site, "")
	id, err := a.connMgr.Connect(cfg, name)
	if err != nil {
		return connection.ConnInfo{}, err
	}
	a.queue.SetExecutor(a.connMgr.GetClient())
	return a.connInfoFrom(cfg, id, name), nil
}

// ConnectWithPassword connects to an ask_password site with a runtime password.
func (a *App) ConnectWithPassword(siteID, password string) (connection.ConnInfo, error) {
	site, ok := a.siteMgr.GetByID(siteID)
	if !ok {
		return connection.ConnInfo{}, fmt.Errorf("site not found")
	}
	name := site.Name
	if name == "" {
		name = site.Host
	}
	cfg := a.buildSiteConfig(site, password)
	cfg.AuthType = connection.AuthPassword
	id, err := a.connMgr.Connect(cfg, name)
	if err != nil {
		return connection.ConnInfo{}, err
	}
	a.queue.SetExecutor(a.connMgr.GetClient())
	return a.connInfoFrom(cfg, id, name), nil
}

// ConnectAdditional adds a new connection alongside existing ones using a direct config (no saved site).
func (a *App) ConnectAdditional(cfg connection.Config) (connection.ConnInfo, error) {
	if cfg.TimeoutSec == 0 {
		cfg.TimeoutSec = a.appSettings.ConnectionTimeoutSec
	}
	name := cfg.Host
	id, err := a.connMgr.ConnectNew(cfg, name)
	if err != nil {
		return connection.ConnInfo{}, err
	}
	a.queue.SetExecutor(a.connMgr.GetClient())
	return a.connInfoFrom(cfg, id, name), nil
}

// ConnectToSiteAdditional adds a new connection while keeping existing ones active.
// overridePassword is used for ask_password sites; pass "" otherwise.
func (a *App) ConnectToSiteAdditional(siteID, overridePassword string) (connection.ConnInfo, error) {
	site, ok := a.siteMgr.GetByID(siteID)
	if !ok {
		return connection.ConnInfo{}, fmt.Errorf("site not found")
	}
	name := site.Name
	if name == "" {
		name = site.Host
	}
	cfg := a.buildSiteConfig(site, overridePassword)
	if overridePassword != "" {
		cfg.AuthType = connection.AuthPassword
	}
	id, err := a.connMgr.ConnectNew(cfg, name)
	if err != nil {
		return connection.ConnInfo{}, err
	}
	a.queue.SetExecutor(a.connMgr.GetClient())
	return a.connInfoFrom(cfg, id, name), nil
}

// GetConnections returns info about all currently open connections.
func (a *App) GetConnections() []connection.ConnInfo {
	return a.connMgr.GetConnections()
}

// SwitchConnection makes a different connection the active one.
func (a *App) SwitchConnection(id string) error {
	if err := a.connMgr.SwitchTo(id); err != nil {
		return err
	}
	a.queue.SetExecutor(a.connMgr.GetClient())
	return nil
}

// CloseConnection disconnects and removes a specific connection.
func (a *App) CloseConnection(id string) error {
	if err := a.connMgr.CloseOne(id); err != nil {
		return err
	}
	a.queue.SetExecutor(a.connMgr.GetClient())
	return nil
}

// GetActiveConnectionID returns the ID of the currently active connection.
func (a *App) GetActiveConnectionID() string {
	return a.connMgr.ActiveID()
}

func (a *App) Disconnect() error {
	a.queue.SetExecutor(nil)
	return a.connMgr.Disconnect()
}

func (a *App) GetConnectionStatus() string {
	return string(a.connMgr.GetStatus())
}

// ─── Remote Filesystem ────────────────────────────────────────────────────────

func (a *App) RemoteListDir(path string) ([]connection.RemoteFileEntry, error) {
	entries, err := a.connMgr.ListDir(path)
	if err != nil {
		return nil, err
	}
	a.connMgr.SetCwd(path)
	return entries, nil
}

func (a *App) RemoteMkDir(path string) error {
	return a.connMgr.MkDir(path)
}

func (a *App) RemoteDelete(path string) error {
	return a.connMgr.Delete(path)
}

func (a *App) RemoteRename(oldPath, newPath string) error {
	return a.connMgr.Rename(oldPath, newPath)
}

func (a *App) GetRemoteCwd() string {
	return a.connMgr.GetCwd()
}

// remoteSearchResultLimit caps how many matches RemoteSearch returns, to
// keep a recursive search over a huge remote tree from hanging the UI
// (each subdirectory requires a network round-trip).
const remoteSearchResultLimit = 500

func (a *App) RemoteSearch(path, query string, recursive bool) ([]connection.RemoteFileEntry, error) {
	var result []connection.RemoteFileEntry
	err := a.remoteSearchWalk(path, strings.ToLower(query), recursive, &result)
	if err != nil && len(result) == 0 {
		return nil, err
	}
	return result, nil
}

func (a *App) remoteSearchWalk(path, q string, recursive bool, result *[]connection.RemoteFileEntry) error {
	if len(*result) >= remoteSearchResultLimit {
		return nil
	}
	entries, err := a.connMgr.ListDir(path)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if len(*result) >= remoteSearchResultLimit {
			return nil
		}
		if strings.Contains(strings.ToLower(e.Name), q) {
			*result = append(*result, e)
		}
		if recursive && e.IsDir {
			// Best-effort: a permission error on one subdirectory
			// shouldn't abort the rest of the search.
			_ = a.remoteSearchWalk(e.Path, q, recursive, result)
		}
	}
	return nil
}

// ─── Local Filesystem ─────────────────────────────────────────────────────────

func (a *App) LocalListDir(path string) ([]localfs.FileEntry, error) {
	return localfs.ListDir(path, a.appSettings.ShowHiddenFiles)
}

func (a *App) LocalSearch(path, query string, recursive bool) ([]localfs.FileEntry, error) {
	return localfs.Search(path, query, recursive, a.appSettings.ShowHiddenFiles)
}

func (a *App) LocalMkDir(path string) error {
	return localfs.MkDir(path)
}

func (a *App) LocalCopy(srcPath, destPath string) error {
	return localfs.Copy(srcPath, destPath)
}

func (a *App) RemoteCopy(srcPath, destPath string) error {
	return a.connMgr.CopyRemote(srcPath, destPath)
}

func (a *App) RemoteCopyDir(srcPath, destPath string) error {
	return a.connMgr.CopyRemoteDir(srcPath, destPath)
}

func (a *App) LocalDelete(path string) error {
	return localfs.Trash(path)
}

func (a *App) LocalRename(oldPath, newPath string) error {
	return localfs.Rename(oldPath, newPath)
}

func (a *App) GetLocalHome() string {
	return localfs.HomeDir()
}

func (a *App) GetLocalParent(path string) string {
	return localfs.ParentDir(path)
}

func (a *App) GetLocalRoots() []localfs.FileEntry {
	return localfs.Roots()
}

func (a *App) BrowseLocalDir() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select folder",
	})
	return dir, err
}

func (a *App) BrowseSSHKey() (string, error) {
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select SSH key",
		Filters: []runtime.FileFilter{
			{DisplayName: "SSH Keys", Pattern: "*.pem;*.key;*.ppk;id_rsa;id_ed25519;id_ecdsa"},
			{DisplayName: "All Files", Pattern: "*"},
		},
	})
	return file, err
}

// ─── Transfers ────────────────────────────────────────────────────────────────

func (a *App) QueueUpload(localPath, remotePath string) {
	a.queue.Add(transfer.Upload, localPath, remotePath, a.connMgr.GetActiveHost())
}

func (a *App) QueueDownload(remotePath, localPath string) {
	a.queue.Add(transfer.Download, localPath, remotePath, a.connMgr.GetActiveHost())
}

// QueueUploadDir recursively enumerates localPath and queues one upload job per file.
// Remote subdirectories are created before their contents are queued.
func (a *App) QueueUploadDir(localPath, remotePath string) {
	go a.enqueueUploadDir(localPath, remotePath)
}

func (a *App) enqueueUploadDir(localPath, remotePath string) {
	if err := a.connMgr.MkDir(remotePath); err != nil {
		return
	}
	entries, err := os.ReadDir(localPath)
	if err != nil {
		return
	}
	host := a.connMgr.GetActiveHost()
	for _, e := range entries {
		src := filepath.Join(localPath, e.Name())
		dst := remotePath + "/" + e.Name()
		if e.IsDir() {
			a.enqueueUploadDir(src, dst)
		} else {
			a.queue.Add(transfer.Upload, src, dst, host)
		}
	}
}

// QueueDownloadDir recursively enumerates remotePath and queues one download job per file.
// Local subdirectories are created before their contents are queued.
func (a *App) QueueDownloadDir(remotePath, localPath string) {
	go a.enqueueDownloadDir(remotePath, localPath)
}

func (a *App) enqueueDownloadDir(remotePath, localPath string) {
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return
	}
	entries, err := a.connMgr.ListDir(remotePath)
	if err != nil {
		return
	}
	host := a.connMgr.GetActiveHost()
	for _, e := range entries {
		dst := filepath.Join(localPath, e.Name)
		if e.IsDir {
			a.enqueueDownloadDir(e.Path, dst)
		} else {
			a.queue.Add(transfer.Download, dst, e.Path, host)
		}
	}
}

func (a *App) GetTransfers() []*transfer.Job {
	return a.queue.GetAll()
}

func (a *App) CancelTransfer(id string) error {
	return a.queue.Cancel(id)
}

func (a *App) RetryTransfer(id string) error {
	return a.queue.Retry(id)
}

func (a *App) ClearTransfers(status string) {
	a.queue.Clear(transfer.JobStatus(status))
}

func (a *App) RemoveTransfer(id string) error {
	return a.queue.RemoveJob(id)
}
