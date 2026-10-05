package sstable

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"slices"
	"testing"
)

func TestEncodeSparseIndex(t *testing.T) {
	indexes := []indexEntry{
		{key: "key1", offset: 100},
		{key: "key2", offset: 200},
		{key: "long_key_3_hello_world", offset: 300},
	}

	indexBuffer, indexSize := encodeSparseIndex(indexes)

	// Calculate expected index size
	var expectedSize uint64
	for _, idx := range indexes {
		expectedSize += 2 + uint64(len(idx.key)) + 8
	}

	if indexSize != expectedSize {
		t.Errorf("expected index size %d, got %d", expectedSize, indexSize)
	}

	// Verify length including checksum
	if uint64(len(indexBuffer)) != expectedSize+4 {
		t.Errorf("expected index data length %d, got %d", expectedSize+4, len(indexBuffer))
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

	if !bytes.Equal(indexBuffer[:expectedSize], expectedBytes) {
		t.Errorf("written data does not match expected bytes")
	}

	actualChecksum := binary.BigEndian.Uint32(indexBuffer[expectedSize:])
	if actualChecksum != expectedChecksum {
		t.Errorf("expected checksum %d, got %d", expectedChecksum, actualChecksum)
	}
}

func TestDecodeSparseIndex(t *testing.T) {
	indexes := []indexEntry{
		{key: "key1", offset: 100},
		{key: "key2", offset: 200},
		{key: "long_key_3_hello_world", offset: 300},
	}

	indexBuffer, indexSize := encodeSparseIndex(indexes)

	got, err := decodeSparseIndex(indexBuffer, indexSize)
	if err != nil {
		t.Errorf("decodeSparseIndex failed: %v", err)
	}

	if !slices.Equal(indexes, got) {
		t.Fatalf("after encoding and then decoding indexes differ, got %v, expected %v", got, indexes)
	}
}
