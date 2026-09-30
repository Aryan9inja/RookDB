package memtable

import (
	"math/rand"
)

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

func (list *skipList) getValue(key string) (string, bool) {
	update := list.searchList(key)

	predecessor := update[0]
	candidate := predecessor.next[0]

	if candidate != nil && candidate.key == key {
		return candidate.value, true
	}

	return "", false
}

func (list *skipList) putNode(k, v string) {
	list.putNodeWithHeight(k, v, randomHeight())
}

func (list *skipList) putNodeWithHeight(k, v string, height int) {
	update := list.searchList(k)

	candidate := update[0].next[0]

	if candidate != nil && candidate.key == k {
		candidate.value = v
		return
	}

	node := &skipListNode{
		key:   k,
		value: v,
		next:  make([]*skipListNode, height),
	}

	for level := range height {
		node.next[level] = update[level].next[level]
		update[level].next[level] = node
	}
}

func (list *skipList) deleteNode(k string) {
	update := list.searchList(k)

	candidate := update[0].next[0]
	if candidate == nil || candidate.key != k {
		// no op
		return
	}

	nextLevels := len(candidate.next)
	for level := range nextLevels {
		update[level].next[level] = candidate.next[level]
	}
}
