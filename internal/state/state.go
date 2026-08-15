// Package state persists the previous sample so cumulative counters can be
// reported as a delta "since last check" across one-shot runs.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Snapshot is the subset of a report persisted between runs.
type Snapshot struct {
	EvictedKeys         int64     `json:"evicted_keys"`
	RejectedConnections int64     `json:"rejected_connections"`
	SampledAt           time.Time `json:"sampled_at"`
}

// Store reads and writes a Snapshot as JSON at Path. An empty Path disables
// persistence (Load returns nil, Save is a no-op) — useful for daemon mode,
// which keeps the previous sample in memory instead.
type Store struct {
	Path string
}

// Load returns the stored snapshot, or nil if Path is empty or the file does
// not exist yet (the first run has no baseline).
func (s Store) Load() (*Snapshot, error) {
	if s.Path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

// Save writes the snapshot, creating the parent directory if needed.
func (s Store) Save(snap Snapshot) error {
	if s.Path == "" {
		return nil
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(s.Path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(s.Path, data, 0o644)
}
