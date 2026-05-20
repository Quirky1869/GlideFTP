package connection

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	goftp "github.com/jlaffaye/ftp"
)

type FTPClient struct {
	cfg  Config
	conn *goftp.ServerConn
}

func NewFTPClient(cfg Config) *FTPClient {
	return &FTPClient{cfg: cfg}
}

func (c *FTPClient) Connect() error {
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
		return fmt.Errorf("connection failed: %w", err)
	}

	user := c.cfg.User
	pass := c.cfg.Password
	if c.cfg.AuthType == AuthAnonymous || user == "" {
		user = "anonymous"
		pass = "anonymous@"
	}

	if err := conn.Login(user, pass); err != nil {
		conn.Quit()
		return fmt.Errorf("login failed: %w", err)
	}

	c.conn = conn
	return nil
}

func (c *FTPClient) Disconnect() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Quit()
	c.conn = nil
	return err
}

func (c *FTPClient) ListDir(path string) ([]RemoteFileEntry, error) {
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
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return c.conn.MakeDir(path)
}

func (c *FTPClient) Delete(path string) error {
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
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return c.conn.Rename(oldPath, newPath)
}

func (c *FTPClient) CurrentDir() (string, error) {
	if c.conn == nil {
		return "", fmt.Errorf("not connected")
	}
	return c.conn.CurrentDir()
}

func (c *FTPClient) Upload(localPath, remotePath string, progress func(sent, total int64)) error {
	if c.conn == nil {
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
	total := info.Size()

	pr := &progressReader{r: f, total: total, cb: progress}
	return c.conn.Stor(remotePath, pr)
}

func (c *FTPClient) Download(remotePath, localPath string, progress func(received, total int64)) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	resp, err := c.conn.Retr(remotePath)
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

	size, _ := c.conn.FileSize(remotePath)
	pw := &progressWriter{w: f, total: size, cb: progress}
	_, err = io.Copy(pw, resp)
	return err
}

type progressReader struct {
	r     io.Reader
	total int64
	sent  int64
	cb    func(sent, total int64)
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	p.sent += int64(n)
	if p.cb != nil {
		p.cb(p.sent, p.total)
	}
	return n, err
}

type progressWriter struct {
	w        io.Writer
	total    int64
	received int64
	cb       func(received, total int64)
}

func (p *progressWriter) Write(buf []byte) (int, error) {
	n, err := p.w.Write(buf)
	p.received += int64(n)
	if p.cb != nil {
		p.cb(p.received, p.total)
	}
	return n, err
}
