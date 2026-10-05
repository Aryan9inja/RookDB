package sstable

import (
	"encoding/binary"
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
