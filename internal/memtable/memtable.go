package memtable

const MaxSkipListHeight = 20
const MemTableFlushThreshold = 80 * 1024 * 1024 // 80 MiB

type MemTable struct {
	list       *skipList
	approxSize int64
}

type skipList struct {
	head *skipListNode
}

type skipListNode struct {
	key   string
	value string
	next  []*skipListNode
}