package builtin

import "github.com/fahid/reclaim/internal/detect"

// RegisterAll registers Tier-2 Go detectors. Safe to call multiple times from tests
// after detect.ResetRegistryForTest (duplicates are not deduped — call once per reset).
func RegisterAll() {
	detect.Register(&RustDetector{})
	detect.Register(&BazelDetector{})
}

func init() {
	RegisterAll()
}
