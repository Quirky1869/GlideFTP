package connection

import (
	"context"
	"time"
)

type Protocol string
type AuthType string
type EncryptionType string

const (
	ProtocolFTP  Protocol = "ftp"
	ProtocolSFTP Protocol = "sftp"

	AuthPassword    AuthType = "password"
	AuthSSHKey      AuthType = "key"
	AuthInteractive AuthType = "interactive"
	AuthAnonymous   AuthType = "anonymous"

	EncryptionNone  EncryptionType = "none"
	EncryptionTLS   EncryptionType = "tls"
	EncryptionFTPES EncryptionType = "ftpes"
)

type Config struct {
	Protocol   Protocol       `json:"protocol"`
	Host       string         `json:"host"`
	Port       int            `json:"port"`
	User       string         `json:"user"`
	Password   string         `json:"password"`
	Encryption EncryptionType `json:"encryption"`
	AuthType   AuthType       `json:"authType"`
	SSHKeyPath string         `json:"sshKeyPath"`
	TimeoutSec int            `json:"timeoutSec"`
	Passive    bool           `json:"passive"`
}

// ConnInfo is the public representation of an active connection, returned to the frontend.
type ConnInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	User     string `json:"user"`
}

type RemoteFileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	Mode    string    `json:"mode"`
}

type Client interface {
	Connect() error
	Disconnect() error
	Keepalive() error
	ListDir(path string) ([]RemoteFileEntry, error)
	MkDir(path string) error
	Delete(path string) error
	Rename(oldPath, newPath string) error
	Upload(ctx context.Context, localPath, remotePath string, progress func(sent, total int64)) error
	Download(ctx context.Context, remotePath, localPath string, progress func(received, total int64)) error
	CurrentDir() (string, error)
}
