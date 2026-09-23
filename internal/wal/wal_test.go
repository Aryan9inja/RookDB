package wal

import (
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/Aryan9inja/RookDB/internal/operation"
)

func TestWAL(t *testing.T) {
	// setup wal
	dir := t.TempDir()
	path := filepath.Join(dir, "wal.log")
	wal, err := NewWAL(path)
	if err != nil {
		t.Fatalf("wal test: wal setup: %v", err)
	}

	// test append
	op := operation.Operation{
		OpType: operation.Set,
		Key:    "name",
		Value:  "Aryan",
	}

	ok, err := Append(wal, op)
	if !ok || err != nil {
		t.Fatalf("wal test: append happy path: %v", err)
	}

	// simulate restart
	if err := wal.Close(); err != nil {
		t.Fatalf("wal test: wal close: %v", err)
	}
	wal2, err := NewWAL(path)
	if err != nil {
		t.Fatalf("wal test: wal restart: %v", err)
	}
	defer wal2.Close()

	// test recover
	recovered, err := Next(wal2)
	if err != nil {
		t.Fatalf("wal test: recover happy path: %v", err)
	}

	// test assertion
	if op != *recovered {
		t.Fatalf("wal test: assertion: data mismatch")
	}

	_, err = Next(wal2)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("wal test: expected EOF, got: %v", err)
	}
}
