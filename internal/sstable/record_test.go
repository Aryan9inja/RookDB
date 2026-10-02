package sstable

import (
	"bytes"
	"errors"
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

func TestDecodeRecord(t *testing.T) {
	t.Run("valid set", func(t *testing.T) {
		data := []byte{
			0x01,
			0x00, 0x05,
			0x00, 0x05,
			'h', 'e', 'l', 'l', 'o',
			'w', 'o', 'r', 'l', 'd',
		}

		got, err := DecodeRecord(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.rType != setRecord || got.key != "hello" || got.value != "world" {
			t.Errorf("got %v, want setRecord hello world", got)
		}
	})

	t.Run("valid delete", func(t *testing.T) {
		data := []byte{
			0x02,
			0x00, 0x05,
			0x00, 0x00,
			'h', 'e', 'l', 'l', 'o',
		}

		got, err := DecodeRecord(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.rType != deleteRecord || got.key != "hello" || got.value != "" {
			t.Errorf("got %v, want deleteRecord hello", got)
		}
	})

	t.Run("less than 5 bytes data", func(t *testing.T) {
		data := []byte{0x01, 0x00, 0x05, 0x00}
		_, err := DecodeRecord(data)
		if err == nil {
			t.Error("expected error for less than 5 bytes data")
		} else if !errors.Is(err, ErrInvalidSSTableRecord) {
			t.Errorf("expected ErrInvalidSSTableRecord, got %v", err)
		}
	})

	t.Run("unknown record type", func(t *testing.T) {
		data := []byte{
			0x03,
			0x00, 0x05,
			0x00, 0x05,
			'h', 'e', 'l', 'l', 'o',
			'w', 'o', 'r', 'l', 'd',
		}
		_, err := DecodeRecord(data)
		if err == nil {
			t.Error("expected error for unknown record type")
		} else if !errors.Is(err, ErrInvalidSSTableRecord) {
			t.Errorf("expected ErrInvalidSSTableRecord, got %v", err)
		}
	})

	t.Run("delete with value", func(t *testing.T) {
		data := []byte{
			0x02,
			0x00, 0x05,
			0x00, 0x05,
			'h', 'e', 'l', 'l', 'o',
			'w', 'o', 'r', 'l', 'd',
		}
		_, err := DecodeRecord(data)
		if err == nil {
			t.Error("expected error for delete with value")
		} else if !errors.Is(err, ErrInvalidSSTableRecord) {
			t.Errorf("expected ErrInvalidSSTableRecord, got %v", err)
		}
	})

	t.Run("truncated key value", func(t *testing.T) {
		data := []byte{
			0x01,
			0x00, 0x05,
			0x00, 0x05,
			'h', 'e', 'l', 'l', 'o',
			'w', 'o', 'r', 'l', // missing 'd'
		}
		_, err := DecodeRecord(data)
		if err == nil {
			t.Error("expected error for truncated key value")
		} else if !errors.Is(err, ErrInvalidSSTableRecord) {
			t.Errorf("expected ErrInvalidSSTableRecord, got %v", err)
		}
	})

	t.Run("trailing bytes", func(t *testing.T) {
		data := []byte{
			0x01,
			0x00, 0x05,
			0x00, 0x05,
			'h', 'e', 'l', 'l', 'o',
			'w', 'o', 'r', 'l', 'd',
			'e', 'x', 't', 'r', 'a',
		}
		_, err := DecodeRecord(data)
		if err == nil {
			t.Error("expected error for trailing bytes")
		} else if !errors.Is(err, ErrInvalidSSTableRecord) {
			t.Errorf("expected ErrInvalidSSTableRecord, got %v", err)
		}
	})

	t.Run("record to encode to decode to record", func(t *testing.T) {
		orig := &record{
			rType: setRecord,
			key:   "roundtrip",
			value: "test",
		}

		encoded := orig.Encode()
		decoded, err := DecodeRecord(encoded)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if decoded.rType != orig.rType || decoded.key != orig.key || decoded.value != orig.value {
			t.Errorf("got %v, want %v", decoded, orig)
		}
	})
}
