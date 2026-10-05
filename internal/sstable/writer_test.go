package sstable

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Aryan9inja/RookDB/internal/memtable"
)

func TestWriteSSTable(t *testing.T) {
	// create a memtable
	mTable := memtable.NewMemTable()

	// fill memtable with some entries
	for i := range 35 {
		if i%5 == 0 {
			mTable.Delete(strconv.Itoa(i))
		} else {
			mTable.Put(strconv.Itoa(i), strconv.Itoa(i))
		}
	}

	// Create SSTable path
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test_index_writer.sst")

	err := WriteSSTable(mTable, tempFile)
	if err != nil {
		t.Fatalf("WriteSSTable failed: %v", err)
	}

	// Read the generated file
	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("failed to read SSTable file: %v", err)
	}

	if len(data) == 0 {
		t.Errorf("expected file size > 0, got %d", len(data))
	}
}
