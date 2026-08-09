package ui_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fahid/reclaim/internal/ui"
)

func TestConfirm(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"yes", "y\n", true},
		{"YES", "YES\n", true},
		{"yes word", "yes\n", true},
		{"no", "n\n", false},
		{"empty", "\n", false},
		{"garbage", "maybe\n", false},
		{"eof", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			ok, err := ui.Confirm(strings.NewReader(tt.in), &out)
			if err != nil {
				t.Fatal(err)
			}
			if ok != tt.want {
				t.Fatalf("got %v want %v", ok, tt.want)
			}
			if !strings.Contains(out.String(), "Proceed?") {
				t.Fatalf("missing prompt: %q", out.String())
			}
		})
	}
}
