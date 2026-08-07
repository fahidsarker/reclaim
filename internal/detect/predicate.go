package detect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/goccy/go-yaml"
)

const maxFileRead = 1 << 20 // 1 MiB

// predResult is the outcome of evaluating a predicate.
type predResult int

const (
	predPass predResult = iota
	predFail
	predMissing // required path does not exist
	predParse   // file exists but could not be parsed
)

// Predicate is a declarative detect/when condition.
type Predicate struct {
	FileExists   string
	DirExists    string
	GlobExists   string
	FileContains *fileContainsPred
	JSONPath     *pathPred
	YAMLPath     *pathPred
	TOMLPath     *pathPred
	Any          []Predicate
	All          []Predicate
	Not          *Predicate
}

type fileContainsPred struct {
	File    string `yaml:"file"`
	Pattern string `yaml:"pattern"`
}

type pathPred struct {
	File string `yaml:"file"`
	Path string `yaml:"path"`
}

// UnmarshalYAML parses a single-key predicate node.
func (p *Predicate) UnmarshalYAML(data []byte) error {
	var raw map[string]yaml.RawMessage
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("predicate: %w", err)
	}
	if len(raw) != 1 {
		return fmt.Errorf("predicate: expected exactly one key, got %d", len(raw))
	}
	for key, val := range raw {
		switch key {
		case "file_exists":
			return yaml.Unmarshal(val, &p.FileExists)
		case "dir_exists":
			return yaml.Unmarshal(val, &p.DirExists)
		case "glob_exists":
			return yaml.Unmarshal(val, &p.GlobExists)
		case "file_contains":
			p.FileContains = &fileContainsPred{}
			return yaml.Unmarshal(val, p.FileContains)
		case "json_path":
			p.JSONPath = &pathPred{}
			return yaml.Unmarshal(val, p.JSONPath)
		case "yaml_path":
			p.YAMLPath = &pathPred{}
			return yaml.Unmarshal(val, p.YAMLPath)
		case "toml_path":
			p.TOMLPath = &pathPred{}
			return yaml.Unmarshal(val, p.TOMLPath)
		case "any":
			return yaml.Unmarshal(val, &p.Any)
		case "all":
			return yaml.Unmarshal(val, &p.All)
		case "not":
			p.Not = &Predicate{}
			return yaml.Unmarshal(val, p.Not)
		default:
			return fmt.Errorf("predicate: unknown key %q", key)
		}
	}
	return nil
}

