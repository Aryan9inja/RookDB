package wal

import (
	"fmt"
	"os"
	"path/filepath"
)

// WAL manages the open WAL file.
type WAL struct {
	file *os.File
}

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
