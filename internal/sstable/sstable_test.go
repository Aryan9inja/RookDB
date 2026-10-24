package sstable

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover(t *testing.T) {
	dir := t.TempDir()

	filePath := filepath.Join(dir, "000001.sst")

	os.Create(filePath)

	h, _:=Discover(dir)
	if len(h) != 1{
		t.Fatalf("fuck man")
	}
	if h[0].Name() != "000001"{
		t.Fatalf("fuck again")
	}
}