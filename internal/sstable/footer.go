package sstable

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
)

const footerSize = 36 // bytes with CRC

var ErrCorruptedFooter error = errors.New("corrupted footer")

type footer struct {
	bloomOffset uint64
	bloomSize   uint64
	indexOffset uint64
	indexSize   uint64
}

func newFooter(bOffset, bSize, idxOffset, idxSize uint64) *footer {
	return &footer{
		bloomOffset: bOffset,
		bloomSize:   bSize,
		indexOffset: idxOffset,
		indexSize:   idxSize,
	}
}

func encodeFooter(f *footer) []byte {
	bufferSize := footerSize
	buffer := make([]byte, bufferSize)

	binary.BigEndian.PutUint64(buffer[:8], f.bloomOffset)
	binary.BigEndian.PutUint64(buffer[8:16], f.bloomSize)
	binary.BigEndian.PutUint64(buffer[16:24], f.indexOffset)
	binary.BigEndian.PutUint64(buffer[24:32], f.indexSize)

	checksum := crc32.ChecksumIEEE(buffer[:32])
	binary.BigEndian.PutUint32(buffer[32:], checksum)

	return buffer
}

func decodeFooter(encoded []byte) (*footer, error) {
	if len(encoded) != footerSize {
		return nil, fmt.Errorf("footer length mismatch: %w", ErrCorruptedFooter)
	}

	checksumOffset := 32
	expectedCRC := crc32.ChecksumIEEE(encoded[:checksumOffset])
	actualCRC := binary.BigEndian.Uint32(encoded[checksumOffset:])
	if expectedCRC != actualCRC {
		return nil, fmt.Errorf("footer checksum mismatch: %w", ErrCorruptedFooter)
	}

	bOffset := binary.BigEndian.Uint64(encoded[:8])
	bSize := binary.BigEndian.Uint64(encoded[8:16])
	idxOffset := binary.BigEndian.Uint64(encoded[16:24])
	idxSize := binary.BigEndian.Uint64(encoded[24:32])

	return &footer{
		bloomOffset: bOffset,
		bloomSize:   bSize,
		indexOffset: idxOffset,
		indexSize:   idxSize,
	}, nil
}
