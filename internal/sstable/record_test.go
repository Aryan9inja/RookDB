package sstable

import (
	"bytes"
	"testing"
)

func TestEncode(t *testing.T) {
	t.Run("encode set record", func(t *testing.T) {
		r := &record{
			rType: setRecord,
			key:   "hello",
			value: "world",
		}

		got := r.Encode()

		expected := []byte{
			0x01,
			0x00, 0x05,
			0x00, 0x05,
			'h', 'e', 'l', 'l', 'o',
			'w', 'o', 'r', 'l', 'd',
		}

		if !bytes.Equal(got, expected) {
			t.Errorf("got %v, expected %v", got, expected)
		}
	})

	t.Run("encode delete record", func(t *testing.T) {
		r := &record{
			rType: deleteRecord,
			key:   "hello",
			value: "",
		}

		got := r.Encode()

		expected := []byte{
			0x02,
			0x00, 0x05,
			0x00, 0x00,
			'h', 'e', 'l', 'l', 'o',
		}

		if !bytes.Equal(got, expected) {
			t.Errorf("got %v, expected %v", got, expected)
		}
	})
}
