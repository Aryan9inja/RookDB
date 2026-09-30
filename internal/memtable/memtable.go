package memtable

import (
	"math/rand"
)

const MaxSkipListHeight = 20
const MemTableFlushThreshold = 80 * 1024 * 1024 // 80 MiB

// string = 16 bytes - (data pointer,8) + (length,8)
//
// key   = 16 bytes
// value = 16 bytes
// next  = 24 bytes - (data pointer,8) + (length,8) + (capacity,8)
//
// 16 + 16 + 24 = 56 bytes
const SkipListNodeOverhead = 56

type MemTable struct {
	list *skipList

	// approxSize estimates the logical memory footprint of the MemTable.
	// It is intentionally approximate and is used only as a flush heuristic,
	// not as an exact measurement of process memory.
	approxSize int64
}

type iterator struct {
	current *skipListNode
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

func NewMemTable() *MemTable {
	// head size in skip list
	sentinelOverhead := int64(SkipListNodeOverhead + 8*MaxSkipListHeight)

	return &MemTable{
		list:       newSkipList(),
		approxSize: sentinelOverhead,
	}
}

func (mTable *MemTable) Put(key, value string) {
	sizeDelta := mTable.list.putNode(key, value)
	mTable.approxSize += sizeDelta
}

func (mTable *MemTable) Get(key string) (string, bool) {
	return mTable.list.getValue(key)
}

func (mTable *MemTable) Delete(key string) {
	sizeDelta := mTable.list.deleteNode(key)
	mTable.approxSize += sizeDelta
}

func (m *MemTable) Iterator() *iterator {
	return &iterator{
		current: m.list.head,
	}
}

func (it *iterator) Next() bool {
	if it.current == nil || it.current.next[0] == nil {
		it.current = nil
		return false
	}

	// move one step
	it.current = it.current.next[0]
	return true
}

// will panic if Iterator.Next() returned false
func (it *iterator) Key() string {
	return it.current.key
}

// will panic if Iterator.Next() returned false
func (it *iterator) Value() string {
	return it.current.value
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

func (list *skipList) putNode(k, v string) int64 {
	return list.putNodeWithHeight(k, v, randomHeight())
}

func (list *skipList) putNodeWithHeight(k, v string, height int) (sizeDelta int64) {
	update := list.searchList(k)

	candidate := update[0].next[0]

	if candidate != nil && candidate.key == k {
		sizeDelta = int64(len(v) - len(candidate.value))
		candidate.value = v
		return
	}

	node := &skipListNode{
		key:   k,
		value: v,
		next:  make([]*skipListNode, height),
	}
	sizeDelta = int64(SkipListNodeOverhead + len(k) + len(v) + 8*height)

	for level := range height {
		node.next[level] = update[level].next[level]
		update[level].next[level] = node
	}

	return
}

func (list *skipList) deleteNode(k string) (sizeDelta int64) {
	update := list.searchList(k)

	candidate := update[0].next[0]
	if candidate == nil || candidate.key != k {
		// no op
		return
	}

	nextLevels := len(candidate.next)

	sizeDelta = -int64(SkipListNodeOverhead + len(candidate.key) + len(candidate.value) + 8*nextLevels)

	for level := range nextLevels {
		update[level].next[level] = candidate.next[level]
	}
	return
}
