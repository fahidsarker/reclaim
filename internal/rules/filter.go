package rules

// FrameworkAllowed reports whether framework passes include/exclude filters.
// Phase 3 has no CLI framework filters; always true until Phase 8 wires --framework.
func FrameworkAllowed(framework string, include, exclude []string) bool {
	if len(include) > 0 {
		ok := false
		for _, f := range include {
			if f == framework {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	for _, f := range exclude {
		if f == framework {
			return false
		}
	}
	return true
}
