package watch

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestShouldIgnore(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		".git/config":            true,
		".fkn/last-guard.json":   true,
		"bin/fkn":                true,
		"README.md":              false,
	}

	for path, want := range cases {
		if got := shouldIgnore(path); got != want {
			t.Fatalf("shouldIgnore(%q) = %t, want %t", path, got, want)
		}
	}
}

func TestChangedDetectsDifferences(t *testing.T) {
	t.Parallel()

	before := snapshot{"a": {modTime: time.Unix(1, 0), size: 1}}
	after := snapshot{"a": {modTime: time.Unix(2, 0), size: 1}}
	if !changed(before, after) {
		t.Fatal("changed() = false, want true")
	}
}

func TestRunTriggersOnFileChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watched.txt")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := New(dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	triggerCount := 0
	done := make(chan struct{})

	go func() {
		_ = r.Run(ctx, Options{
			Paths:    []string{"watched.txt"},
			Poll:     20 * time.Millisecond,
			Debounce: 30 * time.Millisecond,
			OnTrigger: func(time.Time) error {
				mu.Lock()
				defer mu.Unlock()
				triggerCount++
				if triggerCount == 2 {
					close(done)
					cancel()
				}
				return nil
			},
		})
	}()

	time.Sleep(60 * time.Millisecond)
	if err := os.WriteFile(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not retrigger after file change")
	}
}
