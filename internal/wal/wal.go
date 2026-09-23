package wal

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/Aryan9inja/RookDB/internal/operation"
)

const MaxRecordSize = 512

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

	// Seek cursor position to start of file
	// This is used to recover cleanly
	// Invariant: a newly created WAL starts with its cursor at offset 0.
	if _, err := fd.Seek(0, io.SeekStart); err != nil {
		fd.Close()
		return nil, fmt.Errorf("seek WAL: %w", err)
	}

	return &WAL{file: fd}, nil
	// TODO : After WAL lifecycle is clear, we need a close call on fd
}

// The caller is responsible for closing the WAL.
func (w *WAL) Close() error {
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("wal close: %w", err)
	}

	return nil
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

	if len(buffer) > MaxRecordSize {
		return false, errors.New("large payload error")
	}

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

func Next(wal *WAL) (*operation.Operation, error) {
	currOffset, err := wal.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("recover: offset seeking: %w", err)
	}

	lengthBuffer := make([]byte, 4)

	_, err = io.ReadFull(wal.file, lengthBuffer)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		} else if errors.Is(err, io.ErrUnexpectedEOF) {
			if err := truncateWAL(wal, currOffset); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("recover: read error: length: %w", err)
		} else {
			return nil, fmt.Errorf("recover: read error: length: %w", err)
		}
	}

	length := binary.BigEndian.Uint32(lengthBuffer)
	if length > MaxRecordSize-8 {
		if err := truncateWAL(wal, currOffset); err != nil {
			return nil, err
		}
		return nil, errors.New("recover: max record size exceeded")
	}

	payloadBuffer := make([]byte, length)
	_, err = io.ReadFull(wal.file, payloadBuffer)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			if err := truncateWAL(wal, currOffset); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("recover: read error: payload: %w", err)
		} else {
			return nil, fmt.Errorf("recover: read error: payload: %w", err)
		}
	}

	checksumBuffer := make([]byte, 4)
	_, err = io.ReadFull(wal.file, checksumBuffer)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			if err := truncateWAL(wal, currOffset); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("recover: read error: checksum: %w", err)
		} else {
			return nil, fmt.Errorf("recover: read error: checksum: %w", err)
		}
	}

	// length buffer work is done as we extracted length so reusing it
	lengthBuffer = append(lengthBuffer, payloadBuffer...)
	verifier := crc32.ChecksumIEEE(lengthBuffer)
	verifierBuff := make([]byte, 4)
	binary.BigEndian.PutUint32(verifierBuff, verifier)

	if !slices.Equal(checksumBuffer, verifierBuff) {
		if err := truncateWAL(wal, currOffset); err != nil {
			return nil, err
		}
		return nil, errors.New("recover: checksum mismatch")
	}

	var op operation.Operation
	if err := json.Unmarshal(payloadBuffer, &op); err != nil {
		if err := truncateWAL(wal, currOffset); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("recover: json unmarshal: %w", err)
	}

	return &op, nil
}

func truncateWAL(wal *WAL, truncateOffset int64) error {
	if err := wal.file.Truncate(truncateOffset); err != nil {
		return fmt.Errorf("recover: wal truncate: %w", err)
	}
	if _, err := wal.file.Seek(truncateOffset, io.SeekStart); err != nil {
		return fmt.Errorf("recover: seek after truncate: %w", err)
	}
	return nil
}
