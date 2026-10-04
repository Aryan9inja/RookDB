package sstable

import (
	"fmt"
	"os"
)

type SSTableReader struct {
	fd *os.File
}

func NewSSTableReader(path string) (*SSTableReader, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening sstable file: %w", err)
	}

	return &SSTableReader{
		fd: fd,
	}, nil
}

func (reader *SSTableReader) Close() error {
	if err := reader.fd.Close(); err != nil {
		return fmt.Errorf("error closing sstable file: %w", err)
	}
	return nil
}
