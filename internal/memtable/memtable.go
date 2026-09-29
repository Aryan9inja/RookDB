package memtable

import "math/rand"

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

func newSkipList() *skipList {
	h := &skipListNode{
		next: make([]*skipListNode, MaxSkipListHeight),
	}

	return &skipList{
		head: h,
	}
}

func randomHeight() int {
	height := 1

	for height < MaxSkipListHeight && rand.Intn(2) == 0 {
		height++
	}

	return height
}
