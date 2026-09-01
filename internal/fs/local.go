package fs

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// searchResultLimit caps how many matches Search returns, to keep a
// recursive search over a huge tree from hanging the UI.
const searchResultLimit = 500

// winEnvPattern matches Windows-style %VAR% environment variable references.
var winEnvPattern = regexp.MustCompile(`%([^%]+)%`)

// ExpandPath resolves a leading ~ and environment variable references
// (Windows %VAR% and Unix $VAR / ${VAR}) in a user-typed path, such as the
// "Default local directory" setting or the editable local path bar. It does
// not touch the filesystem, so it is safe to call on any string. A reference
// to an unset variable is left untouched rather than collapsed to "".
func ExpandPath(path string) string {
	if path == "" {
		return path
	}
	if path == "~" {
		return HomeDir()
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home := HomeDir(); home != "" {
			path = filepath.Join(home, path[2:])
		}
	}
	path = winEnvPattern.ReplaceAllStringFunc(path, func(match string) string {
		name := match[1 : len(match)-1]
		if v, ok := os.LookupEnv(name); ok {
			return v
		}
		return match
	})
	path = os.Expand(path, func(name string) string {
		if v, ok := os.LookupEnv(name); ok {
			return v
		}
		return "$" + name
	})
	return path
}

type FileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	Mode    string    `json:"mode"`
}

func ListDir(dir string, showHidden bool) ([]FileEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var result []FileEntry
	for _, e := range entries {
		if !showHidden && len(e.Name()) > 0 && e.Name()[0] == '.' {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, FileEntry{
			Name:    e.Name(),
			Path:    filepath.Join(dir, e.Name()),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Mode:    info.Mode().String(),
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

// Search looks for entries whose name contains query (case-insensitive)
// inside dir. When recursive is true it also descends into every
// subdirectory; otherwise it only looks at dir's direct children.
func Search(dir, query string, recursive, showHidden bool) ([]FileEntry, error) {
	q := strings.ToLower(query)
	var result []FileEntry
	err := searchWalk(dir, q, recursive, showHidden, &result)
	if err != nil && len(result) == 0 {
		return nil, err
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func searchWalk(dir, q string, recursive, showHidden bool, result *[]FileEntry) error {
	if len(*result) >= searchResultLimit {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if len(*result) >= searchResultLimit {
			return nil
		}
		hidden := len(e.Name()) > 0 && e.Name()[0] == '.'
		if !showHidden && hidden {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		fullPath := filepath.Join(dir, e.Name())
		if strings.Contains(strings.ToLower(e.Name()), q) {
			*result = append(*result, FileEntry{
				Name:    e.Name(),
				Path:    fullPath,
				IsDir:   e.IsDir(),
				Size:    info.Size(),
				ModTime: info.ModTime(),
				Mode:    info.Mode().String(),
			})
		}
		if recursive && e.IsDir() {
			_ = searchWalk(fullPath, q, recursive, showHidden, result)
		}
	}
	return nil
}

func MkDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func Delete(path string) error {
	return os.RemoveAll(path)
}

func Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func Copy(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := Copy(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()
	dstF, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstF.Close()
	_, err = io.Copy(dstF, srcF)
	return err
}

func HomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

func ParentDir(path string) string {
	return filepath.Dir(path)
}

func Roots() []FileEntry {
	home, _ := os.UserHomeDir()
	roots := []FileEntry{
		{Name: "Home", Path: home, IsDir: true},
	}
	// On Linux, add filesystem root
	if _, err := os.Stat("/"); err == nil {
		roots = append(roots, FileEntry{Name: "/ (root)", Path: "/", IsDir: true})
	}
	return roots
}
