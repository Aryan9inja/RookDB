package sstable

import (
	"fmt"
	"testing"
)

func TestBloomFilter_Constructor(t *testing.T) {
	t.Run("non-zero keys", func(t *testing.T) {
		bf := newBloomFilter(100)
		if bf.numBits <= 0 {
			t.Errorf("expected numBits > 0, got %d", bf.numBits)
		}
		expectedLen := (bf.numBits + 7) / 8
		if uint64(len(bf.bits)) != expectedLen {
			t.Errorf("expected len(bits) == %d, got %d", expectedLen, len(bf.bits))
		}
		if bf.numHashes != 4 {
			t.Errorf("expected numHashes == 4, got %d", bf.numHashes)
		}
	})

	t.Run("zero keys", func(t *testing.T) {
		bf := newBloomFilter(0)
		if bf.numBits != 0 {
			t.Errorf("expected numBits == 0, got %d", bf.numBits)
		}
		
		// Should not panic
		err := bf.add("test")
		if err != nil {
			t.Errorf("unexpected error on add: %v", err)
		}
		
		contains, err := bf.maycontain("test")
		if err != nil {
			t.Errorf("unexpected error on maycontain: %v", err)
		}
		if contains {
			t.Errorf("expected contains == false for zero-key bloom filter")
		}
	})
}

func TestBloomFilter_BasicMembership(t *testing.T) {
	bf := newBloomFilter(10)
	keys := []string{"Aryan", "test", "rookdb", "sstable"}
	for _, key := range keys {
		if err := bf.add(key); err != nil {
			t.Errorf("unexpected error adding %s: %v", key, err)
		}
	}
	for _, key := range keys {
		contains, err := bf.maycontain(key)
		if err != nil {
			t.Errorf("unexpected error checking %s: %v", key, err)
		}
		if !contains {
			t.Errorf("expected to contain %s", key)
		}
	}
}

func TestBloomFilter_NoFalseNegatives(t *testing.T) {
	numKeys := 1000
	bf := newBloomFilter(uint64(numKeys))
	
	for i := range numKeys {
		key := fmt.Sprintf("key_%d", i)
		if err := bf.add(key); err != nil {
			t.Errorf("unexpected error adding %s: %v", key, err)
		}
	}
	
	for i := range numKeys {
		key := fmt.Sprintf("key_%d", i)
		contains, err := bf.maycontain(key)
		if err != nil {
			t.Errorf("unexpected error checking %s: %v", key, err)
		}
		if !contains {
			t.Errorf("false negative detected for %s", key)
		}
	}
}

func TestBloomFilter_EmptyFilter(t *testing.T) {
	bf := newBloomFilter(100)
	contains, err := bf.maycontain("something")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if contains {
		t.Errorf("expected false for empty filter")
	}
	
	// verify all bits are zero
	for i, b := range bf.bits {
		if b != 0 {
			t.Errorf("expected bits to be 0, got %d at index %d", b, i)
		}
	}
}

func TestBloomFilter_DuplicateInsertion(t *testing.T) {
	bf := newBloomFilter(10)
	if err := bf.add("foo"); err != nil {
		t.Errorf("unexpected error adding foo: %v", err)
	}
	if err := bf.add("foo"); err != nil {
		t.Errorf("unexpected error adding foo again: %v", err)
	}
	contains, err := bf.maycontain("foo")
	if err != nil {
		t.Errorf("unexpected error checking foo: %v", err)
	}
	if !contains {
		t.Errorf("expected true for foo")
	}
}
