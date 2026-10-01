package memtable

import (
	"slices"
	"testing"
)

func TestIterator(t *testing.T) {
	t.Run("empty MemTable", func(t *testing.T) {
		memT := MemTable{list: newSkipList()}

		it := memT.Iterator()

		if it.Next() {
			t.Fatalf("expected Next() to return false but got true")
		}
	})

	t.Run("single element", func(t *testing.T) {
		memT := MemTable{list: newSkipList()}
		memT.list.putNode(setEntry, "name", "aryan")

		it := memT.Iterator()

		if !it.Next() {
			t.Fatalf("expected Next() to return true but got false")
		}

		key := it.Key()
		value := it.Value()
		if key != "name" || value != "aryan" {
			t.Fatalf("expected key-value to be name-aryan, got %v-%v", key, value)
		}

		if it.Next() {
			t.Fatalf("expected Next() to return false but got true")
		}
	})

	t.Run("multiple elements in sorted order", func(t *testing.T) {
		memT := MemTable{list: newSkipList()}
		memT.list.putNode(setEntry, "pet", "3")
		memT.list.putNode(setEntry, "bat", "1")
		memT.list.putNode(setEntry, "cat", "2")

		expected := []struct{ key, value string }{
			{"bat", "1"},
			{"cat", "2"},
			{"pet", "3"},
		}
		it := memT.Iterator()
		for _, item := range expected {
			if !it.Next() {
				t.Fatalf("expected Next() for %q to return true", item.key)
			}
			if got := it.Key(); got != item.key {
				t.Fatalf("expected key %q, got %q", item.key, got)
			}
			if got := it.Value(); got != item.value {
				t.Fatalf("expected value %q, got %q", item.value, got)
			}
		}
	})

	t.Run("exhaustion", func(t *testing.T) {
		memT := MemTable{list: newSkipList()}
		memT.list.putNode(setEntry, "bat", "1")
		memT.list.putNode(setEntry, "cat", "2")

		it := memT.Iterator()
		if !it.Next() {
			t.Fatal("expected first Next() to return true")
		}
		if !it.Next() {
			t.Fatal("expected second Next() to return true")
		}
		if it.Next() {
			t.Fatal("expected Next() to return false after consuming all elements")
		}
	})

	t.Run("repeated Next() after exhaustion", func(t *testing.T) {
		memT := MemTable{list: newSkipList()}
		memT.list.putNode(setEntry, "name", "aryan")

		it := memT.Iterator()
		if !it.Next() {
			t.Fatal("expected Next() to return true for the element")
		}
		if it.Next() {
			t.Fatal("expected Next() to return false after consuming the element")
		}
		if it.Next() {
			t.Fatal("expected repeated Next() after exhaustion to return false")
		}
		if it.Next() {
			t.Fatal("expected subsequent Next() after exhaustion to return false")
		}
	})
}

// Helper function to safely compare two skip list nodes
func nodesEqual(a, b *skipListNode) bool {
	if a == b {
		return true
	}
	
	if a == nil || b == nil {
		return false
	}
	
	if a.listEntry == nil || b.listEntry == nil {
		return false
	}
	
	return a.listEntry.key == b.listEntry.key
}

