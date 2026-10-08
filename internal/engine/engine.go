package engine

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/Aryan9inja/RookDB/internal/memtable"
	"github.com/Aryan9inja/RookDB/internal/operation"
	"github.com/Aryan9inja/RookDB/internal/sstable"
	"github.com/Aryan9inja/RookDB/internal/wal"
)

type Engine struct {
	wal      *wal.WAL
	memTable *memtable.MemTable
	sstables []*sstable.Handler
}

var ErrKeyNotFound = errors.New("key not found")
var ErrUnsupportedCommand = errors.New("unsupported command")
var ErrStorageFailure = errors.New("storage failure")

func NewEngine(dataDir string) (*Engine, error) {
	w, err := wal.NewWAL(filepath.Join(dataDir, "rookdb.wal"))
	if err != nil {
		return nil, fmt.Errorf("new engine: new wal: %w", err)
	}

	handlers, err := sstable.Discover(dataDir)
	if err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("new engine: discover sstables: %w", err)
	}

	engine := &Engine{
		wal:      w,
		memTable: memtable.NewMemTable(),
		sstables: handlers,
	}

	if err := engine.recover(); err != nil {
		if closeErr := engine.Close(); closeErr != nil {
			return nil, fmt.Errorf(
				"new engine: recover: %w; close resources: %v",
				err,
				closeErr,
			)
		}

		return nil, fmt.Errorf("new engine: recover: %w", err)
	}

	return engine, nil
}

func (engine *Engine) Close() error {
	var closeErrs []error

	for _, h := range engine.sstables {
		if err := h.Close(); err != nil {
			closeErrs = append(closeErrs, err)
		}
	}

	if err := engine.wal.Close(); err != nil {
		closeErrs = append(closeErrs, err)
	}

	return errors.Join(closeErrs...)
}

func (engine *Engine) Execute(command, key, value string) (string, error) {
	switch command {
	case "SET":
		return "", engine.set(key, value)

	case "GET":
		return engine.get(key)

	case "DELETE":
		return "", engine.delete(key)

	default:
		return "", ErrUnsupportedCommand
	}
}

func (engine *Engine) set(key, value string) error {
	setOp := operation.Operation{
		OpType: operation.Set,
		Key:    key,
		Value:  value,
	}

	if err := validateOperation(setOp); err != nil {
		return err
	}

	if err := wal.Append(engine.wal, setOp); err != nil {
		return fmt.Errorf("%w: wal append: %w", ErrStorageFailure, err)
	}

	engine.applyOperation(setOp)

	return nil
}

func (engine *Engine) delete(key string) error {
	deleteOp := operation.Operation{
		OpType: operation.Delete,
		Key:    key,
	}

	if err := validateOperation(deleteOp); err != nil {
		return err
	}

	if err := wal.Append(engine.wal, deleteOp); err != nil {
		return fmt.Errorf("%w: wal append: %w", ErrStorageFailure, err)
	}

	engine.applyOperation(deleteOp)

	return nil
}

func (engine *Engine) get(key string) (string, error) {
	// look in memtable
	value, eType, exists := engine.memTable.Get(key)
	if exists {
		if eType == memtable.DeleteEntry {
			return "", ErrKeyNotFound
		} else {
			return value, nil
		}
	}

	// look in sstables sequentially
	for _, ss := range engine.sstables {
		value, eType, exists, err := ss.Get(key)
		if err != nil {
			return "", fmt.Errorf("error getting key %q from SSTable: %w", key, err)
		}
		if exists {
			if eType == sstable.DeleteEntry {
				return "", ErrKeyNotFound
			} else {
				return value, nil
			}
		}
	}

	return "", ErrKeyNotFound
}

func validateOperation(op operation.Operation) error {
	switch op.OpType {
	case operation.Set:
		if op.Key == "" || op.Value == "" {
			return errors.New("SET needs both key and value")
		}

	case operation.Delete:
		if op.Key == "" {
			return errors.New("DELETE needs key")
		}
		if op.Value != "" {
			return errors.New("DELETE does not need value")
		}

	default:
		return errors.New("unsupported operation")
	}

	return nil
}

// caller should call this only on valid operations
func (engine *Engine) applyOperation(op operation.Operation) {
	switch op.OpType {
	case operation.Set:
		engine.memTable.Put(op.Key, op.Value)

	case operation.Delete:
		engine.memTable.Delete(op.Key)
	}
}

func (engine *Engine) recover() error {
	for {
		op, err := wal.Next(engine.wal)
		if errors.Is(err, io.EOF) {
			// recovery succeeded
			return nil
		} else if err != nil {
			return fmt.Errorf("engine: recovery error: %w", err)
		}

		if err := validateOperation(*op); err != nil {
			if err := engine.wal.TruncateCurrentRecord(); err != nil {
				return fmt.Errorf("engine: recovery error: truncate invalid records: %w", err)
			}
			return fmt.Errorf("engine: recovery error: %w", err)
		}

		// Apply valid operation
		engine.applyOperation(*op)
	}
}
