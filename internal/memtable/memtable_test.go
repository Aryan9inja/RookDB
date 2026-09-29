package memtable

import (
	"slices"
	"testing"
)

func TestSearchList(t *testing.T) {
	// Skip list nodes
	bat := newTestNode(t, "bat", "xyz", 1)
	cat := newTestNode(t, "cat", "xyz", 3)
	pet := newTestNode(t, "pet", "xyz", 2)

	// Generate skip list structure
	list := newSkipList()
	list.head.next[0] = bat
	list.head.next[1] = cat
	list.head.next[2] = cat

	bat.next[0] = cat

	cat.next[0] = pet
	cat.next[1] = pet

	t.Run("target before every key", func(t *testing.T) {
		expected := make([]*skipListNode, MaxSkipListHeight)
		for i := range expected {
			expected[i] = list.head
		}

		got := list.searchList("aryan")

		if !slices.EqualFunc(got, expected, func(a, b *skipListNode) bool {
			if a == nil || b == nil {
				return a == b
			}
			return a.key == b.key
		}) {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	})

	t.Run("target between two keys", func(t *testing.T) {
		expected := make([]*skipListNode, MaxSkipListHeight)
		for i := range expected {
			expected[i] = list.head
		}
		expected[0] = cat
		expected[1] = cat
		expected[2] = cat

		got := list.searchList("dog")

		if !slices.EqualFunc(got, expected, func(a, b *skipListNode) bool {
			if a == nil || b == nil {
				return a == b
			}
			return a.key == b.key
		}) {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	})

	t.Run("target after every key", func(t *testing.T) {
		expected := make([]*skipListNode, MaxSkipListHeight)
		for i := range expected {
			expected[i] = list.head
		}
		expected[0] = pet
		expected[1] = pet
		expected[2] = cat

		got := list.searchList("thakur")

		if !slices.EqualFunc(got, expected, func(a, b *skipListNode) bool {
			if a == nil || b == nil {
				return a == b
			}
			return a.key == b.key
		}) {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	})

	t.Run("target equal to an existing key", func(t *testing.T) {
		expected := make([]*skipListNode, MaxSkipListHeight)
		for i := range expected {
			expected[i] = list.head
		}
		expected[0] = bat

		got := list.searchList("cat")

		if !slices.EqualFunc(got, expected, func(a, b *skipListNode) bool {
			if a == nil || b == nil {
				return a == b
			}
			return a.key == b.key
		}) {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		emptyList := newSkipList()
		expected := make([]*skipListNode, MaxSkipListHeight)
		for i := range expected {
			expected[i] = emptyList.head
		}

		got := emptyList.searchList("aryan")

		if !slices.EqualFunc(got, expected, func(a, b *skipListNode) bool {
			if a == nil || b == nil {
				return a == b
			}
			return a.key == b.key
		}) {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	})
}

func TestGetValue(t *testing.T) {
	bat := newTestNode(t, "bat", "abc", 1)
	cat := newTestNode(t, "cat", "xyz", 3)
	pet := newTestNode(t, "pet", "ijk", 2)

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
	t.Run("insert to empty list", func(t *testing.T) {
		emptyList := newSkipList()

		emptyList.putNode("name", "aryan")

		expected := "aryan"
		got, ok := emptyList.getValue("name")
		if got != expected || !ok {
			t.Fatalf("expected %v and true, got %v and %v", expected, got, ok)
		}
	})

	t.Run("insert before existing keys", func(t *testing.T) {
		list := newSkipList()
		list.putNode("bat", "abc")
		list.putNode("cat", "xyz")

		list.putNode("ant", "first")

		expected := "first"
		got, ok := list.getValue("ant")
		if got != expected || !ok {
			t.Fatalf("expected %v and true, got %v and %v", expected, got, ok)
		}
		if got := list.head.next[0].key; got != "ant" {
			t.Fatalf("expected ant at the start, got %q", got)
		}
	})

	t.Run("insert between keys", func(t *testing.T) {
		list := newSkipList()
		list.putNode("bat", "abc")
		list.putNode("pet", "ijk")

		list.putNode("cat", "middle")

		expected := "middle"
		got, ok := list.getValue("cat")
		if got != expected || !ok {
			t.Fatalf("expected %v and true, got %v and %v", expected, got, ok)
		}

		first := list.head.next[0]
		second := first.next[0]
		third := second.next[0]
		if first.key != "bat" {
			t.Fatalf("expected first key bat, got %q", first.key)
		}
		if second.key != "cat" {
			t.Fatalf("expected second key cat, got %q", second.key)
		}
		if third.key != "pet" {
			t.Fatalf("expected third key pet, got %q", third.key)
		}
	})

	t.Run("insert after existing keys", func(t *testing.T) {
		list := newSkipList()
		list.putNode("bat", "abc")
		list.putNode("cat", "xyz")

		list.putNode("zoo", "last")

		expected := "last"
		got, ok := list.getValue("zoo")
		if got != expected || !ok {
			t.Fatalf("expected %v and true, got %v and %v", expected, got, ok)
		}

		last := list.head.next[0]
		for last.next[0] != nil {
			last = last.next[0]
		}
		if last.key != "zoo" {
			t.Fatalf("expected zoo at the end, got %q", last.key)
		}
	})

	t.Run("update existing key", func(t *testing.T) {
		list := newSkipList()
		list.putNode("cat", "old")

		original := list.head.next[0]
		list.putNode("cat", "new")

		expected := "new"
		got, ok := list.getValue("cat")
		if got != expected || !ok {
			t.Fatalf("expected %v and true, got %v and %v", expected, got, ok)
		}
		if list.head.next[0] != original || original.next[0] != nil {
			t.Fatal("updating a key should change its value without inserting another node")
		}
	})

	t.Run("node with height greater than one is linked at every level", func(t *testing.T) {
		list := newSkipList()
		list.putNodeWithHeight("bat", "first", 2)
		list.putNodeWithHeight("cat", "middle", 4)
		list.putNodeWithHeight("pet", "last", 1)

		cat := list.head.next[0].next[0]
		if cat.key != "cat" {
			t.Fatalf("expected cat at level 0, got %q", cat.key)
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
			list.putNode(item.key, item.value)
		}

		var keys []string
		for node := list.head.next[0]; node != nil; node = node.next[0] {
			keys = append(keys, node.key)
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

func newTestNode(t *testing.T, key, value string, height int) *skipListNode {
	t.Helper()
	return &skipListNode{
		key:   key,
		value: value,
		next:  make([]*skipListNode, height),
	}
}
