package engine

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Aryan9inja/RookDB/internal/memtable"
	"github.com/Aryan9inja/RookDB/internal/operation"
	"github.com/Aryan9inja/RookDB/internal/sstable"
	"github.com/Aryan9inja/RookDB/internal/wal"
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

func TestRecovery(t *testing.T) {
	t.Run("empty data directory creates 000001.wal and empty memtable", func(t *testing.T) {
		testEngine := newTestEngine(t)

		if testEngine.currGen != "000001" {
			t.Fatalf("expected generation to be 000001, instead got: %v", testEngine.currGen)
		}

		if testEngine.memTable.EntryCount() != 0 {
			t.Fatalf("expected empty memtable on empty directory recovery")
		}
	})

	t.Run("directory containing only 000003.sst creates 000004.wal", func(t *testing.T) {
		dir := t.TempDir()
		sstFile := filepath.Join(dir, "000003.sst")

		if _, err := os.Create(sstFile); err != nil {
			t.Fatalf("test setup failed, can't create 000003.sst: %v", err)
		}

		testEngine := newTestEngineAtDir(t, dir)

		if testEngine.currGen != "000004" {
			t.Fatalf("expected generation to be 000004, instead got: %v", testEngine.currGen)
		}
	})

	t.Run("unrecovered wal branch", func(t *testing.T) {
		dir := t.TempDir()

		sstFile := filepath.Join(dir, "000001.sst")
		if _, err := os.Create(sstFile); err != nil {
			t.Fatalf("test setup failed, can't create 000001.sst: %v", err)
		}

		oldWal1_Path := filepath.Join(dir, "000002.wal")
		oldWal1, err := wal.NewWAL(oldWal1_Path)
		if err != nil {
			t.Fatalf("test setup failed, can't create 000002.wal: %v", err)
		}

		oldWal1_ops := []operation.Operation{
			{
				OpType: operation.Set,
				Key:    "name",
				Value:  "Aryan",
			},
			{
				OpType: operation.Set,
				Key:    "age",
				Value:  "22",
			},
		}

		for _, op := range oldWal1_ops {
			if err := wal.Append(oldWal1, op); err != nil {
				t.Fatalf("test setup failed, can't write 000002.wal's operations: %v", err)
			}
		}
		if err := oldWal1.Close(); err != nil {
			t.Fatalf("test setup failed, can't close wal 000002 after write")
		}

		oldWal2_Path := filepath.Join(dir, "000003.wal")
		oldWal2, err := wal.NewWAL(oldWal2_Path)
		if err != nil {
			t.Fatalf("test setup failed, can't create 000003.wal: %v", err)
		}

		oldWal2_ops := []operation.Operation{
			{
				OpType: operation.Set,
				Key:    "location",
				Value:  "world",
			},
			{
				OpType: operation.Set,
				Key:    "true",
				Value:  "false",
			},
		}

		for _, op := range oldWal2_ops {
			if err := wal.Append(oldWal2, op); err != nil {
				t.Fatalf("test setup failed, can't write 000003.wal's operations: %v", err)
			}
		}
		if err := oldWal2.Close(); err != nil {
			t.Fatalf("test setup failed, can't close wal 000003 after write")
		}

		testEngine := newTestEngineAtDir(t, dir)

		keysToGet := []string{"name", "age", "location", "true"}
		expectedRes := []string{"Aryan", "22", "world", "false"}

		for i, key := range keysToGet {
			got, err := testEngine.get(key)
			if err != nil {
				t.Fatalf("error getting key: %v: %v", key, err)
			}

			if got != expectedRes[i] {
				t.Fatalf("expected get to return :%v, got :%v", expectedRes[i], got)
			}
		}

		if testEngine.currGen != "000003" {
			t.Fatalf("expected current gen to be 000003, got: %v", testEngine.currGen)
		}

		// execute a command to check where the wal write happens
		if err := testEngine.set("country", "India"); err != nil {
			t.Fatalf("set failed after recovery: %v", err)
		}

		if err := testEngine.Close(); err != nil {
			t.Fatalf("error closing the engine after set: %v", err)
		}

		walCheck, err := wal.NewWAL(oldWal2_Path)
		if err != nil {
			t.Fatalf("error opening wal file to inspect: %v", err)
		}
		defer walCheck.Close()

		var lastOp *operation.Operation
		for {
			op, err := wal.Next(walCheck)
			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				t.Fatalf("error getting walCheck operations: %v", err)
			}

			lastOp = op
		}

		if lastOp == nil || lastOp.OpType != operation.Set || lastOp.Key != "country" || lastOp.Value != "India" {
			t.Fatalf("unexpected last operation in wal 000003, %v", lastOp)
		}

		// verify wal 000004 is not created
		match, err := filepath.Glob(filepath.Join(dir, "000004.wal"))
		if err != nil {
			t.Fatalf("error checking existence of 000004.wal: %v", err)
		}
		if len(match) != 0 {
			t.Fatalf("expected no file with name 000004.wal to be present but got some")
		}
	})
}

