package sstable

import "path/filepath"

type Handler struct{
	name string
	path string
}

func newHandler(path string) *Handler{
	return &Handler{
		name: filepath.Base(path),
		path: path,
	}
}