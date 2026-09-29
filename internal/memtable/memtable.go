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

// method to find nodes which are predecessor to targetKey at each level
func (list *skipList) searchList(targetKey string) []*skipListNode {
	update := make([]*skipListNode, MaxSkipListHeight)

	current := list.head
	for level := MaxSkipListHeight - 1; level >= 0; level-- {
		for current.next[level] != nil && current.next[level].key < targetKey {
			current = current.next[level]
		}

		update[level] = current
	}

	return update
}
