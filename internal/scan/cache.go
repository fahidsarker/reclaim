package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// DirCache memoises readdir, stat, lstat, and file reads per path.
type DirCache struct {
	mu       sync.RWMutex
	readdir  map[string]readdirResult
	stat     map[string]statResult
	lstat    map[string]statResult
	readFile map[string]readFileResult
}

type readdirResult struct {
	entries []fs.DirEntry
	err     error
}

type statResult struct {
	info fs.FileInfo
	err  error
}

type readFileResult struct {
	data []byte
	err  error
}

// NewDirCache creates an empty directory cache.
func NewDirCache() *DirCache {
	return &DirCache{
		readdir:  make(map[string]readdirResult),
		stat:     make(map[string]statResult),
		lstat:    make(map[string]statResult),
		readFile: make(map[string]readFileResult),
	}
}

// ReadDir returns cached directory entries for dir.
func (c *DirCache) ReadDir(dir string) ([]fs.DirEntry, error) {
	c.mu.RLock()
	if r, ok := c.readdir[dir]; ok {
		c.mu.RUnlock()
		return r.entries, r.err
	}
	c.mu.RUnlock()

	entries, err := os.ReadDir(dir)

	c.mu.Lock()
	c.readdir[dir] = readdirResult{entries: entries, err: err}
	c.mu.Unlock()
	return entries, err
}

// Stat returns cached FileInfo for path (follows symlinks).
func (c *DirCache) Stat(path string) (fs.FileInfo, error) {
	c.mu.RLock()
	if r, ok := c.stat[path]; ok {
		c.mu.RUnlock()
		return r.info, r.err
	}
	c.mu.RUnlock()

	info, err := os.Stat(path)

	c.mu.Lock()
	c.stat[path] = statResult{info: info, err: err}
	c.mu.Unlock()
	return info, err
}

// Lstat returns cached FileInfo for path (does not follow symlinks).
func (c *DirCache) Lstat(path string) (fs.FileInfo, error) {
	c.mu.RLock()
	if r, ok := c.lstat[path]; ok {
		c.mu.RUnlock()
		return r.info, r.err
	}
	c.mu.RUnlock()

	info, err := os.Lstat(path)

	c.mu.Lock()
	c.lstat[path] = statResult{info: info, err: err}
	c.mu.Unlock()
	return info, err
}

// ReadFile returns cached file contents for path.
func (c *DirCache) ReadFile(path string) ([]byte, error) {
	c.mu.RLock()
	if r, ok := c.readFile[path]; ok {
		c.mu.RUnlock()
		return r.data, r.err
	}
	c.mu.RUnlock()

	data, err := os.ReadFile(path)

	c.mu.Lock()
	c.readFile[path] = readFileResult{data: data, err: err}
	c.mu.Unlock()
	return data, err
}

// Abs cleans and absolutises path.
func Abs(path string) (string, error) {
	return filepath.Abs(path)
}
