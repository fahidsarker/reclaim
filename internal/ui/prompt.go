package ui

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Confirm asks for yes/no confirmation. Default is no.
// Accepts "y" or "yes" (case-insensitive). EOF or other input returns false.
func Confirm(in io.Reader, out io.Writer) (bool, error) {
	if _, err := fmt.Fprint(out, "Proceed? [y/N] "); err != nil {
		return false, err
	}
	br := bufio.NewReader(in)
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	switch answer {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
