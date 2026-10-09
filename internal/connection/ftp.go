package connection

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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

// isConnLost reports whether err means the control connection itself is
// gone (closed by the server, network cut) rather than a command error such
// as "550 permission denied". Windows socket errors don't map to syscall.EPIPE
// / ECONNRESET, hence the message checks.
func isConnLost(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, net.ErrClosed) || errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED) {
		return true
	}
	var tpErr *textproto.Error
	if errors.As(err, &tpErr) && tpErr.Code == 421 { // "Service not available, closing control connection"
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, s := range []string{"broken pipe", "connection reset", "use of closed network connection",
		"forcibly closed", "connection was aborted", "connection refused"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// reconnectLocked replaces a dead control connection with a fresh one (new
// dial + login). Paths are always absolute, so no state needs restoring.
// Caller must hold c.mu.
func (c *FTPClient) reconnectLocked() error {
	conn, err := c.dial()
	if err != nil {
		return err
	}
	if old := c.conn; old != nil {
		go old.Quit() // dead anyway; don't block on it
	}
	c.conn = conn
	return nil
}

// control runs a control-connection operation under c.mu. If it fails
// because the server closed the session (idle timeout, flood protection,
// network cut...), it reconnects and retries once, so the user doesn't have
// to disconnect/reconnect by hand. If reconnecting fails too, the original
// error is returned.
func (c *FTPClient) control(op func(conn *goftp.ServerConn) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	err := op(c.conn)
	if !isConnLost(err) {
		return err
	}
	if rerr := c.reconnectLocked(); rerr != nil {
		return err
	}
	return op(c.conn)
}

// Keepalive sends a NOOP command to keep the control connection alive. If
// the server has dropped the session, it reconnects; only a failed
// reconnection is reported (and makes the manager declare the connection lost).
// Uses TryLock so it never blocks if another control operation is already running.
func (c *FTPClient) Keepalive() error {
	if !c.mu.TryLock() {
		return nil // another operation is in progress; connection is alive
	}
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	err := c.conn.NoOp()
	if isConnLost(err) {
		return c.reconnectLocked()
	}
	return err
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
	var entries []*goftp.Entry
	err := c.control(func(conn *goftp.ServerConn) error {
		var lerr error
		entries, lerr = conn.List(path)
		return lerr
	})
	if err != nil {
		return nil, err
	}
	return ftpEntries(path, entries), nil
}

func ftpEntries(path string, entries []*goftp.Entry) []RemoteFileEntry {
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
	return result
}

// ftpLister is a search worker's own FTP connection (jlaffaye/ftp can't run
// commands concurrently on one connection - same reason transfers dial
// their own). Opened once per search and reused for every folder.
type ftpLister struct {
	c    *FTPClient
	conn *goftp.ServerConn
}

func (c *FTPClient) OpenLister() (Lister, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, err
	}
	return &ftpLister{c: c, conn: conn}, nil
}

// ListDir reconnects and retries once if the server dropped this connection,
// like FTPClient.control does for the main one.
func (l *ftpLister) ListDir(path string) ([]RemoteFileEntry, error) {
	entries, err := l.conn.List(path)
	if isConnLost(err) {
		if conn, derr := l.c.dial(); derr == nil {
			go l.conn.Quit()
			l.conn = conn
			entries, err = l.conn.List(path)
		}
	}
	if err != nil {
		return nil, err
	}
	return ftpEntries(path, entries), nil
}

func (l *ftpLister) Close() error {
	return l.conn.Quit()
}

func (c *FTPClient) MkDir(path string) error {
	return c.control(func(conn *goftp.ServerConn) error {
		return conn.MakeDir(path)
	})
}

func (c *FTPClient) Delete(path string) error {
	return c.control(func(conn *goftp.ServerConn) error {
		err := conn.Delete(path)
		if err == nil {
			return nil
		}
		if isConnLost(err) {
			return err // let control() reconnect and retry
		}
		return conn.RemoveDirRecur(path) // not a file: remove it as a directory
	})
}

func (c *FTPClient) Rename(oldPath, newPath string) error {
	return c.control(func(conn *goftp.ServerConn) error {
		return conn.Rename(oldPath, newPath)
	})
}

func (c *FTPClient) CurrentDir() (string, error) {
	var dir string
	err := c.control(func(conn *goftp.ServerConn) error {
		var derr error
		dir, derr = conn.CurrentDir()
		return derr
	})
	return dir, err
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
