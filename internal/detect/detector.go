package detect

import (
	"io/fs"
	"time"
)

// Confidence indicates how sure a detector is that a directory is a real project.
type Confidence int

const (
	ConfidenceStrong Confidence = iota
	ConfidenceWeak
)

// TargetKind describes what kind of filesystem object a target refers to.
type TargetKind int

const (
	KindDir TargetKind = iota
	KindFile
	KindGlob
)

// Safety marks how aggressive a target is to reclaim.
type Safety int

const (
	SafetyNormal Safety = iota
	SafetyRequiresFlag
)

// Project is a directory positively identified by a detector.
type Project struct {
	Root       string
	Framework  string
	Confidence Confidence
	Manifest   string
	Metadata   map[string]string
	Parent     *Project
}

// Target is a single deletion candidate produced by a detector.
type Target struct {
	Path       string
	RelPath    string
	Kind       TargetKind
	Reason     string
	Regenerate string
	Safety     Safety
	Size       int64
	ModTime    time.Time
}

// Match is the result of a successful detector Detect call.
type Match struct {
	Framework  string
	Confidence Confidence
	Manifest   string
	Targets    []Target
	Metadata   map[string]string
}

// DirCacher memoises filesystem reads for detectors and the walker.
type DirCacher interface {
	ReadDir(dir string) ([]fs.DirEntry, error)
	Stat(path string) (fs.FileInfo, error)
	Lstat(path string) (fs.FileInfo, error)
	ReadFile(path string) ([]byte, error)
}

// Context is passed to detectors; Detect must remain pure and side-effect free.
type Context struct {
	FS    fs.StatFS
	Cache DirCacher
}

// Detector identifies a project framework in a directory.
type Detector interface {
	Name() string
	Priority() int
	Detect(ctx *Context, dir string) (*Match, error)
}
