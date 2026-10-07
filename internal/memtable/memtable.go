package memtable

import (
	"math/rand"
)

const maxSkipListHeight = 20
const MemTableFlushThreshold = 80 * 1024 * 1024 // 80 MiB

// string = 16 bytes - (data pointer,8) + (length,8)
//
// key   = 16 bytes
// value = 16 bytes
// next  = 24 bytes - (data pointer,8) + (length,8) + (capacity,8)
//
// 16 + 16 + 24 = 56 bytes
//
// 2 more bytes for uint8 entry type
const skipListNodeOverhead = 58

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

	// tells how many key/value nodes are present
	entryCount uint64
}

type skipListNode struct {
	listEntry *entry
	next      []*skipListNode
}

type entry struct {
	key   string
	value string
	eType entryType
}

type entryType uint8

const (
	SetEntry    entryType = 0x01
	DeleteEntry entryType = 0x02
)

func newSkipList() *skipList {
	h := &skipListNode{
		next: make([]*skipListNode, maxSkipListHeight),
	}

	return &skipList{
		head: h,
	}
}

func randomHeight() int {
	height := 1

	for height < maxSkipListHeight && rand.Intn(2) == 0 {
		height++
	}

	return height
}

func NewMemTable() *MemTable {
	// head size in skip list
	sentinelOverhead := int64(skipListNodeOverhead + 8*maxSkipListHeight)

	return &MemTable{
		list:       newSkipList(),
		approxSize: sentinelOverhead,
	}
}

func (mTable *MemTable) Put(key, value string) {
	sizeDelta := mTable.list.putNode(SetEntry, key, value)
	mTable.approxSize += sizeDelta
}

func (mTable *MemTable) Get(key string) (string, entryType, bool) {
	return mTable.list.getNode(key)
}

func (mTable *MemTable) Delete(key string) {
	sizeDelta := mTable.list.putNode(DeleteEntry, key, "")
	mTable.approxSize += sizeDelta
}

func (mTable *MemTable) EntryCount() uint64 {
	return mTable.list.entryCount
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
	return it.current.listEntry.key
}

// will panic if Iterator.Next() returned false
func (it *iterator) Value() string {
	return it.current.listEntry.value
}

func (it *iterator) Type() entryType {
	return it.current.listEntry.eType
}

// method to find nodes which are predecessor to targetKey at each level
func (list *skipList) searchList(targetKey string) []*skipListNode {
	update := make([]*skipListNode, maxSkipListHeight)

	current := list.head
	for level := maxSkipListHeight - 1; level >= 0; level-- {
		for current.next[level] != nil && current.next[level].listEntry.key < targetKey {
			current = current.next[level]
		}

		update[level] = current
	}

	return update
}

func (list *skipList) getNode(key string) (string, entryType, bool) {
	update := list.searchList(key)

	predecessor := update[0]
	candidate := predecessor.next[0]

	if candidate != nil && candidate.listEntry.key == key {
		return candidate.listEntry.value, candidate.listEntry.eType, true
	}

	return "", 0, false
}

func (list *skipList) putNode(t entryType, k, v string) int64 {
	return list.putNodeWithHeight(t, k, v, randomHeight())
}

func (list *skipList) putNodeWithHeight(t entryType, k, v string, height int) (sizeDelta int64) {
	update := list.searchList(k)

	candidate := update[0].next[0]

	if candidate != nil && candidate.listEntry.key == k {
		old := candidate.listEntry

		// SET -> DELETE && DELETE -> DELETE
		if t == DeleteEntry {
			if old.eType == SetEntry {
				sizeDelta = -int64(len(old.value))
				old.value = ""
				old.eType = DeleteEntry
			}
			return
		}

		// SET -> SET
		if old.eType == SetEntry {
			sizeDelta = int64(len(v) - len(old.value))
			old.value = v
			return
		}

		// DELETE → SET
		sizeDelta = int64(len(v))
		old.value = v
		old.eType = SetEntry
		return
	}

	// new node to be added
	entry := &entry{
		key:   k,
		value: v,
		eType: t,
	}

	node := &skipListNode{
		listEntry: entry,
		next:      make([]*skipListNode, height),
	}
	sizeDelta = int64(skipListNodeOverhead + len(k) + len(v) + 8*height)
	list.entryCount++

	for level := range height {
		node.next[level] = update[level].next[level]
		update[level].next[level] = node
	}

	return
}
