package connection

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	goftp "github.com/jlaffaye/ftp"
)

type FTPClient struct {
	cfg  Config
	mu   sync.Mutex
	conn *goftp.ServerConn // for control operations (ListDir, MkDir, Delete, Rename)
}

func NewFTPClient(cfg Config) *FTPClient {
	return &FTPClient{cfg: cfg}
}

func (c *FTPClient) dial() (*goftp.ServerConn, error) {
	timeout := time.Duration(c.cfg.TimeoutSec) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	opts := []goftp.DialOption{
		goftp.DialWithTimeout(timeout),
	}

	if c.cfg.Encryption == EncryptionTLS || c.cfg.Encryption == EncryptionFTPES {
		opts = append(opts, goftp.DialWithExplicitTLS(nil))
	}

	addr := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)
	conn, err := goftp.Dial(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}

	user := c.cfg.User
	pass := c.cfg.Password
	if c.cfg.AuthType == AuthAnonymous || user == "" {
		user = "anonymous"
		pass = "anonymous@"
	}

	if err := conn.Login(user, pass); err != nil {
		conn.Quit()
		return nil, fmt.Errorf("login failed: %w", err)
	}

	return conn, nil
}

func (c *FTPClient) Connect() error {
	conn, err := c.dial()
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	return nil
}

func (c *FTPClient) Disconnect() error {
	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()
	if conn != nil {
		return conn.Quit()
	}
	return nil
}

func (c *FTPClient) ListDir(path string) ([]RemoteFileEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}
	entries, err := c.conn.List(path)
	if err != nil {
		return nil, err
	}
	var result []RemoteFileEntry
	for _, e := range entries {
		if e.Name == "." || e.Name == ".." {
			continue
		}
		entryPath := path
		if !strings.HasSuffix(entryPath, "/") {
			entryPath += "/"
		}
		entryPath += e.Name

		result = append(result, RemoteFileEntry{
			Name:    e.Name,
			Path:    entryPath,
			IsDir:   e.Type == goftp.EntryTypeFolder,
			Size:    int64(e.Size),
			ModTime: e.Time,
		})
	}
	return result, nil
}

func (c *FTPClient) MkDir(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return c.conn.MakeDir(path)
}

func (c *FTPClient) Delete(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	err := c.conn.Delete(path)
	if err != nil {
		return c.conn.RemoveDirRecur(path)
	}
	return nil
}

func (c *FTPClient) Rename(oldPath, newPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return c.conn.Rename(oldPath, newPath)
}

func (c *FTPClient) CurrentDir() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return "", fmt.Errorf("not connected")
	}
	return c.conn.CurrentDir()
}

// Upload opens a dedicated FTP connection for this transfer so multiple uploads
// can run concurrently (each on its own control+data connection pair).
func (c *FTPClient) Upload(ctx context.Context, localPath, remotePath string, progress func(sent, total int64)) error {
	c.mu.Lock()
	connected := c.conn != nil
	c.mu.Unlock()
	if !connected {
		return fmt.Errorf("not connected")
	}

	conn, err := c.dial()
	if err != nil {
		return err
	}
	defer conn.Quit()

	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	pr := &progressReader{ctx: ctx, r: f, total: info.Size(), cb: progress}
	return conn.Stor(remotePath, pr)
}

// Download opens a dedicated FTP connection for this transfer so multiple downloads
// can run concurrently (each on its own control+data connection pair).
func (c *FTPClient) Download(ctx context.Context, remotePath, localPath string, progress func(received, total int64)) error {
	c.mu.Lock()
	connected := c.conn != nil
	c.mu.Unlock()
	if !connected {
		return fmt.Errorf("not connected")
	}

	conn, err := c.dial()
	if err != nil {
		return err
	}
	defer conn.Quit()

	// SIZE must be sent on the control connection BEFORE RETR opens the data transfer.
	size, _ := conn.FileSize(remotePath)

	resp, err := conn.Retr(remotePath)
	if err != nil {
		return err
	}
	defer resp.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}
	f, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	pw := &progressWriter{ctx: ctx, w: f, total: size, cb: progress}
	_, err = io.Copy(pw, resp)
	return err
}

type progressReader struct {
	ctx   context.Context
	r     io.Reader
	total int64
	sent  int64
	cb    func(sent, total int64)
}

func (p *progressReader) Read(buf []byte) (int, error) {
	if p.ctx != nil {
		if err := p.ctx.Err(); err != nil {
			return 0, err
		}
	}
	n, err := p.r.Read(buf)
	p.sent += int64(n)
	if p.cb != nil {
		p.cb(p.sent, p.total)
	}
	return n, err
}

type progressWriter struct {
	ctx      context.Context
	w        io.Writer
	total    int64
	received int64
	cb       func(received, total int64)
}

func (p *progressWriter) Write(buf []byte) (int, error) {
	if p.ctx != nil {
		if err := p.ctx.Err(); err != nil {
			return 0, err
		}
	}
	n, err := p.w.Write(buf)
	p.received += int64(n)
	if p.cb != nil {
		p.cb(p.received, p.total)
	}
	return n, err
}
