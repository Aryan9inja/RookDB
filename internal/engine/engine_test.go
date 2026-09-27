package engine

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestEngine(t *testing.T) {
	t.Run("set and get", func(t *testing.T) {
		engine := newTestEngine(t)
		defer engine.Close()

		key := "name"
		value := "Aryan"

		if err := engine.set(key, value); err != nil {
			t.Fatalf("engine test: set and get: set: %v", err)
		}

		got, err := engine.get(key)
		if err != nil {
			t.Fatalf("engine test: set and get: get: %v", err)
		}
		if got != value {
			t.Fatalf("Expected got to be %v but it is %v", value, got)
		}
	})

	t.Run("get missing key", func(t *testing.T) {
		engine := newTestEngine(t)
		defer engine.Close()

		key := "missing"
		got, err := engine.get(key)
		if !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("engine test: get missing key: expected error to be %v, got %v", ErrKeyNotFound, err)
		}
		if got != "" {
			t.Fatalf("engine test: get missing key: expected empty string but got %v", got)
		}
	})

	t.Run("set overwrite existing value", func(t *testing.T) {
		engine := newTestEngine(t)
		defer engine.Close()

		key := "name"
		value := "Aryan"
		newValue := "Ninja"

		if err := engine.set(key, value); err != nil {
			t.Fatalf("engine test: set overwrite existing value: set1: %v", err)
		}

		if err := engine.set(key, newValue); err != nil {
			t.Fatalf("engine test: set overwrite existing value: set2: %v", err)
		}

		got, err := engine.get(key)
		if err != nil {
			t.Fatalf("engine test: set overwrite existing value: get: %v", err)
		}
		if got != newValue {
			t.Fatalf("Expected got to be %v but it is %v", newValue, got)
		}
	})

	t.Run("delete existing value", func(t *testing.T) {
		engine := newTestEngine(t)
		defer engine.Close()

		key := "name"
		value := "Aryan"

		if err := engine.set(key, value); err != nil {
			t.Fatalf("engine test: delete existing value: set: %v", err)
		}

		if err := engine.delete(key); err != nil {
			t.Fatalf("engine test: delete existing value: delete: %v", err)
		}

		if _, err := engine.get(key); !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("engine test: delete existing value: get: expected error to be %v, got %v", ErrKeyNotFound, err)
		}
	})

	t.Run("delete missing key", func(t *testing.T) {
		engine := newTestEngine(t)
		defer engine.Close()

		key := "missing"
		if err := engine.delete(key); err != nil {
			t.Fatalf("engine test: delete missing key: expected no error got: %v", err)
		}
	})

	t.Run("persistance across restarts", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "wal.log")

		engine1 := newTestEngineAtPath(t, path)

		key := "name"
		value := "Aryan"

		if err := engine1.set(key, value); err != nil {
			t.Fatalf("engine test: persistence across restarts: set: %v", err)
		}

		if err := engine1.Close(); err != nil {
			t.Fatalf("engine test: persistence across restarts: Close: %v", err)
		}

		engine2 := newTestEngineAtPath(t, path)
		defer engine2.Close()

		got, err := engine2.get(key)
		if err != nil {
			t.Fatalf("engine test: persistence across restarts: get: %v", err)
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