func TestFlushGeneration(t *testing.T) {
	t.Run("successful flush", func(t *testing.T) {
		testEngine := newTestEngine(t)
		defer testEngine.Close()

		testEngine.set("name", "Aryan")
		testEngine.set("age", "22")

		// Just for testing
		// Does not communicate real behaviour
		job := &flushJob{
			generation: testEngine.currGen,
			dataDir:    testEngine.dataDir,
			wal:        testEngine.wal,
			memTable:   testEngine.memTable,
		}

		flushRes := job.flushGeneration()

		if flushRes.err != nil {
			t.Fatalf("expected no error on valid memtable, got: %v", flushRes.err)
		}

		if flushRes.generation != "000001" {
			t.Fatalf("expected flushed generation to be 000001, got: %v", flushRes.generation)
		}

		if flushRes.tempPath != filepath.Join(testEngine.dataDir, "000001.sst.tmp") {
			t.Fatalf("expected temp path for 000001.sst.tmp, got path: %s", flushRes.tempPath)
		}

		if _, err := os.Stat(flushRes.tempPath); err != nil {
			t.Fatalf("expected temporary SSTable to exist: %v", err)
		}
	})

	t.Run("invalid target directory forces WriteSSTable failure", func(t *testing.T) {
		testEngine := newTestEngine(t)
		defer testEngine.Close()

		testEngine.set("name", "Aryan")
		testEngine.set("age", "22")

		job := &flushJob{
			generation: testEngine.currGen,
			dataDir:    filepath.Join(testEngine.dataDir, "nonexistent"),
			wal:        testEngine.wal,
			memTable:   testEngine.memTable,
		}

		flushRes := job.flushGeneration()

		if flushRes.err == nil {
			t.Fatalf("expected error on invalid target directory, got nil")
		}
	})
}

