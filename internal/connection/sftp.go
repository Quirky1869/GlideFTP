package connection

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"time"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	gsftp "github.com/pkg/sftp"
)

type SFTPClient struct {
	cfg    Config
	ssh    *gossh.Client
	client *gsftp.Client
}

func NewSFTPClient(cfg Config) *SFTPClient {
	return &SFTPClient{cfg: cfg}
}

func (c *SFTPClient) Connect() error {
	timeout := time.Duration(c.cfg.TimeoutSec) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	authMethods, err := c.buildAuthMethods()
	if err != nil {
		return err
	}

	user := c.cfg.User
	if user == "" {
		user = "anonymous"
	}

	sshCfg := &gossh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}

	addr := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)
	sshConn, err := gossh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}

	sftpClient, err := gsftp.NewClient(sshConn)
	if err != nil {
		sshConn.Close()
		return fmt.Errorf("SFTP session failed: %w", err)
	}

	c.ssh = sshConn
	c.client = sftpClient
	return nil
}

func (c *SFTPClient) buildAuthMethods() ([]gossh.AuthMethod, error) {
	var methods []gossh.AuthMethod

	switch c.cfg.AuthType {
	case AuthSSHKey:
		if c.cfg.SSHKeyPath == "" {
			return nil, fmt.Errorf("SSH key path required for key authentication")
		}
		key, err := os.ReadFile(c.cfg.SSHKeyPath)
		if err != nil {
			return nil, fmt.Errorf("cannot read SSH key: %w", err)
		}
		var signer gossh.Signer
		if c.cfg.Password != "" {
			signer, err = gossh.ParsePrivateKeyWithPassphrase(key, []byte(c.cfg.Password))
		} else {
			signer, err = gossh.ParsePrivateKey(key)
		}
		if err != nil {
			return nil, fmt.Errorf("cannot parse SSH key: %w", err)
		}
		methods = append(methods, gossh.PublicKeys(signer))

		// Also try SSH agent if available
		if agentSock := os.Getenv("SSH_AUTH_SOCK"); agentSock != "" {
			conn, err := net.Dial("unix", agentSock)
			if err == nil {
				methods = append(methods, gossh.PublicKeysCallback(agent.NewClient(conn).Signers))
			}
		}

	case AuthInteractive:
		methods = append(methods, gossh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range questions {
				answers[i] = c.cfg.Password
			}
			return answers, nil
		}))

	default:
		methods = append(methods, gossh.Password(c.cfg.Password))
	}

	return methods, nil
}

func (c *SFTPClient) Disconnect() error {
	if c.client != nil {
		c.client.Close()
		c.client = nil
	}
	if c.ssh != nil {
		err := c.ssh.Close()
		c.ssh = nil
		return err
	}
	return nil
}

func (c *SFTPClient) ListDir(path string) ([]RemoteFileEntry, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not connected")
	}
	entries, err := c.client.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var result []RemoteFileEntry
	for _, e := range entries {
		result = append(result, RemoteFileEntry{
			Name:    e.Name(),
			Path:    path + "/" + e.Name(),
			IsDir:   e.IsDir(),
			Size:    e.Size(),
			ModTime: e.ModTime(),
			Mode:    e.Mode().String(),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (c *SFTPClient) MkDir(path string) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}
	return c.client.MkdirAll(path)
}

func (c *SFTPClient) Delete(path string) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}
	info, err := c.client.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return c.deleteDir(path)
	}
	return c.client.Remove(path)
}

func (c *SFTPClient) deleteDir(path string) error {
	entries, err := c.client.ReadDir(path)
	if err != nil {
		return err
	}
	for _, e := range entries {
		full := path + "/" + e.Name()
		if e.IsDir() {
			if err := c.deleteDir(full); err != nil {
				return err
			}
		} else {
			if err := c.client.Remove(full); err != nil {
				return err
			}
		}
	}
	return c.client.RemoveDirectory(path)
}

func (c *SFTPClient) Rename(oldPath, newPath string) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}
	return c.client.Rename(oldPath, newPath)
}

func (c *SFTPClient) CurrentDir() (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("not connected")
	}
	return c.client.Getwd()
}

func (c *SFTPClient) Upload(ctx context.Context, localPath, remotePath string, progress func(sent, total int64)) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	if err := c.client.MkdirAll(filepath.Dir(remotePath)); err != nil {
		return err
	}

	dst, err := c.client.Create(remotePath)
	if err != nil {
		return err
	}
	defer dst.Close()

	pr := &progressReader{ctx: ctx, r: f, total: info.Size(), cb: progress}
	_, err = io.Copy(dst, pr)
	return err
}

func (c *SFTPClient) Download(ctx context.Context, remotePath, localPath string, progress func(received, total int64)) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}
	src, err := c.client.Open(remotePath)
	if err != nil {
		return err
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}
	dst, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	pw := &progressWriter{ctx: ctx, w: dst, total: info.Size(), cb: progress}
	_, err = io.Copy(pw, src)
	return err
}
