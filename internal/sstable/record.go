package sstable

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
