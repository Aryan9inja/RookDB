package sstable

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

type indexEntry struct {
	key    string
	offset uint64
}

// return byte representation of sparse indexes
// also return size which is without crc size
func encodeSparseIndex(indexes []indexEntry) ([]byte, uint64) {
	var indexBuffer []byte
	var indexSize uint64

	for _, index := range indexes {
		// [key_length][key][key_offset]
		recordLen := 2 + len(index.key) + 8
		buf := make([]byte, recordLen)

		binary.BigEndian.PutUint16(buf[0:2], uint16(len(index.key)))
		copy(buf[2:], index.key)
		binary.BigEndian.PutUint64(buf[2+len(index.key):], index.offset)

		indexSize += uint64(recordLen)
		indexBuffer = append(indexBuffer, buf...)
	}

	checksum := crc32.ChecksumIEEE(indexBuffer)
	checksumBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(checksumBuf, checksum)

	indexBuffer = append(indexBuffer, checksumBuf...)

	return indexBuffer, indexSize
}

func decodeSparseIndex(encoded []byte, size uint64) ([]indexEntry, error) {
	if uint64(len(encoded))-4 != size {
		return nil, fmt.Errorf("decode: sparse index: size corruption")
	}

	expectedCRC := crc32.ChecksumIEEE(encoded[:size])
	actualCRC := binary.BigEndian.Uint32(encoded[size:])
	if actualCRC != expectedCRC {
		return nil, fmt.Errorf("decode: sparse index data corrupted: failed CRC check")
	}

	var indexes []indexEntry
	var ptr uint64

	for ptr < size {
		if ptr+2 > size {
			return nil, fmt.Errorf("decode: sparse index data corrupted: unexpected EOF")
		}
		keyLen := uint64(binary.BigEndian.Uint16(encoded[ptr : ptr+2]))
		ptr += 2

		if ptr+keyLen+8 > size {
			return nil, fmt.Errorf("decode: sparse index data corrupted: unexpected EOF")
		}
		key := string(encoded[ptr : ptr+keyLen])
		ptr += keyLen

		valOffset := binary.BigEndian.Uint64(encoded[ptr : ptr+8])
		ptr += 8

		indexes = append(indexes, indexEntry{
			key:    key,
			offset: valOffset,
		})
	}

	return indexes, nil
}
