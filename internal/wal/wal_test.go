package wal

import (
	"io"
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
