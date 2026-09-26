package engine

import (
	"errors"
	"fmt"
	"io"

	"github.com/Aryan9inja/RookDB/internal/operation"
	"github.com/Aryan9inja/RookDB/internal/wal"
)

type Engine struct {
	wal   *wal.WAL
	store map[string]string
}

var ErrKeyNotFound = errors.New("key not found")

func NewEngine(path string) (*Engine, error) {
	w, err := wal.NewWAL(path)
	if err != nil {
		return nil, fmt.Errorf("new engine: new wal: %w", err)
	}

	engine := &Engine{
		wal:   w,
		store: make(map[string]string),
	}

	if err := engine.recover(); err != nil {
		if closeErr := engine.wal.Close(); closeErr != nil {
			return nil, fmt.Errorf(
				"new engine: recover: %w; close wal: %v",
				err,
				closeErr,
			)
		}

		return nil, fmt.Errorf("new engine: recover: %w", err)
	}

	return engine, nil
}

func (engine *Engine) Close() error {
	return engine.wal.Close()
}

func (engine *Engine) Set(key, value string) error {
	setOp := operation.Operation{
		OpType: operation.Set,
		Key:    key,
		Value:  value,
	}

	if err := validateOperation(setOp); err != nil {
		return err
	}

	if err := wal.Append(engine.wal, setOp); err != nil {
		return fmt.Errorf("wal append: %w", err)
	}

	engine.applyOperation(setOp)

	return nil
}

func (engine *Engine) Delete(key string) error {
	deleteOp := operation.Operation{
		OpType: operation.Delete,
		Key:    key,
	}

	if err := validateOperation(deleteOp); err != nil {
		return err
	}

	if err := wal.Append(engine.wal, deleteOp); err != nil {
		return fmt.Errorf("wal append: %w", err)
	}

	engine.applyOperation(deleteOp)

	return nil
}

func (engine *Engine) Get(key string) (string, error) {
	value, exists := engine.store[key]
	if !exists {
		return "", ErrKeyNotFound
	}

	return value, nil
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
		engine.store[op.Key] = op.Value

	case operation.Delete:
		delete(engine.store, op.Key)
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
