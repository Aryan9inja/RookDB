package sstable

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Aryan9inja/RookDB/internal/memtable"
)

func TestSSTableReader(t *testing.T) {
	mTable := memtable.NewMemTable()

	for i := range 5000 {
		if i%5 == 0 {
			mTable.Delete(strconv.Itoa(i))
		} else {
			mTable.Put(strconv.Itoa(i), strconv.Itoa(i))
		}
	}

	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.sst")

	err := WriteSSTable(mTable, tempFile)
	if err != nil {
		t.Fatalf("WriteSSTable failed: %v", err)
	}

	reader, err := NewSSTableReader(tempFile)
	if err != nil {
		t.Fatalf("NewSSTableReader failed: %v", err)
	}

	t.Run("test footer reader", func(t *testing.T) {
		if err := reader.footerReader(); err != nil {
			t.Fatalf("footer reader failed: %v", err)
		}
		if reader.foot == nil {
			t.Fatal("expected footer to be populated, but it was nil")
		}
		if reader.foot.bloomSize == 0 {
			t.Error("expected non-zero bloom size in footer")
		}
		if reader.foot.indexSize == 0 {
			t.Error("expected non-zero index size in footer")
		}
		// bloomOffset should definitely be greater than 0 given we wrote 5000 items
		if reader.foot.bloomOffset == 0 {
			t.Error("expected non-zero bloom offset in footer")
		}
	})
}
