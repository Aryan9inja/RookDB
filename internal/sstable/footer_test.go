package sstable

import (
	"errors"
	"testing"
)

func TestFooter(t *testing.T) {
	t.Run("footer preserved in round trip", func(t *testing.T) {
		f := newFooter(1096, 46, 1000, 92)

		encoded := encodeFooter(f)
		got, err := decodeFooter(encoded)
		if err != nil {
			t.Fatalf("error while decoding encoded data")
		}

		if got.bloomOffset != f.bloomOffset ||
			got.bloomSize != f.bloomSize ||
			got.indexOffset != f.indexOffset ||
			got.indexSize != f.indexSize {
			t.Fatalf("expected equal data after round trip, got: %v, expected: %v", got, f)
		}
	})

	t.Run("decoder returns error on truncated length", func(t *testing.T) {
		_, err := decodeFooter([]byte{0xff, 0x01, 0x12})
		if !errors.Is(err, ErrCorruptedFooter) {
			t.Fatalf("expected corrupted footer error on truncated length footer, got %v", err)
		}
	})

	t.Run("decoder returns error on corrupted payload", func(t *testing.T) {
		f := newFooter(1096, 46, 1000, 92)

		encoded := encodeFooter(f)
		encoded[1] = 0xff

		_, err := decodeFooter(encoded)
		if !errors.Is(err, ErrCorruptedFooter) {
			t.Fatalf("expected corrupted footer error on corrupted payload, got %v", err)
		}
	})
}
