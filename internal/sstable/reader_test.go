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

	t.Run("test bloomFilter reader", func(t *testing.T) {
		bf, err := reader.bloomFilterReader()
		if err != nil {
			t.Fatalf("bloom filter reader failed: %v", err)
		}

		// Verify all inserted keys (including tombstones) are in the bloom filter
		for i := range 5000 {
			key := strconv.Itoa(i)
			contains, err := bf.maycontain(key)
			if err != nil {
				t.Fatalf("maycontain check failed for key %s: %v", key, err)
			}
			if !contains {
				t.Errorf("expected bloom filter to contain key %s, but it did not", key)
			}
		}

		// Verify keys that were never inserted are mostly reported as absent
		falsePositives := 0
		for i := 5000; i < 6000; i++ {
			key := strconv.Itoa(i)
			contains, err := bf.maycontain(key)
			if err != nil {
				t.Fatalf("maycontain check failed for absent key %s: %v", key, err)
			}
			if contains {
				falsePositives++
			}
		}

		// A false positive rate of 100% on 1000 items would strongly indicate a bug
		if falsePositives == 1000 {
			t.Error("bloom filter returned true for all absent keys (100% false positive rate)")
		}
	})

	t.Run("test indexEntry reader", func(t *testing.T) {
		indexes, err := reader.indexEntryReader()
		if err != nil {
			t.Fatalf("indexEntry reader failed: %v", err)
		}

		if len(indexes) == 0 {
			t.Fatalf("expected non-zero index entries")
		}

		// Since we wrote 5000 records and each block has up to 16 records,
		// the number of blocks (and thus index entries) should be exactly ceil(5000/16) = 313.
		expectedIndexes := 313
		if len(indexes) != expectedIndexes {
			t.Errorf("expected %d index entries, got %d", expectedIndexes, len(indexes))
		}

		if indexes[0].offset != 0 {
			t.Errorf("expected first index offset to be 0, got %d", indexes[0].offset)
		}

		// Ensure offsets are strictly increasing
		var prevOffset uint64 = 0
		for i, index := range indexes {
			if i > 0 {
				if index.offset <= prevOffset {
					t.Errorf("expected index offsets to be strictly increasing, but at %d found offset %d <= %d", i, index.offset, prevOffset)
				}
				if index.key <= indexes[i-1].key {
					t.Errorf("expected index keys to be strictly increasing, but at %d found key %s <= %s", i, index.key, indexes[i-1].key)
				}
			}
			prevOffset = index.offset
		}
	})

	t.Run("test block reader", func(t *testing.T) {
		indexes, err := reader.indexEntryReader()
		if err != nil {
			t.Fatalf("failed to read indexes for block testing: %v", err)
		}

		var totalRecords int
		for i, index := range indexes {
			records, err := reader.blockReader(index.offset)
			if err != nil {
				t.Fatalf("blockReader failed at block %d (offset %d): %v", i, index.offset, err)
			}
			if len(records) == 0 {
				t.Errorf("expected records in block %d, got 0", i)
			}
			totalRecords += len(records)
		}

		if totalRecords != 5000 {
			t.Errorf("expected to read exactly 5000 records across all blocks, but got %d", totalRecords)
		}
	})
}
