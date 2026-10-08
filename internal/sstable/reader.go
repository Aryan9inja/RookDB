package sstable

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
)

type SSTableReader struct {
	fd   *os.File
	foot *footer
}

const maxExpectedBlockSize = 10240 // 10 KB

func newSSTableReader(path string) (*SSTableReader, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening sstable file: %w", err)
	}

	return &SSTableReader{
		fd:   fd,
		foot: nil,
	}, nil
}

func (reader *SSTableReader) close() error {
	if err := reader.fd.Close(); err != nil {
		return fmt.Errorf("error closing sstable file: %w", err)
	}
	return nil
}

func (reader *SSTableReader) get(target string) (string, recordType, bool, error) {
	if err := reader.footerReader(); err != nil {
		return "", 0, false, fmt.Errorf("error reading file footer: %w", err)
	}

	bloom, err := reader.bloomFilterReader()
	if err != nil {
		return "", 0, false, fmt.Errorf("error reading bloom filter: %w", err)
	}

	contains, err := bloom.maycontain(target)
	if err != nil {
		return "", 0, false, fmt.Errorf("error running bloom check: %w", err)
	}

	if !contains {
		return "", 0, false, nil
	}

	indexes, err := reader.indexEntryReader()
	if err != nil {
		return "", 0, false, fmt.Errorf("error reading sparse indexes: %w", err)
	}

	blockIdx, ok := searchIndexes(indexes, target)
	if !ok {
		return "", 0, false, nil
	}

	records, err := reader.blockReader(blockIdx.offset)
	if err != nil {
		return "", 0, false, fmt.Errorf("error reading block: %w", err)
	}

	for _, rec := range records {
		if rec.key == target {
			return rec.value, rec.rType, true, nil
		}
	}

	return "", 0, false, nil
}

func searchIndexes(indexes []indexEntry, target string) (indexEntry, bool) {
	if len(indexes) == 0 {
		return indexEntry{}, false
	}

	low := 0
	high := len(indexes) - 1
	ans := -1

	for low <= high {
		mid := low + (high-low)/2

		if indexes[mid].key <= target {
			ans = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	if ans == -1 {
		return indexEntry{}, false
	}

	return indexes[ans], true
}

func (reader *SSTableReader) footerReader() error {
	info, err := reader.fd.Stat()
	if err != nil {
		return fmt.Errorf("footer reader: file Stat: %w", err)
	}
	fileSize := info.Size()

	footerOffset := fileSize - footerSize
	buffer := make([]byte, footerSize)

	_, err = reader.fd.ReadAt(buffer, footerOffset)
	if err != nil {
		return fmt.Errorf("footer reader: readAt: %w", err)
	}

	footer, err := decodeFooter(buffer)
	if err != nil {
		return fmt.Errorf("footer reader: decode footer: %w", err)
	}

	reader.foot = footer

	return nil
}

func (reader *SSTableReader) bloomFilterReader() (*bloomFilter, error) {
	buffer := make([]byte, reader.foot.bloomSize+4)

	_, err := reader.fd.ReadAt(buffer, int64(reader.foot.bloomOffset))
	if err != nil {
		return nil, fmt.Errorf("bloomFilter reader: readAt: %w", err)
	}

	bloomFilter, err := decodeBloomFilter(buffer, reader.foot.bloomSize)
	if err != nil {
		return nil, fmt.Errorf("bloomFilter reader: decode bloomFilter: %w", err)
	}

	return bloomFilter, nil
}

func (reader *SSTableReader) indexEntryReader() ([]indexEntry, error) {
	buffer := make([]byte, reader.foot.indexSize+4)

	_, err := reader.fd.ReadAt(buffer, int64(reader.foot.indexOffset))
	if err != nil {
		return nil, fmt.Errorf("indexEntry reader: readAt: %w", err)
	}

	indexes, err := decodeSparseIndex(buffer, reader.foot.indexSize)
	if err != nil {
		return nil, fmt.Errorf("indexEntry reader: decode indexEntry: %w", err)
	}

	return indexes, nil
}

func (reader *SSTableReader) blockReader(offset uint64) ([]*record, error) {
	buffer := make([]byte, 4)

	_, err := reader.fd.ReadAt(buffer, int64(offset))
	if err != nil {
		return nil, fmt.Errorf("block reader: readAt for len: %w", err)
	}

	length := binary.BigEndian.Uint32(buffer)
	if length > maxExpectedBlockSize {
		return nil, fmt.Errorf("block reader: unexpectedly large block size")
	}

	dataBufferAndCRC := make([]byte, length+4)
	_, err = reader.fd.ReadAt(dataBufferAndCRC, int64(offset+4))
	if err != nil {
		return nil, fmt.Errorf("block reader: readAt for data and crc: %w", err)
	}

	buffer = append(buffer, dataBufferAndCRC...)

	expectedCRC := crc32.ChecksumIEEE(buffer[:len(buffer)-4])
	actualCRC := binary.BigEndian.Uint32(buffer[len(buffer)-4:])
	if expectedCRC != actualCRC {
		return nil, fmt.Errorf("block reader: crc mismatch: corruption detected")
	}

	var records []*record
	var ptr uint32 = 4 // Start from first record

	// till checksum bit encounter
	for ptr < length+4 {
		if ptr+5 > length+4 {
			return nil, fmt.Errorf("block reader: unexpected end of block before record header")
		}

		keyLen := binary.BigEndian.Uint16(buffer[ptr+1 : ptr+3])
		valueLen := binary.BigEndian.Uint16(buffer[ptr+3 : ptr+5])

		recordEnd := ptr + 5 + uint32(keyLen) + uint32(valueLen)
		if recordEnd > length+4 {
			return nil, fmt.Errorf("block reader: unexpected end of block before record data")
		}

		rec, err := decodeRecord(buffer[ptr:recordEnd])
		if err != nil {
			return nil, fmt.Errorf("block reader: block decoding failed: %w", err)
		}

		records = append(records, rec)
		ptr = recordEnd
	}

	return records, nil
}