func TestHandleFlushResult(t *testing.T) {
	t.Run("normal publication and WAL reclamation", func(t *testing.T) {
		engine, tempPath := setupFrozenEngine(t)
		defer engine.Close()

		dir := engine.dataDir
		frozenGen := engine.frozenGen
		frozenWal := engine.frozenWal

		sstablesBefore := len(engine.sstables)

		res := flushResult{generation: frozenGen, tempPath: tempPath}
		if err := engine.handleFlushResult(res); err != nil {
			t.Fatalf("scenario 1: handleFlushResult returned unexpected error: %v", err)
		}

		finalPath := filepath.Join(dir, frozenGen+".sst")
		if _, err := os.Stat(finalPath); err != nil {
			t.Errorf("scenario 1: expected final SSTable %q to exist: %v", finalPath, err)
		}

		if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("scenario 1: expected temp file %q to be gone after rename", tempPath)
		}

		if got, want := len(engine.sstables), sstablesBefore+1; got != want {
			t.Errorf("scenario 1: sstables len = %d, want %d", got, want)
		}

		if engine.sstables[0].Name() != frozenGen {
			t.Errorf("scenario 1: sstables[0].Name() = %q, want %q", engine.sstables[0].Name(), frozenGen)
		}

		if engine.frozenGen != "" {
			t.Errorf("scenario 1: frozenGen = %q, want empty", engine.frozenGen)
		}
		if engine.frozenMemTable != nil {
			t.Errorf("scenario 1: frozenMemTable should be nil after reclaim")
		}
		if engine.frozenSSTable != nil {
			t.Errorf("scenario 1: frozenSSTable should be nil after reclaim")
		}
		if engine.frozenWal != nil {
			t.Errorf("scenario 1: frozenWal should be nil after reclaim")
		}

		walPath := filepath.Join(dir, frozenGen+".wal")
		if _, err := os.Stat(walPath); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("scenario 1: expected WAL %q to be removed from disk", walPath)
		}

		if err := frozenWal.Close(); err == nil {
			t.Errorf("scenario 1: expected frozen WAL handle to be closed already (double-close should fail)")
		}
	})

	t.Run("publication failure preserves frozen WAL and MemTable", func(t *testing.T) {
		engine, _ := setupFrozenEngine(t)
		defer engine.Close()

		frozenGen := engine.frozenGen
		frozenWal := engine.frozenWal
		frozenMem := engine.frozenMemTable
		sstablesBefore := len(engine.sstables)

		res := flushResult{
			generation: frozenGen,
			tempPath:   filepath.Join(engine.dataDir, "nonexistent.sst.tmp"),
		}

		err := engine.handleFlushResult(res)
		if err == nil {
			t.Fatal("scenario 2: expected an error when temp file is missing, got nil")
		}

		if engine.frozenWal != frozenWal {
			t.Errorf("scenario 2: frozenWal changed after publish failure")
		}

		if engine.frozenMemTable != frozenMem {
			t.Errorf("scenario 2: frozenMemTable changed after publish failure")
		}

		if engine.frozenSSTable != nil {
			t.Errorf("scenario 2: frozenSSTable should remain nil after publish failure")
		}

		if engine.frozenGen != frozenGen {
			t.Errorf("scenario 2: frozenGen changed from %q to %q", frozenGen, engine.frozenGen)
		}

		if len(engine.sstables) != sstablesBefore {
			t.Errorf("scenario 2: sstables grew from %d to %d on failure", sstablesBefore, len(engine.sstables))
		}
	})

	t.Run("WAL removal failure retains frozen state and published handler", func(t *testing.T) {
		engine, tempPath := setupFrozenEngine(t)
		defer engine.Close()

		frozenGen := engine.frozenGen
		dir := engine.dataDir

		walPath := filepath.Join(dir, frozenGen+".wal")
		if err := engine.frozenWal.Close(); err != nil {
			t.Fatalf("scenario 3: pre-close frozen WAL: %v", err)
		}
		engine.frozenWal = nil

		if err := os.Remove(walPath); err != nil {
			t.Fatalf("scenario 3: remove WAL file before mkdir: %v", err)
		}
		if err := os.Mkdir(walPath, 0755); err != nil {
			t.Fatalf("scenario 3: mkdir WAL path: %v", err)
		}
		// Place a sentinel file inside so the directory is non-empty.
		sentinel := filepath.Join(walPath, "sentinel")
		if f, err := os.Create(sentinel); err != nil {
			t.Fatalf("scenario 3: create sentinel file: %v", err)
		} else {
			f.Close()
		}
		defer os.RemoveAll(walPath)

		sstablesBefore := len(engine.sstables)

		res := flushResult{generation: frozenGen, tempPath: tempPath}
		err := engine.handleFlushResult(res)
		if err == nil {
			t.Fatal("scenario 3: expected error when WAL removal fails, got nil")
		}

		if engine.frozenSSTable == nil {
			t.Errorf("scenario 3: frozenSSTable should be set after successful publication")
		}

		if len(engine.sstables) != sstablesBefore+1 {
			t.Errorf("scenario 3: sstables len = %d, want %d", len(engine.sstables), sstablesBefore+1)
		}

		// frozenGen and frozenMemTable must still be intact (reclaim didn't finish).
		if engine.frozenGen != frozenGen {
			t.Errorf("scenario 3: frozenGen should still be %q, got %q", frozenGen, engine.frozenGen)
		}
		if engine.frozenMemTable == nil {
			t.Errorf("scenario 3: frozenMemTable should still be set")
		}
	})

	t.Run("retry after partial reclaim: no duplicate SSTable, no double WAL close", func(t *testing.T) {
		engine, tempPath := setupFrozenEngine(t)
		defer engine.Close()

		frozenGen := engine.frozenGen
		dir := engine.dataDir

		firstRes := flushResult{generation: frozenGen, tempPath: tempPath}
		if err := engine.publishFrozenSSTable(firstRes); err != nil {
			t.Fatalf("scenario 4: manual publishFrozenSSTable: %v", err)
		}

		sstablesAfterPublish := len(engine.sstables)
		publishedHandler := engine.frozenSSTable

		if err := engine.frozenWal.Close(); err != nil {
			t.Fatalf("scenario 4: close frozen WAL: %v", err)
		}
		engine.frozenWal = nil

		walPath := filepath.Join(dir, frozenGen+".wal")
		if _, err := os.Stat(walPath); errors.Is(err, os.ErrNotExist) {
			if f, err := os.Create(walPath); err == nil {
				f.Close()
			}
		}

		retryRes := flushResult{generation: frozenGen, tempPath: tempPath}
		if err := engine.handleFlushResult(retryRes); err != nil {
			t.Fatalf("scenario 4: retry handleFlushResult returned error: %v", err)
		}

		// Assertion 1 – no duplicate SSTable registered.
		if got, want := len(engine.sstables), sstablesAfterPublish; got != want {
			t.Errorf("scenario 4: sstables len after retry = %d, want %d (no duplicate)", got, want)
		}

		// Assertion 2 – the handler pointer is the same object (not a new one).
		if engine.sstables[0] != publishedHandler {
			t.Errorf("scenario 4: handler pointer changed on retry – a duplicate was added")
		}

		if engine.frozenGen != "" {
			t.Errorf("scenario 4: frozenGen should be empty after successful retry, got %q", engine.frozenGen)
		}
		if engine.frozenMemTable != nil {
			t.Errorf("scenario 4: frozenMemTable should be nil after successful retry")
		}
		if engine.frozenSSTable != nil {
			t.Errorf("scenario 4: frozenSSTable should be nil after successful retry")
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

// setupFrozenEngine creates an engine that already has a frozen generation
// ready for handleFlushResult. It returns the engine with:
//   - frozenGen  = "000001"
//   - frozenWal  / frozenMemTable populated
//   - a valid temp SSTable written at <dir>/000001.sst.tmp
func setupFrozenEngine(t *testing.T) (*Engine, string) {
	t.Helper()

	dir := t.TempDir()
	engine, err := NewEngine(dir)
	if err != nil {
		t.Fatalf("setupFrozenEngine: NewEngine: %v", err)
	}

	if err := engine.set("key", "value"); err != nil {
		t.Fatalf("setupFrozenEngine: set: %v", err)
	}

	if err := engine.CreateNewGeneration(); err != nil {
		t.Fatalf("setupFrozenEngine: CreateNewGeneration: %v", err)
	}

	// Write the temp SSTable that a real flushWorker would have produced.
	tempPath := filepath.Join(dir, "000001.sst.tmp")
	if err := sstable.WriteSSTable(engine.frozenMemTable, tempPath); err != nil {
		t.Fatalf("setupFrozenEngine: WriteSSTable: %v", err)
	}

	return engine, tempPath
}

func TestFlushIntegration(t *testing.T) {
	dir := t.TempDir()

	// Engine A: write and flush
	engine := newTestEngineAtDir(t, dir)

	if err := engine.set("name", "Aryan"); err != nil {
		t.Fatalf("set name: %v", err)
	}

	if err := engine.set("age", "22"); err != nil {
		t.Fatalf("set age: %v", err)
	}

	genBeforeFlush := engine.currGen
	walBeforeFlush := filepath.Join(dir, genBeforeFlush+".wal")
	expectedSSTPath := filepath.Join(dir, genBeforeFlush+".sst")
	expectedTmpPath := filepath.Join(dir, genBeforeFlush+".sst.tmp")

	if err := engine.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if engine.currGen == genBeforeFlush {
		t.Fatalf("generation was not rotated")
	}

	newGen := engine.currGen

	// Close: must drain the pending flush
	if err := engine.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify on-disk flush artefacts
	if _, err := os.Stat(expectedSSTPath); err != nil {
		t.Fatalf("published SSTable missing: %v", err)
	}

	if _, err := os.Stat(expectedTmpPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("temporary SSTable should not exist; stat error: %v", err)
	}

	if _, err := os.Stat(walBeforeFlush); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("frozen WAL should have been reclaimed; stat error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, newGen+".wal")); err != nil {
		t.Errorf("active WAL missing: %v", err)
	}

	// Engine B: recover from the same directory
	recovered := newTestEngineAtDir(t, dir)
	defer recovered.Close()

	for key, want := range map[string]string{
		"name": "Aryan",
		"age":  "22",
	} {
		got, err := recovered.get(key)
		if err != nil {
			t.Errorf("recovery get(%q): %v", key, err)
			continue
		}

		if got != want {
			t.Errorf("recovery get(%q) = %q, want %q", key, got, want)
		}
	}
}
