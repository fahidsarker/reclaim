//go:build unix

package exec

import "os"

func removeAll(path string) error {
	return os.RemoveAll(path)
}
