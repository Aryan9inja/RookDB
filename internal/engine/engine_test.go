package engine

import (
	"errors"
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

	t.Run("get missing key", func(t *testing.T) {
		engine := newTestEngine(t)

		key := "missing"
		got, err := engine.Get(key)
		if !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("engine test: get missing key: expected error to be %v, got %v", ErrKeyNotFound, err)
		}
		if got != "" {
			t.Fatalf("engine test: get missing key: expected empty string but got %v", got)
		}
	})

	t.Run("set overwrite existing value", func(t *testing.T) {
		engine := newTestEngine(t)

		key := "name"
		value := "Aryan"
		newValue := "Ninja"

		if err := engine.Set(key, value); err != nil {
			t.Fatalf("engine test: set overwrite existing value: Set1: %v", err)
		}

		if err := engine.Set(key, newValue); err != nil {
			t.Fatalf("engine test: set overwrite existing value: Set1: %v", err)
		}

		got, err := engine.Get(key)
		if err != nil {
			t.Fatalf("engine test: set overwrite existing value: Get: %v", err)
		}
		if got != newValue {
			t.Fatalf("Expected got to be %v but it is %v", newValue, got)
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