func TestSearchList(t *testing.T) {
	// Skip list nodes
	bat := newTestNode(t, setEntry, "bat", "xyz", 1)
	cat := newTestNode(t, setEntry, "cat", "xyz", 3)
	pet := newTestNode(t, setEntry, "pet", "xyz", 2)

	// Generate skip list structure
	list := newSkipList()
	list.head.next[0] = bat
	list.head.next[1] = cat
	list.head.next[2] = cat

	bat.next[0] = cat

	cat.next[0] = pet
	cat.next[1] = pet

	t.Run("target before every key", func(t *testing.T) {
		expected := make([]*skipListNode, maxSkipListHeight)
		for i := range expected {
			expected[i] = list.head
		}

		got := list.searchList("aryan")

		if !slices.EqualFunc(got, expected, nodesEqual) {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	})

	t.Run("target between two keys", func(t *testing.T) {
		expected := make([]*skipListNode, maxSkipListHeight)
		for i := range expected {
			expected[i] = list.head
		}
		expected[0] = cat
		expected[1] = cat
		expected[2] = cat

		got := list.searchList("dog")

		if !slices.EqualFunc(got, expected, nodesEqual) {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	})

	t.Run("target after every key", func(t *testing.T) {
		expected := make([]*skipListNode, maxSkipListHeight)
		for i := range expected {
			expected[i] = list.head
		}
		expected[0] = pet
		expected[1] = pet
		expected[2] = cat

		got := list.searchList("thakur")

		if !slices.EqualFunc(got, expected, nodesEqual) {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	})

	t.Run("target equal to an existing key", func(t *testing.T) {
		expected := make([]*skipListNode, maxSkipListHeight)
		for i := range expected {
			expected[i] = list.head
		}
		expected[0] = bat

		got := list.searchList("cat")

		if !slices.EqualFunc(got, expected, nodesEqual) {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		emptyList := newSkipList()
		expected := make([]*skipListNode, maxSkipListHeight)
		for i := range expected {
			expected[i] = emptyList.head
		}

		got := emptyList.searchList("aryan")

		if !slices.EqualFunc(got, expected, nodesEqual) {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	})
}

func TestGetValue(t *testing.T) {
	bat := newTestNode(t, setEntry, "bat", "abc", 1)
	cat := newTestNode(t, setEntry, "cat", "xyz", 3)
	pet := newTestNode(t, setEntry, "pet", "ijk", 2)

	// Generate skip list structure
	list := newSkipList()
	list.head.next[0] = bat
	list.head.next[1] = cat
	list.head.next[2] = cat

	bat.next[0] = cat

	cat.next[0] = pet
	cat.next[1] = pet

	t.Run("exisiting key", func(t *testing.T) {
		expected := "abc"
		got, ok := list.getValue("bat")

		if got != expected || !ok {
			t.Fatalf("expected %v and true, got %v and %v", expected, got, ok)
		}
	})

	t.Run("target missing between keys", func(t *testing.T) {
		expected := ""
		got, ok := list.getValue("dog")

		if got != expected || ok {
			t.Fatalf("expected %v and false, got %v and %v", expected, got, ok)
		}
	})

	t.Run("target missing before existing keys", func(t *testing.T) {
		expected := ""
		got, ok := list.getValue("aryan")

		if got != expected || ok {
			t.Fatalf("expected %v and false, got %v and %v", expected, got, ok)
		}
	})

	t.Run("target missing after existing keys", func(t *testing.T) {
		expected := ""
		got, ok := list.getValue("thakur")

		if got != expected || ok {
			t.Fatalf("expected %v and false, got %v and %v", expected, got, ok)
		}
	})

	t.Run("get value in empty list", func(t *testing.T) {
		emptyList := newSkipList()
		expected := ""
		got, ok := emptyList.getValue("cat")

		if got != expected || ok {
			t.Fatalf("expected %v and false, got %v and %v", expected, got, ok)
		}
	})
}

func TestPutNode(t *testing.T) {
	t.Run("insert to empty list, SET", func(t *testing.T) {
		emptyList := newSkipList()

		delta:=emptyList.putNode(setEntry, "name", "aryan")

		expected := "aryan"
		got, ok := emptyList.getValue("name")
		if got != expected || !ok {
			t.Fatalf("expected %v and true, got %v and %v", expected, got, ok)
		}

		if delta <= 0{
			t.Fatalf("expected delta to be positive on new set operation")
		}
	})

	t.Run("insert to empty list, SET", func(t *testing.T) {
		emptyList := newSkipList()

		delta := emptyList.putNode(deleteEntry, "name", "aryan")

		expected := "aryan"
		got, ok := emptyList.getValue("name")
		if got != expected || !ok {
			t.Fatalf("expected %v and true, got %v and %v", expected, got, ok)
		}

		if delta <= 0{
			t.Fatalf("expected delta to be positive on new delete operation")
		}
	})

	t.Run("insert before existing keys", func(t *testing.T) {
		list := newSkipList()
		list.putNode(setEntry, "bat", "abc")
		list.putNode(setEntry, "cat", "xyz")

		list.putNode(setEntry, "ant", "first")

		expected := "first"
		got, ok := list.getValue("ant")
		if got != expected || !ok {
			t.Fatalf("expected %v and true, got %v and %v", expected, got, ok)
		}
		if got := list.head.next[0].listEntry.key; got != "ant" {
			t.Fatalf("expected ant at the start, got %q", got)
		}
	})

	t.Run("insert between keys", func(t *testing.T) {
		list := newSkipList()
		list.putNode(setEntry, "bat", "abc")
		list.putNode(setEntry, "pet", "ijk")

		list.putNode(setEntry, "cat", "middle")

		expected := "middle"
		got, ok := list.getValue("cat")
		if got != expected || !ok {
			t.Fatalf("expected %v and true, got %v and %v", expected, got, ok)
		}

		first := list.head.next[0]
		second := first.next[0]
		third := second.next[0]
		if first.listEntry.key != "bat" {
			t.Fatalf("expected first key bat, got %q", first.listEntry.key)
		}
		if second.listEntry.key != "cat" {
			t.Fatalf("expected second key cat, got %q", second.listEntry.key)
		}
		if third.listEntry.key != "pet" {
			t.Fatalf("expected third key pet, got %q", third.listEntry.key)
		}
	})

	t.Run("insert after existing keys", func(t *testing.T) {
		list := newSkipList()
		list.putNode(setEntry, "bat", "abc")
		list.putNode(setEntry, "cat", "xyz")

		list.putNode(setEntry, "zoo", "last")

		expected := "last"
		got, ok := list.getValue("zoo")
		if got != expected || !ok {
			t.Fatalf("expected %v and true, got %v and %v", expected, got, ok)
		}

		last := list.head.next[0]
		for last.next[0] != nil {
			last = last.next[0]
		}
		if last.listEntry.key != "zoo" {
			t.Fatalf("expected zoo at the end, got %q", last.listEntry.key)
		}
	})

	t.Run("update existing key, SET -> SET", func(t *testing.T) {
		list := newSkipList()
		list.putNode(setEntry, "cat", "old")

		original := list.head.next[0]
		delta := list.putNode(setEntry, "cat", "newVal")

		expected := "newVal"
		got, ok := list.getValue("cat")
		if got != expected || !ok {
			t.Fatalf("expected %v and true, got %v and %v", expected, got, ok)
		}
		if list.head.next[0] != original || original.next[0] != nil {
			t.Fatal("updating a key should change its value without inserting another node")
		}

		if delta <= 0{
			t.Fatalf("expected delta to be positive for bigger value")
		}
	})

	t.Run("update existing key, SET -> DELETE", func(t *testing.T) {
		list := newSkipList()
		list.putNode(setEntry, "cat", "old")

		original := list.head.next[0]
		delta := list.putNode(deleteEntry, "cat", "")

		expected := ""
		got, ok := list.getValue("cat")
		if got != expected || !ok {
			t.Fatalf("expected %v and true, got %v and %v", expected, got, ok)
		}
		if list.head.next[0] != original || original.next[0] != nil {
			t.Fatal("updating a key should change its value without inserting another node")
		}

		if delta >= 0{
			t.Fatalf("expected delta to be negative for delete operation")
		}
	})

	t.Run("update existing key, DELETE -> SET", func(t *testing.T) {
		list := newSkipList()
		list.putNode(deleteEntry, "cat", "")

		original := list.head.next[0]
		delta := list.putNode(setEntry, "cat", "new")

		expected := "new"
		got, ok := list.getValue("cat")
		if got != expected || !ok {
			t.Fatalf("expected %v and true, got %v and %v", expected, got, ok)
		}
		if list.head.next[0] != original || original.next[0] != nil {
			t.Fatal("updating a key should change its value without inserting another node")
		}

		if delta <= 0{
			t.Fatalf("expected delta to be negative for set after delete operation")
		}
	})

	t.Run("update existing key, DELETE - DELETE", func(t *testing.T) {
		list := newSkipList()
		list.putNode(deleteEntry, "cat", "")

		original := list.head.next[0]
		delta := list.putNode(deleteEntry, "cat", "")

		expected := ""
		got, ok := list.getValue("cat")
		if got != expected || !ok {
			t.Fatalf("expected %v and true, got %v and %v", expected, got, ok)
		}
		if list.head.next[0] != original || original.next[0] != nil {
			t.Fatal("updating a key should change its value without inserting another node")
		}

		if delta != 0{
			t.Fatalf("expected delta to be zero for delete after delete operation")
		}
	})

	t.Run("node with height greater than one is linked at every level", func(t *testing.T) {
		list := newSkipList()
		list.putNodeWithHeight(setEntry, "bat", "first", 2)
		list.putNodeWithHeight(setEntry, "cat", "middle", 4)
		list.putNodeWithHeight(setEntry, "pet", "last", 1)

		cat := list.head.next[0].next[0]
		if cat.listEntry.key != "cat" {
			t.Fatalf("expected cat at level 0, got %q", cat.listEntry.key)
		}
		if len(cat.next) != 4 {
			t.Fatalf("expected cat height 4, got %d", len(cat.next))
		}

		for level := range len(cat.next) {
			predecessor := list.head
			for predecessor.next[level] != nil && predecessor.next[level] != cat {
				predecessor = predecessor.next[level]
			}
			if predecessor.next[level] != cat {
				t.Fatalf("cat is not linked at level %d", level)
			}
		}
		if list.head.next[4] != nil {
			t.Fatal("cat should not be linked above its height")
		}
	})

	t.Run("multiple inserts preserve sorted order", func(t *testing.T) {
		list := newSkipList()
		inserts := []struct{ key, value string }{
			{"pet", "3"},
			{"bat", "1"},
			{"cat", "2"},
			{"ant", "0"},
			{"zoo", "4"},
			{"cat", "updated"},
		}
		for _, item := range inserts {
			list.putNode(setEntry, item.key, item.value)
		}

		var keys []string
		for node := list.head.next[0]; node != nil; node = node.next[0] {
			keys = append(keys, node.listEntry.key)
		}
		if !slices.Equal(keys, []string{"ant", "bat", "cat", "pet", "zoo"}) {
			t.Fatalf("expected sorted unique keys [ant bat cat pet zoo], got %v", keys)
		}

		expected := "updated"
		got, ok := list.getValue("cat")
		if got != expected || !ok {
			t.Fatalf("expected %v and true, got %v and %v", expected, got, ok)
		}
	})
}

func newTestNode(t *testing.T, eType entryType, key, value string, height int) *skipListNode {
	t.Helper()

	entry := &entry{
		key:   key,
		value: value,
		eType: eType,
	}

	return &skipListNode{
		listEntry: entry,
		next:      make([]*skipListNode, height),
	}
}
