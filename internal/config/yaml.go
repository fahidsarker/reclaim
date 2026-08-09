package config

import (
	"fmt"

	"github.com/goccy/go-yaml"
)

// unmarshalYAML wraps goccy/go-yaml Unmarshal and converts decoder panics
// on pathological input into errors (fuzz-hardening).
func unmarshalYAML(data []byte, v any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("yaml decode panic: %v", r)
		}
	}()
	return yaml.Unmarshal(data, v)
}
