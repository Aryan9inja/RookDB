package sstable

import "encoding/binary"

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
