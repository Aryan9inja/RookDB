package engine

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/Aryan9inja/RookDB/internal/memtable"
	"github.com/Aryan9inja/RookDB/internal/sstable"
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

		engine1 := newTestEngineAtDir(t, dir)

		key := "name"
		value := "Aryan"

		if err := engine1.set(key, value); err != nil {
			t.Fatalf("engine test: persistence across restarts: set: %v", err)
		}

		if err := engine1.Close(); err != nil {
			t.Fatalf("engine test: persistence across restarts: Close: %v", err)
		}

		engine2 := newTestEngineAtDir(t, dir)
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

func TestGetKey(t *testing.T) {
	dir := t.TempDir()

	// create some sstables
	mTable1 := memtable.NewMemTable()
	mTable1.Put("name", "aryan")
	mTable1.Put("age", "22")
	mTable1.Delete("test")

	if err := sstable.WriteSSTable(mTable1, filepath.Join(dir, "000002.sst")); err != nil {
		t.Fatalf("get key test setup failed: write sstable 000002: %v", err)
	}

	mTable2 := memtable.NewMemTable()
	mTable2.Put("test", "yay")

	if err := sstable.WriteSSTable(mTable2, filepath.Join(dir, "000001.sst")); err != nil {
		t.Fatalf("get key test setup failed: write sstable 000001: %v", err)
	}

	engine := newTestEngineAtDir(t, dir)
	if err := engine.delete("name"); err != nil {
		t.Fatalf("get key test setup failed: write memtabe entry 1: %v", err)
	}
	if err := engine.set("check", "what"); err != nil {
		t.Fatalf("get key test setup failed: write memtabe entry 2: %v", err)
	}

	t.Run("key-value present in memtable", func(t *testing.T) {
		target := "check"
		expected := "what"

		got, err := engine.get(target)
		if err != nil {
			t.Fatalf("get key test: key-value present in memtable: %v", err)
		}

		if got != expected {
			t.Fatalf("get key test: key-value present in memtable: wrong value")
		}
	})

	t.Run("tombstone in memtable", func(t *testing.T) {
		target := "name"
		expected := ""

		got, err := engine.get(target)
		if err == nil {
			t.Fatalf("get key test: tombstone in memtable: expected err: %v, got nil", ErrKeyNotFound)
		}
		if !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("get key test: tombstone in memtable: expected err: %v, got %v", ErrKeyNotFound, err)
		}

		if got != expected {
			t.Fatalf("get key test: tombstone in memtable: wrong value")
		}
	})

	t.Run("key value present in sstable", func(t *testing.T) {
		target := "age"
		expected := "22"

		got, err := engine.get(target)
		if err != nil {
			t.Fatalf("get key test: key value present in sstable: %v", err)
		}

		if got != expected {
			t.Fatalf("get key test: key value present in sstable: wrong value")
		}
	})

	t.Run("tombstone in latest sstable", func(t *testing.T) {
		target := "test"
		expected := ""

		got, err := engine.get(target)
		if err == nil {
			t.Fatalf("get key test: tombstone in latest sstable: expected err: %v, got nil", ErrKeyNotFound)
		}
		if !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("get key test: tombstone in latest sstable: expected err: %v, got %v", ErrKeyNotFound, err)
		}

		if got != expected {
			t.Fatalf("get key test: tombstone in latest sstable: wrong value")
		}
	})

	t.Run("missing key in both memtable and sstable", func(t *testing.T) {
		target := "missing"
		expected := ""

		got, err := engine.get(target)
		if err == nil {
			t.Fatalf("get key test: missing key in both memtable and sstable: expected err: %v, got nil", ErrKeyNotFound)
		}
		if !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("get key test: missing key in both memtable and sstable: expected err: %v, got %v", ErrKeyNotFound, err)
		}

		if got != expected {
			t.Fatalf("get key test: missing key in both memtable and sstable: wrong value")
		}
	})
}

func newTestEngine(t *testing.T) *Engine {
	t.Helper()

	dir := t.TempDir()

	engine, err := NewEngine(dir)
	if err != nil {
		t.Fatalf("setup Engine: %v", err)
	}

	return engine
}

func newTestEngineAtDir(t *testing.T, path string) *Engine {
	t.Helper()

	engine, err := NewEngine(path)
	if err != nil {
		t.Fatalf("setup Engine: %v", err)
	}

	return engine
}
