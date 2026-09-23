package wal

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Aryan9inja/RookDB/internal/operation"
)

func TestWAL(t *testing.T) {
	tests := []struct {
		name string
		ops  []operation.Operation
	}{
		{
			name: "append and recover one operation",
			ops: []operation.Operation{
				{
					OpType: operation.Set,
					Key:    "name",
					Value:  "Aryan",
				},
			},
		},
		{
			name: "multiple operations",
			ops: []operation.Operation{
				{
					OpType: operation.Set,
					Key:    "name",
					Value:  "Aryan",
				},
				{
					OpType: operation.Set,
					Key:    "age",
					Value:  "22",
				},
				{
					OpType: operation.Delete,
					Key:    "name",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// setup wal
			wal, path := newTestWAL(t)

			// test append
			for _, op := range tt.ops {
				ok, err := Append(wal, op)
				if !ok || err != nil {
					t.Fatalf("wal test: append happy path: %v", err)
				}
			}

			// simulate restart
			wal2 := reopenTestWAL(t, wal, path)
			defer wal2.Close()

			// test recover
			var recovered []*operation.Operation

			for {
				rec, err := Next(wal2)
				if err != nil {
					if err == io.EOF {
						break
					}
					t.Fatalf("wal test: recover happy path: %v", err)
				}
				recovered = append(recovered, rec)
				if len(recovered) > len(tt.ops) {
					t.Fatalf("wal test: expected EOF after %d next calls", len(tt.ops))
				}
			}

			// test assertion
			if len(recovered) != len(tt.ops) {
				t.Fatalf("wal test: expected %d recovered operations, got %d",
					len(tt.ops), len(recovered))
			}

			for i := 0; i < len(tt.ops); i++ {
				if tt.ops[i] != *recovered[i] {
					t.Fatalf("wal test: assertion: data mismatch")
				}
			}
		})
	}

	t.Run("incomplete length", func(t *testing.T) {
		wal, path := newTestWAL(t)

		incompleteLength := []byte{0x01, 0x02, 0x03}
		writeRawWAL(t, path, incompleteLength)

		wal2 := reopenTestWAL(t, wal, path)
		defer wal2.Close()

		_, err := Next(wal2)
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("wal test: incomplete length: expected: %v, got %v", io.ErrUnexpectedEOF, err)
		}

		// Check file size after truncate
		// Should be zero
		info, err := wal2.file.Stat()
		if err != nil {
			t.Fatalf("wal test: incomplete length: stat WAL: %v", err)
		}

		if info.Size() != 0 {
			t.Fatalf("wal test: incomplete length: expected file size to be 0 after truncate, got: %d", info.Size())
		}

		_, err = Next(wal2)
		if !errors.Is(err, io.EOF) {
			t.Fatalf("wal test: incomplete length: expected: %v, got %v", io.EOF, err)
		}
	})
}

func newTestWAL(t *testing.T) (*WAL, string) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "wal.log")

	wal, err := NewWAL(path)
	if err != nil {
		t.Fatalf("setup WAL: %v", err)
	}

	return wal, path
}

func reopenTestWAL(t *testing.T, wal *WAL, path string) *WAL {
	t.Helper()

	if err := wal.Close(); err != nil {
		t.Fatalf("close WAL: %v", err)
	}

	wal, err := NewWAL(path)
	if err != nil {
		t.Fatalf("reopen WAL: %v", err)
	}

	return wal
}

func writeRawWAL(t *testing.T, path string, raw []byte) {
	t.Helper()

	fd, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("write raw WAL: %v", err)
	}
	defer fd.Close()

	n, err := fd.Write(raw)
	if err != nil {
		t.Fatalf("write raw WAL: %v", err)
	}

	if n != len(raw) {
		t.Fatalf("write raw WAL: short write: wrote %d of %d bytes", n, len(raw))
	}

	if err := fd.Sync(); err != nil {
		t.Fatalf("sync write WAL: %v", err)
	}
}
