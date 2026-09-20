package wal

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"

	"github.com/Aryan9inja/RookDB/internal/operation"
)

// WAL manages the open WAL file.
type WAL struct {
	file *os.File
}

// New WAL initialize a WAL struct in memory.
// This gives us open file descriptor of the wal file.
func NewWAL(path string) (*WAL, error) {
	// Generate absolute path to WAL
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("absolutePath WAL: %w", err)
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return nil, fmt.Errorf("mkdirall WAL: %w", err)
	}

	// Open file with append mode
	fd, err := os.OpenFile(absPath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0640)
	if err != nil {
		return nil, fmt.Errorf("open WAL: %w", err)
	}

	return &WAL{file: fd}, nil
	// TODO : After WAL lifecycle is clear, we need a close call on fd
}

// Used to append operations into wal file.
// Internally does serialization and checksum generation.
func Append(wal *WAL, op operation.Operation) (bool, error) {
	// Serialize operation to form json bytes
	payload, err := json.Marshal(op)
	if err != nil {
		return false, fmt.Errorf("payload creation WAL: %w", err)
	}

	// Get length of payload in uint32
	length := uint32(len(payload))

	// Put length to a buffer in big endian
	buffer := make([]byte, 4)
	binary.BigEndian.PutUint32(buffer, length)

	// Append payload to buffer
	buffer = append(buffer, payload...)

	// Generate checksum and convert to big endian
	checksum := crc32.ChecksumIEEE(buffer)
	chBuff := make([]byte, 4)
	binary.BigEndian.PutUint32(chBuff, checksum)

	// Create length + payload + checksum
	buffer = append(buffer, chBuff...)

	// Write the buffer to disk
	n, err := wal.file.Write(buffer)
	if err != nil {
		return false, fmt.Errorf("write WAL: %w", err)
	}
	if n < len(buffer) {
		return false, fmt.Errorf("short write: wrote %d of %d bytes", n, len(buffer))
	}

	// Sync to ensure durability
	if err := wal.file.Sync(); err != nil {
		return false, fmt.Errorf("sync write WAL: %w", err)
	}

	return true, nil
}
