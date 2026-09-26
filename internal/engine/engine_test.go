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
			t.Fatalf("engine test: set overwrite existing value: Set2: %v", err)
		}

		got, err := engine.Get(key)
		if err != nil {
			t.Fatalf("engine test: set overwrite existing value: Get: %v", err)
		}
		if got != newValue {
			t.Fatalf("Expected got to be %v but it is %v", newValue, got)
		}
	})

	t.Run("delete existing value", func(t *testing.T) {
		engine := newTestEngine(t)

		key := "name"
		value := "Aryan"

		if err := engine.Set(key, value); err != nil {
			t.Fatalf("engine test: delete existing value: Set: %v", err)
		}

		if err := engine.Delete(key); err != nil {
			t.Fatalf("engine test: delete existing value: Delete: %v", err)
		}

		if _, err := engine.Get(key); !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("engine test: delete existing value: Get: expected error to be %v, got %v", ErrKeyNotFound, err)
		}
	})

	t.Run("delete missing key", func(t *testing.T) {
		engine := newTestEngine(t)

		key := "missing"
		if err := engine.Delete(key); err != nil {
			t.Fatalf("engine test: delete missing key: expected no error got: %v", err)
		}
	})

	t.Run("persistence across restarts", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "wal.log")

		engine1 := newTestEngineAtPath(t, path)

		key := "name"
		value := "Aryan"

		if err := engine1.Set(key, value); err != nil {
			t.Fatalf("engine test: persistence across restarts: Set: %v", err)
		}

		if err := engine1.Close(); err != nil {
			t.Fatalf("engine test: persistence across restarts: Close: %v", err)
		}

		engine2 := newTestEngineAtPath(t, path)
		defer engine2.Close()

		got, err := engine2.Get(key)
		if err != nil {
			t.Fatalf("engine test: persistence across restarts: Get: %v", err)
		}

		if got != value {
			t.Fatalf("engine test: persistence across restarts: expected %q, got %q", value, got)
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

func newTestEngineAtPath(t *testing.T, path string) *Engine {
	t.Helper()

	engine, err := NewEngine(path)
	if err != nil {
		t.Fatalf("setup Engine: %v", err)
	}

	return engine
}
