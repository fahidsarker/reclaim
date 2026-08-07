package builtin

import (
	"encoding/json"
	"path/filepath"

	"github.com/fahid/reclaim/internal/detect"
)

func init() {
	detect.Register(&NodeJS{})
}

// NodeJS detects Node.js projects via a readable, parseable package.json.
type NodeJS struct{}

func (n *NodeJS) Name() string     { return "nodejs" }
func (n *NodeJS) Priority() int    { return 10 }

func (n *NodeJS) Detect(ctx *detect.Context, dir string) (*detect.Match, error) {
	manifest := filepath.Join(dir, "package.json")

	var data []byte
	var err error
	if ctx != nil && ctx.Cache != nil {
		data, err = ctx.Cache.ReadFile(manifest)
	} else {
		return nil, nil
	}
	if err != nil {
		return nil, nil
	}

	var pkg map[string]any
	if err := json.Unmarshal(data, &pkg); err != nil {
		// Corrupt manifest: Weak match, no deletion targets.
		return &detect.Match{
			Framework:  "nodejs",
			Confidence: detect.ConfidenceWeak,
			Manifest:   manifest,
			Targets:    nil,
		}, nil
	}

	return &detect.Match{
		Framework:  "nodejs",
		Confidence: detect.ConfidenceStrong,
		Manifest:   manifest,
		Targets: []detect.Target{
			{
				RelPath:    "node_modules",
				Kind:       detect.KindDir,
				Reason:     "Node.js dependency cache",
				Regenerate: "npm install",
				Safety:     detect.SafetyNormal,
			},
		},
	}, nil
}
