package sstable

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"testing"
)

func TestCreateBlock(t *testing.T) {
	generateRecords := func(n int) []*record {
		var records []*record
		for i := range n {
			records = append(records, &record{
				rType: setRecord,
				key:   fmt.Sprintf("key%d", i),
				value: fmt.Sprintf("val%d", i),
			})
		}
		return records
	}

	tests := []struct {
		name    string
		records []*record
	}{
		{
			name: "one SET record",
			records: []*record{
				{rType: setRecord, key: "key1", value: "val1"},
			},
		},
		{
			name: "one DELETE record",
			records: []*record{
				{rType: deleteRecord, key: "key1", value: ""},
			},
		},
		{
			name: "multiple records",
			records: []*record{
				{rType: setRecord, key: "key1", value: "val1"},
				{rType: deleteRecord, key: "key2", value: ""},
				{rType: setRecord, key: "key3", value: "val3"},
			},
		},
		{
			name:    "fewer than 16 records",
			records: generateRecords(10),
		},
		{
			name:    "exactly 16 records",
			records: generateRecords(16),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, offsetDelta := CreateBlock(tt.records)

			expectedDataSize := 0
			for _, rec := range tt.records {
				expectedDataSize += 5 + len(rec.key) + len(rec.value)
			}
			expectedTotalSize := 4 + expectedDataSize + 4

			if len(block) != expectedTotalSize {
				t.Fatalf("expected block length %d, got %d", expectedTotalSize, len(block))
			}

			t.Run("returned offsetDelta == len(block)", func(t *testing.T) {
				if offsetDelta != uint32(len(block)) {
					t.Errorf("expected offsetDelta %d to equal len(block) %d", offsetDelta, len(block))
				}
			})

			t.Run("CRC validates over BLOCK_LEN + BLOCK_DATA", func(t *testing.T) {
				// BLOCK_LEN + BLOCK_DATA is everything except the last 4 bytes (the CRC itself)
				expectedChecksum := crc32.ChecksumIEEE(block[:len(block)-4])
				actualChecksum := binary.BigEndian.Uint32(block[len(block)-4:])
				if actualChecksum != expectedChecksum {
					t.Errorf("checksum mismatch: expected %d, got %d", expectedChecksum, actualChecksum)
				}
			})
		})
	}
}
