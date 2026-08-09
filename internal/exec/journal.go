package exec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// JournalRecord is one mutating run appended to history.jsonl.
type JournalRecord struct {
	Timestamp   time.Time        `json:"timestamp"`
	Root        string           `json:"root"`
	ToTrash     bool             `json:"toTrash"`
	Yes         bool             `json:"yes"`
	Interrupted bool             `json:"interrupted,omitempty"`
	Outcomes    []JournalOutcome `json:"outcomes"`
}

// JournalOutcome is the per-target result stored in the journal.
type JournalOutcome struct {
	Path      string `json:"path"`
	Project   string `json:"project,omitempty"`
	Framework string `json:"framework,omitempty"`
	Size      int64  `json:"size"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

func defaultJournalPath() (string, error) {
	if state := os.Getenv("XDG_STATE_HOME"); state != "" {
		return filepath.Join(state, "reclaim", "history.jsonl"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "reclaim", "history.jsonl"), nil
}

func appendJournal(path string, rec JournalRecord) error {
	if path == "" {
		var err error
		path, err = defaultJournalPath()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create journal dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open journal: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	if err := enc.Encode(rec); err != nil {
		return fmt.Errorf("write journal: %w", err)
	}
	return nil
}
