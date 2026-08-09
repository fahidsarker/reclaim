package detect

import "sync"

var (
	registryMu sync.RWMutex
	detectors  []Detector
)

// Register adds a detector to the global registry.
// Safe to call from init().
func Register(d Detector) {
	registryMu.Lock()
	defer registryMu.Unlock()
	detectors = append(detectors, d)
}

// Detectors returns a snapshot of registered detectors.
func Detectors() []Detector {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Detector, len(detectors))
	copy(out, detectors)
	return out
}

// DetectBest runs all registered detectors and returns the highest-priority
// match. When several match, targets are unioned and deduplicated by RelPath;
// priority decides Framework and Metadata.
func DetectBest(ctx *Context, dir string) (*Match, error) {
	return DetectBestFiltered(ctx, dir, nil, nil)
}

// DetectBestFiltered is DetectBest with optional framework allow/deny lists
// from a .reclaim.yaml frameworks: block.
func DetectBestFiltered(ctx *Context, dir string, only, disable []string) (*Match, error) {
	registryMu.RLock()
	list := make([]Detector, len(detectors))
	copy(list, detectors)
	registryMu.RUnlock()

	var best *Match
	bestPriority := -1
	seen := map[string]struct{}{}
	var union []Target

	for _, d := range list {
		if !frameworkAllowed(d.Name(), only, disable) {
			continue
		}
		m, err := d.Detect(ctx, dir)
		if err != nil {
			return nil, err
		}
		if m == nil {
			continue
		}
		for _, t := range m.Targets {
			if _, ok := seen[t.RelPath]; ok {
				continue
			}
			seen[t.RelPath] = struct{}{}
			union = append(union, t)
		}
		if best == nil || d.Priority() > bestPriority {
			best = m
			bestPriority = d.Priority()
		} else if d.Priority() == bestPriority && best.Confidence == ConfidenceWeak && m.Confidence == ConfidenceStrong {
			best = m
		}
	}

	if best == nil {
		return nil, nil
	}

	out := *best
	out.Targets = union
	return &out, nil
}

func frameworkAllowed(name string, only, disable []string) bool {
	if len(only) > 0 {
		ok := false
		for _, f := range only {
			if f == name {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	for _, f := range disable {
		if f == name {
			return false
		}
	}
	return true
}
