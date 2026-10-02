package sstable

import (
	"encoding/binary"
	"hash/crc32"
)

func CreateBlock(records []*record) (block []byte, offsetDelta uint32) {
	// Calculate total size of encoded records to preallocate memory
	var dataLen int
	for _, rec := range records {
		dataLen += rec.encodedSize()
	}

	blockSize := 4 + dataLen + 4
	block = make([]byte, 4, blockSize)

	binary.BigEndian.PutUint32(block[:4], uint32(dataLen))

	for _, rec := range records {
		block = append(block, rec.Encode()...)
	}

	// Add the checksum over the length prefix and data
	checksum := crc32.ChecksumIEEE(block)
	block = binary.BigEndian.AppendUint32(block, checksum)

	offsetDelta = uint32(blockSize)
	return
}
