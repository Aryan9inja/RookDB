package sstable

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Aryan9inja/RookDB/internal/memtable"
)

func WriteSSTable(mTable *memtable.MemTable, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("absolutePath SSTable writer: %w", err)
	}

	fd, err := os.OpenFile(absPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		return fmt.Errorf("open file: SSTable writer: %w", err)
	}
	defer fd.Close()

	var indexes []indexEntry
	var indexOffset uint64

	it := mTable.Iterator()

	bloom := newBloomFilter(mTable.EntryCount())

	for it.Next() {
		var records []*record

		// Create first record and its index
		firstRec := &record{
			rType: recordType(it.Type()),
			key:   it.Key(),
			value: it.Value(),
		}
		records = append(records, firstRec)

		index := indexEntry{
			key:    it.Key(),
			offset: indexOffset,
		}
		indexes = append(indexes, index)

		bloom.add(it.Key())

		for range 15 {
			if !it.Next() {
				break
			}
			rec := &record{
				rType: recordType(it.Type()),
				key:   it.Key(),
				value: it.Value(),
			}

			records = append(records, rec)

			bloom.add(it.Key())
		}

		block, offsetDelta := createBlock(records)
		indexOffset += uint64(offsetDelta)

		n, err := fd.Write(block)
		if err != nil {
			return fmt.Errorf("write SSTable: %w", err)
		}
		if n < int(offsetDelta) {
			return fmt.Errorf("short write: wrote %d of %d bytes", n, offsetDelta)
		}
	}

	indexBuffer, indexSize := encodeSparseIndex(indexes)
	n, err := fd.Write(indexBuffer)
	if err != nil {
		return fmt.Errorf("write SSTable indexes: %w", err)
	}
	if n < len(indexBuffer) {
		return fmt.Errorf("short write: wrote %d of %d bytes", n, len(indexBuffer))
	}

	bloomOffset := indexOffset + indexSize + 4
	bloomBuffer, bloomSize := encodeBloomFilter(bloom)
	n, err = fd.Write(bloomBuffer)
	if err != nil {
		return fmt.Errorf("write SSTable bloomFilter: %w", err)
	}
	if n < len(bloomBuffer) {
		return fmt.Errorf("short write: wrote %d of %d bytes", n, len(bloomBuffer))
	}

	footer := newFooter(bloomOffset, bloomSize, indexOffset, indexSize)
	footerBuffer := encodeFooter(footer)
	n, err = fd.Write(footerBuffer)
	if err != nil {
		return fmt.Errorf("write SSTable footer: %w", err)
	}
	if n < len(footerBuffer) {
		return fmt.Errorf("short write: wrote %d of %d bytes", n, len(footerBuffer))
	}

	if err := fd.Sync(); err != nil {
		return fmt.Errorf("sync SSTable: %w", err)
	}

	return nil
}
