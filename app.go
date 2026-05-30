package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	gfeCrypto "GlideFTP/internal/crypto"
	"GlideFTP/internal/connection"
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
	a.migratePasswords()
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
	a.queue.SetSpeedLimit(s.MaxTransferSpeedKBps)
	return s.Save()
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

// ─── Local Filesystem ─────────────────────────────────────────────────────────

func (a *App) LocalListDir(path string) ([]localfs.FileEntry, error) {
	return localfs.ListDir(path, a.appSettings.ShowHiddenFiles)
}

func (a *App) LocalMkDir(path string) error {
	return localfs.MkDir(path)
}

func (a *App) LocalDelete(path string) error {
	return localfs.Delete(path)
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
