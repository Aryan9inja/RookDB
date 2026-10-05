package sstable

import (
	"fmt"
	"os"
)

type SSTableReader struct {
	fd   *os.File
	foot *footer
}

func NewSSTableReader(path string) (*SSTableReader, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening sstable file: %w", err)
	}

	return &SSTableReader{
		fd:   fd,
		foot: nil,
	}, nil
}

func (reader *SSTableReader) Close() error {
	if err := reader.fd.Close(); err != nil {
		return fmt.Errorf("error closing sstable file: %w", err)
	}
	return nil
}

func (reader *SSTableReader) footerReader() error {
	info, err := reader.fd.Stat()
	if err != nil {
		return fmt.Errorf("footer reader: file Stat: %w", err)
	}
	fileSize := info.Size()

	footerOffset := fileSize - footerSize
	buffer := make([]byte, footerSize)

	_, err = reader.fd.ReadAt(buffer, footerOffset)
	if err != nil {
		return fmt.Errorf("footer reader: readAt: %w", err)
	}

	footer, err := decodeFooter(buffer)
	if err != nil {
		return fmt.Errorf("footer reader: decode footer: %w", err)
	}

	reader.foot = footer

	return nil
}
