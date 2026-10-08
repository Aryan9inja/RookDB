package sstable

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

type Handler struct {
	name   string
	path   string
	reader *SSTableReader
}

func newHandler(path string) (*Handler, error) {
	ssReader, err := newSSTableReader(path)
	if err != nil {
		return nil, fmt.Errorf("error creating sstable reader: %w", err)
	}

	return &Handler{
		name:   filepath.Base(path),
		path:   path,
		reader: ssReader,
	}, nil
}

func (h *Handler) Get(target string) (string, recordType, bool, error) {
	return h.reader.get(target)
}

func Discover(dataDir string) ([]*Handler, error) {
	path, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, fmt.Errorf("error discovering data dir: %w", err)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("error reading data directory %q: %w", path, err)
	}

	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sst" {
			matches = append(matches, filepath.Join(path, entry.Name()))
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i] > matches[j]
	})

	handlers := make([]*Handler, len(matches))
	for i, file := range matches {
		if err := parseFileName(file); err != nil {
			if err := cleanupDiscover(handlers); err != nil {
				return nil, fmt.Errorf("error cleaning up handlers after validation faliure: %w", err)
			}
			return nil, fmt.Errorf("error in file validation: %s: %w", file, err)
		}

		handlers[i], err = newHandler(file)
		if err != nil {
			if err := cleanupDiscover(handlers); err != nil {
				return nil, fmt.Errorf("error cleaning up handlers after handler construction faliure: %w", err)
			}
			return nil, fmt.Errorf("error creating handler for sstable path: %s: with error: %w", file, err)
		}
	}

	return handlers, nil
}

func parseFileName(file string) error {
	fileName := filepath.Base(file)

	ok, err := regexp.MatchString(`^[0-9]{6}\.sst$`, fileName)
	if err != nil {
		return fmt.Errorf("error validating file name: %w", err)
	}

	if !ok {
		return fmt.Errorf("unrecognized file format for sstable")
	}

	return nil
}

func cleanupDiscover(handlers []*Handler) error {
	var allErrors []error
	for _, h := range handlers {
		if h == nil {
			continue
		}
		if err := h.reader.close(); err != nil {
			allErrors = append(allErrors, err)
		}
	}
	return errors.Join(allErrors...)
}
