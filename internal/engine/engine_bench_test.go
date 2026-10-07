package engine

import (
	"fmt"
	"path/filepath"
	"testing"
)

func BenchmarkEngineSet(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.wal")

	engine, err := NewEngine(path)
	if err != nil {
		b.Fatalf("benchmark set: engine init failed: %v", err)
	}
	defer engine.Close()

	const poolSize = 10000
	keys := make([]string, poolSize)
	values := make([]string, poolSize)
	for i := range poolSize {
		keys[i] = fmt.Sprintf("key%d", i)
		values[i] = fmt.Sprintf("value%d", i)
	}

	b.ResetTimer()

	for i := range b.N {
		idx := i % poolSize
		_, err := engine.Execute("SET", keys[idx], values[idx])
		if err != nil {
			b.Fatalf("benchmark set: set operation failed: %v", err)
		}
	}
}

func BenchmarkEngineGet(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.wal")

	engine, err := NewEngine(path)
	if err != nil {
		b.Fatalf("benchmark set: engine init failed: %v", err)
	}
	defer engine.Close()

	const poolSize = 10000
	keys := make([]string, poolSize)
	for i := range poolSize {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)

		keys[i] = key
		engine.set(key, value)
	}

	b.ResetTimer()

	for i := range b.N {
		idx := i % poolSize
		_, err := engine.Execute("GET", keys[idx], "")
		if err != nil {
			b.Fatalf("benchmark get: get operation failed: %v", err)
		}
	}
}

func BenchmarkEngineDelete(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.wal")

	engine, err := NewEngine(path)
	if err != nil {
		b.Fatalf("benchmark delete: engine init failed: %v", err)
	}
	defer engine.Close()

	const batchSize = 10000
	keys := make([]string, batchSize)
	for i := range batchSize {
		keys[i] = fmt.Sprintf("key%d", i)
	}

	b.ResetTimer()

	for i := 0; i < b.N; {
		b.StopTimer()

		chunk := batchSize
		if remaining := b.N - i; remaining < batchSize {
			chunk = remaining
		}

		for j := 0; j < chunk; j++ {
			if _, err := engine.Execute("SET", keys[j], "value"); err != nil {
				b.Fatalf("benchmark delete: set operation failed: %v", err)
			}
		}

		b.StartTimer()
		for j := 0; j < chunk; j++ {
			if _, err := engine.Execute("DELETE", keys[j], ""); err != nil {
				b.Fatalf("benchmark delete: delete operation failed: %v", err)
			}
		}

		i += chunk
	}
}
