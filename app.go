package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"GlideFTP/internal/connection"
	localfs "GlideFTP/internal/fs"
	"GlideFTP/internal/settings"
	"GlideFTP/internal/sites"
	"GlideFTP/internal/transfer"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx        context.Context
	connMgr    *connection.Manager
	queue      *transfer.Queue
	siteMgr    *sites.Manager
	appSettings *settings.Settings
}

func NewApp() *App {
	s, _ := settings.Load()
	app := &App{
		connMgr:     connection.NewManager(),
		siteMgr:     sites.NewManager(),
		appSettings: s,
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
}

func (a *App) shutdown(ctx context.Context) {
	a.connMgr.Disconnect()
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

// ─── Sites ───────────────────────────────────────────────────────────────────

func (a *App) GetSites() []sites.Site {
	return a.siteMgr.GetAll()
}

func (a *App) CreateSite(s sites.Site) (sites.Site, error) {
	return a.siteMgr.Create(s)
}

func (a *App) UpdateSite(s sites.Site) error {
	return a.siteMgr.Update(s)
}

func (a *App) DeleteSite(id string) error {
	return a.siteMgr.Delete(id)
}

func (a *App) ExportSites() error {
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
	data, err := json.MarshalIndent(a.siteMgr.GetAll(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (a *App) ImportSites() (int, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Import sites",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files", Pattern: "*.json"},
		},
	})
	if err != nil || path == "" {
		return 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var imported []sites.Site
	if err := json.Unmarshal(data, &imported); err != nil {
		return 0, fmt.Errorf("invalid file format: %w", err)
	}
	count := 0
	for _, s := range imported {
		s.ID = ""
		if _, err := a.siteMgr.Create(s); err == nil {
			count++
		}
	}
	return count, nil
}

// ─── Connection ───────────────────────────────────────────────────────────────

func (a *App) buildSiteConfig(site sites.Site, password string) connection.Config {
	if password == "" {
		password = site.Password
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
