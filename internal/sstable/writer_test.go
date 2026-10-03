package sstable

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
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

	idxOffset, idxSize, err := WriteSSTable(mTable, tempFile)
	if err != nil {
		t.Fatalf("WriteSSTable failed: %v", err)
	}

	if idxOffset == 0 {
		t.Errorf("expected idxOffset > 0, got %d", idxOffset)
	}
	if idxSize == 0 {
		t.Errorf("expected idxSize > 0, got %d", idxSize)
	}

	// Read the generated file
	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("failed to read SSTable file: %v", err)
	}

	// Confirm the index starts exactly at indexOffset
	expectedFileSize := idxOffset + idxSize + 4 // 4 bytes for CRC
	if uint64(len(data)) != expectedFileSize {
		t.Errorf("expected file size %d, got %d", expectedFileSize, len(data))
	}

	// Confirm the index CRC is present/correct
	indexData := data[idxOffset : idxOffset+idxSize]
	expectedCRC := crc32.ChecksumIEEE(indexData)
	actualCRC := binary.BigEndian.Uint32(data[idxOffset+idxSize:])
	if expectedCRC != actualCRC {
		t.Errorf("expected CRC %d, got %d", expectedCRC, actualCRC)
	}

	// Confirm there are multiple blocks and sparse index points to the correct block offsets
	var parsedIndexes []indexEntry
	offset := uint64(0)
	for offset < idxSize {
		keyLen := binary.BigEndian.Uint16(indexData[offset : offset+2])
		key := string(indexData[offset+2 : offset+2+uint64(keyLen)])
		blockOffset := binary.BigEndian.Uint64(indexData[offset+2+uint64(keyLen) : offset+2+uint64(keyLen)+8])
		parsedIndexes = append(parsedIndexes, indexEntry{key: key, offset: blockOffset})
		offset += 2 + uint64(keyLen) + 8
	}

	if len(parsedIndexes) < 2 {
		t.Errorf("expected multiple blocks, got %d", len(parsedIndexes))
	}

	// First block offset should be 0
	if parsedIndexes[0].offset != 0 {
		t.Errorf("expected first block offset to be 0, got %d", parsedIndexes[0].offset)
	}

	// Subsequent blocks should have increasing offsets within the data section
	for i := 1; i < len(parsedIndexes); i++ {
		if parsedIndexes[i].offset <= parsedIndexes[i-1].offset {
			t.Errorf("expected block offset to be strictly increasing: %d <= %d", parsedIndexes[i].offset, parsedIndexes[i-1].offset)
		}
		if parsedIndexes[i].offset >= idxOffset {
			t.Errorf("block offset %d is beyond data section (size %d)", parsedIndexes[i].offset, idxOffset)
		}
	}
}

func TestIndexWriter(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test_index_writer.sst")
	fd, err := os.OpenFile(tempFile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer fd.Close()

	indexes := []indexEntry{
		{key: "key1", offset: 100},
		{key: "key2", offset: 200},
		{key: "long_key_3_hello_world", offset: 300},
	}

	indexSize, err := indexWriter(indexes, fd)
	if err != nil {
		t.Fatalf("indexWriter failed: %v", err)
	}

	// Calculate expected index size
	var expectedSize uint64
	for _, idx := range indexes {
		expectedSize += 2 + uint64(len(idx.key)) + 8
	}

	if indexSize != expectedSize {
		t.Errorf("expected index size %d, got %d", expectedSize, indexSize)
	}

	// Synchronize file before reading
	if err := fd.Sync(); err != nil {
		t.Fatalf("failed to sync file: %v", err)
	}

	// Read the written data
	writtenData, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	// Verify length including checksum
	if uint64(len(writtenData)) != expectedSize+4 {
		t.Errorf("expected written data length %d, got %d", expectedSize+4, len(writtenData))
	}

	// Verify content and checksum
	var expectedBuffer bytes.Buffer
	for _, idx := range indexes {
		buf := make([]byte, 2+len(idx.key)+8)
		binary.BigEndian.PutUint16(buf[0:2], uint16(len(idx.key)))
		copy(buf[2:], idx.key)
		binary.BigEndian.PutUint64(buf[2+len(idx.key):], idx.offset)
		expectedBuffer.Write(buf)
	}

	expectedBytes := expectedBuffer.Bytes()
	expectedChecksum := crc32.ChecksumIEEE(expectedBytes)

	if !bytes.Equal(writtenData[:expectedSize], expectedBytes) {
		t.Errorf("written data does not match expected bytes")
	}

	actualChecksum := binary.BigEndian.Uint32(writtenData[expectedSize:])
	if actualChecksum != expectedChecksum {
		t.Errorf("expected checksum %d, got %d", expectedChecksum, actualChecksum)
	}
}