func (p *Predicate) validate(path string) error {
	if p == nil {
		return fmt.Errorf("%s: nil predicate", path)
	}
	n := 0
	if p.FileExists != "" {
		n++
	}
	if p.DirExists != "" {
		n++
	}
	if p.GlobExists != "" {
		n++
	}
	if p.FileContains != nil {
		n++
		if p.FileContains.File == "" || p.FileContains.Pattern == "" {
			return fmt.Errorf("%s.file_contains: file and pattern required", path)
		}
		if _, err := regexp.Compile(p.FileContains.Pattern); err != nil {
			return fmt.Errorf("%s.file_contains.pattern: %w", path, err)
		}
	}
	if p.JSONPath != nil {
		n++
		if p.JSONPath.File == "" {
			return fmt.Errorf("%s.json_path: file required", path)
		}
	}
	if p.YAMLPath != nil {
		n++
		if p.YAMLPath.File == "" {
			return fmt.Errorf("%s.yaml_path: file required", path)
		}
	}
	if p.TOMLPath != nil {
		n++
		if p.TOMLPath.File == "" {
			return fmt.Errorf("%s.toml_path: file required", path)
		}
	}
	if len(p.Any) > 0 {
		n++
		for i := range p.Any {
			if err := p.Any[i].validate(fmt.Sprintf("%s.any[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	if len(p.All) > 0 {
		n++
		for i := range p.All {
			if err := p.All[i].validate(fmt.Sprintf("%s.all[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	if p.Not != nil {
		n++
		if err := p.Not.validate(path + ".not"); err != nil {
			return err
		}
	}
	if n != 1 {
		return fmt.Errorf("%s: expected exactly one predicate form, got %d", path, n)
	}
	return nil
}

func (p *Predicate) eval(ctx *Context, dir string) predResult {
	switch {
	case p.FileExists != "":
		return existsFile(ctx, filepath.Join(dir, p.FileExists))
	case p.DirExists != "":
		return existsDir(ctx, filepath.Join(dir, p.DirExists))
	case p.GlobExists != "":
		return existsGlob(ctx, dir, p.GlobExists)
	case p.FileContains != nil:
		return evalFileContains(ctx, dir, p.FileContains)
	case p.JSONPath != nil:
		return evalJSONPath(ctx, dir, p.JSONPath)
	case p.YAMLPath != nil:
		return evalYAMLPath(ctx, dir, p.YAMLPath)
	case p.TOMLPath != nil:
		return evalTOMLPath(ctx, dir, p.TOMLPath)
	case len(p.Any) > 0:
		sawParse := false
		for i := range p.Any {
			switch p.Any[i].eval(ctx, dir) {
			case predPass:
				return predPass
			case predParse:
				sawParse = true
			}
		}
		if sawParse {
			return predParse
		}
		return predFail
	case len(p.All) > 0:
		sawParse := false
		for i := range p.All {
			switch r := p.All[i].eval(ctx, dir); r {
			case predPass:
				continue
			case predParse:
				sawParse = true
			default:
				return r
			}
		}
		if sawParse {
			return predParse
		}
		return predPass
	case p.Not != nil:
		switch p.Not.eval(ctx, dir) {
		case predPass:
			return predFail
		case predParse:
			return predParse
		default:
			return predPass
		}
	default:
		return predFail
	}
}

func existsFile(ctx *Context, path string) predResult {
	info, err := lstat(ctx, path)
	if err != nil {
		return predMissing
	}
	if info.IsDir() {
		return predFail
	}
	return predPass
}

func existsDir(ctx *Context, path string) predResult {
	info, err := lstat(ctx, path)
	if err != nil {
		return predMissing
	}
	if !info.IsDir() {
		return predFail
	}
	return predPass
}

func existsGlob(ctx *Context, dir, pattern string) predResult {
	matches, err := doublestar.Glob(os.DirFS(dir), pattern)
	if err != nil || len(matches) == 0 {
		return predMissing
	}
	_ = ctx
	return predPass
}

func evalFileContains(ctx *Context, dir string, fc *fileContainsPred) predResult {
	data, err := readFileCapped(ctx, filepath.Join(dir, fc.File))
	if err != nil {
		if os.IsNotExist(err) {
			return predMissing
		}
		return predFail
	}
	re, err := regexp.Compile(fc.Pattern)
	if err != nil {
		return predFail
	}
	if re.Match(data) {
		return predPass
	}
	return predFail
}

func evalJSONPath(ctx *Context, dir string, pp *pathPred) predResult {
	data, err := readFileCapped(ctx, filepath.Join(dir, pp.File))
	if err != nil {
		if os.IsNotExist(err) {
			return predMissing
		}
		return predFail
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return predParse
	}
	if isRootPath(pp.Path) {
		return predPass
	}
	if lookupPath(root, pp.Path) {
		return predPass
	}
	return predFail
}

func evalYAMLPath(ctx *Context, dir string, pp *pathPred) predResult {
	data, err := readFileCapped(ctx, filepath.Join(dir, pp.File))
	if err != nil {
		if os.IsNotExist(err) {
			return predMissing
		}
		return predFail
	}
	var root any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return predParse
	}
	if isRootPath(pp.Path) {
		return predPass
	}
	if lookupPath(root, pp.Path) {
		return predPass
	}
	return predFail
}

func evalTOMLPath(ctx *Context, dir string, pp *pathPred) predResult {
	data, err := readFileCapped(ctx, filepath.Join(dir, pp.File))
	if err != nil {
		if os.IsNotExist(err) {
			return predMissing
		}
		return predFail
	}
	var root map[string]any
	if err := toml.Unmarshal(data, &root); err != nil {
		return predParse
	}
	if isRootPath(pp.Path) {
		return predPass
	}
	if lookupPath(root, pp.Path) {
		return predPass
	}
	return predFail
}

func isRootPath(path string) bool {
	return path == "" || path == "." || path == "$"
}

func lookupPath(root any, path string) bool {
	parts := strings.Split(path, ".")
	cur := root
	for _, part := range parts {
		if part == "" {
			continue
		}
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[part]
			if !ok {
				return false
			}
			cur = v
		case map[any]any:
			v, ok := node[part]
			if !ok {
				return false
			}
			cur = v
		default:
			return false
		}
	}
	return cur != nil
}

func lstat(ctx *Context, path string) (os.FileInfo, error) {
	if ctx != nil && ctx.Cache != nil {
		return ctx.Cache.Lstat(path)
	}
	return os.Lstat(path)
}

func readFileCapped(ctx *Context, path string) ([]byte, error) {
	var data []byte
	var err error
	if ctx != nil && ctx.Cache != nil {
		data, err = ctx.Cache.ReadFile(path)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	if len(data) > maxFileRead {
		data = data[:maxFileRead]
	}
	// Trim BOM if present for JSON/YAML comfort.
	return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}), nil
}
