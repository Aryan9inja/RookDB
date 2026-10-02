package sstable

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

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