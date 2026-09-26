package engine

import (
	"path/filepath"
	"testing"
)

func TestEngine(t *testing.T) {
	t.Run("set and get", func(t *testing.T) {
		engine := newTestEngine(t)

		key := "name"
		value := "Aryan"

		if err := engine.Set(key, value); err != nil {
			t.Fatalf("engine test: set and get: Set: %v", err)
		}

		got, err := engine.Get(key)
		if err != nil {
			t.Fatalf("engine test: set and get: Get: %v", err)
		}
		if got != value {
			t.Fatalf("Expected got to be %v but it is %v", value, got)
		}
	})
}

func newTestEngine(t *testing.T) *Engine {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "wal.log")

	engine, err := NewEngine(path)
	if err != nil {
		t.Fatalf("setup Engine: %v", err)
	}

	return engine
}
