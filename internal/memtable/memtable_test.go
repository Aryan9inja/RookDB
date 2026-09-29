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

func newTestNode(t *testing.T, key, value string, height int) *skipListNode {
	t.Helper()
	return &skipListNode{
		key:   key,
		value: value,
		next:  make([]*skipListNode, height),
	}
}
