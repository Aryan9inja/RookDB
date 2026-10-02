package sstable

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type record struct {
	rType recordType
	key   string
	value string
}

type recordType uint8

const (
	setRecord    recordType = 0x01
	deleteRecord recordType = 0x02
)

var ErrInvalidSSTableRecord error = errors.New("Invalid record")

func (r *record) Encode() []byte {
	// [type][key_len][value_len][key_bytes][value_bytes]
	recordLen := 1 + 2 + 2 + len(r.key) + len(r.value)
	buf := make([]byte, recordLen)

	buf[0] = byte(r.rType)

	binary.BigEndian.PutUint16(buf[1:3], uint16(len(r.key)))
	binary.BigEndian.PutUint16(buf[3:5], uint16(len(r.value)))

	copy(buf[5:], r.key)
	copy(buf[5+len(r.key):], r.value)

	return buf
}

func DecodeRecord(data []byte) (*record, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("sstable: record decoding: at least 5 bytes are required: %w", ErrInvalidSSTableRecord)
	}

	recType := recordType(data[0])
	if recType != setRecord && recType != deleteRecord {
		return nil, fmt.Errorf("sstable: record decoding: unknown record type: %w", ErrInvalidSSTableRecord)
	}

	keyLen := binary.BigEndian.Uint16(data[1:3])
	valueLen := binary.BigEndian.Uint16(data[3:5])
	if recType == deleteRecord && valueLen != 0 {
		return nil, fmt.Errorf("sstable: record decoding: delete requires no value: %w", ErrInvalidSSTableRecord)
	}

	expectedSize := 5 + int(keyLen) + int(valueLen)
	if expectedSize != len(data) {
		return nil, fmt.Errorf("sstable: record decoding: invalid data size: %w", ErrInvalidSSTableRecord)
	}

	key := string(data[5 : 5+keyLen])
	value := string(data[5+keyLen:])

	return &record{
		rType: recType,
		key:   key,
		value: value,
	}, nil
}
