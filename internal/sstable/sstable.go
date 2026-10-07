package sstable

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
)

type Handler struct {
	name string
	path string
}

func newHandler(path string) *Handler {
	return &Handler{
		name: filepath.Base(path),
		path: path,
	}
}

func Discover(dataDir string) ([]*Handler, error) {
	path, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, fmt.Errorf("error discovering data dir: %w", err)
	}

	matches, err := filepath.Glob(filepath.Join(path, "*.sst"))
	if err != nil {
		return nil, fmt.Errorf("error finding sstable file in %v", path)
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i] > matches[j]
	})

	handlers := make([]*Handler, len(matches))
	for i, file := range matches {
		if err := parseFileName(file); err != nil {
			return nil, fmt.Errorf("error in file validation: %v: %w", file, err)
		}

		handlers[i] = &Handler{
			name: filepath.Base(file),
			path: file,
		}
	}

	return handlers, nil
}

func parseFileName(file string) error {
	fileName := filepath.Base(file)

	ok, err := regexp.MatchString(`[0-9]{6}\.sst`, fileName)
	if err != nil {
		return fmt.Errorf("error validating file name: %w", err)
	}

	if !ok {
		return fmt.Errorf("unrecognized file format for sstable")
	}

	return nil
}
