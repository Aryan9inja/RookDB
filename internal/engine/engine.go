package engine

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Aryan9inja/RookDB/internal/memtable"
	"github.com/Aryan9inja/RookDB/internal/operation"
	"github.com/Aryan9inja/RookDB/internal/sstable"
	"github.com/Aryan9inja/RookDB/internal/wal"
)

type Engine struct {
	dataDir string
	currGen string

	wal      *wal.WAL
	memTable *memtable.MemTable

	frozenWal      *wal.WAL
	frozenMemTable *memtable.MemTable

	sstables []*sstable.Handler
}

var ErrKeyNotFound = errors.New("key not found")
var ErrUnsupportedCommand = errors.New("unsupported command")
var ErrStorageFailure = errors.New("storage failure")

func NewEngine(dataDir string) (*Engine, error) {
	handlers, err := sstable.Discover(dataDir)
	if err != nil {
		return nil, fmt.Errorf("new engine: discover sstables: %w", err)
	}

	engine := &Engine{
		dataDir:  dataDir,
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

func (engine *Engine) CreateNewGeneration() error {
	currGenInt, err := strconv.Atoi(engine.currGen)
	if err != nil {
		return fmt.Errorf("error parsing current gen: %w", err)
	}

	nextGenInt := currGenInt + 1
	nextGen := fmt.Sprintf("%06d", nextGenInt)

	newWal, err := wal.NewWAL(filepath.Join(engine.dataDir, fmt.Sprintf("%s.wal", nextGen)))
	if err != nil {
		return fmt.Errorf("error creating new gen wal: %w", err)
	}

	// freeze old wal and memtable
	engine.frozenWal = engine.wal
	engine.frozenMemTable = engine.memTable

	// assign new wal and memTable
	engine.wal = newWal
	engine.memTable = memtable.NewMemTable()
	engine.currGen = nextGen

	return nil
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

	if engine.memTable.NeedFlush() {

	}
}

func (engine *Engine) recover() error {
	wals, err := filepath.Glob(filepath.Join(engine.dataDir, "*.wal"))
	if err != nil {
		return fmt.Errorf("error getting wal names for recovery: %w", err)
	}

	for i, w := range wals {
		wals[i] = strings.TrimSuffix(filepath.Base(w), ".wal")
		ok, err := regexp.MatchString(`^[0-9]{6}$`, wals[i])
		if err != nil {
			return fmt.Errorf("error validating wal file name: %w", err)
		}
		if !ok {
			return fmt.Errorf("unrecognized file format for wal")
		}
	}

	slices.Sort(wals)
	sstableSet := make(map[string]struct{}, len(engine.sstables))
	for _, s := range engine.sstables {
		sstableSet[s.Name()] = struct{}{}
	}

	var currRecoveredGen string
	for _, w := range wals {
		if _, ok := sstableSet[w]; !ok {
			currRecoveredGen = w
			if err := recoverGeneration(filepath.Join(engine.dataDir, w+".wal"), engine.memTable); err != nil {
				return fmt.Errorf("error recovering wal gen: %w", err)
			}
		}
	}

	if currRecoveredGen != "" {
		oldWal, err := wal.NewWAL(filepath.Join(engine.dataDir, fmt.Sprintf("%s.wal", currRecoveredGen)))
		if err != nil {
			return fmt.Errorf("error opening old gen wal: %w", err)
		}

		engine.currGen = currRecoveredGen
		engine.wal = oldWal
	} else {
		if err := engine.newGenDuringRecovery(); err != nil {
			return fmt.Errorf("error creating new generation: %w", err)
		}
	}

	return nil
}

func (engine *Engine) newGenDuringRecovery() error {
	var nextGen string

	if len(engine.sstables) == 0 {
		nextGen = "000001"
	} else {
		lastGen := engine.sstables[0].Name()

		lastGenInt, err := strconv.Atoi(lastGen)
		if err != nil {
			return fmt.Errorf("error parsing last gen: %w", err)
		}

		nextGenInt := lastGenInt + 1
		nextGen = fmt.Sprintf("%06d", nextGenInt)
	}

	newWal, err := wal.NewWAL(filepath.Join(engine.dataDir, fmt.Sprintf("%s.wal", nextGen)))
	if err != nil {
		return fmt.Errorf("error creating new gen wal: %w", err)
	}

	engine.wal = newWal
	engine.currGen = nextGen

	return nil
}

func recoverGeneration(walPath string, memT *memtable.MemTable) error {
	w, err := wal.NewWAL(walPath)
	if err != nil {
		return fmt.Errorf("error opening wal file: %w", err)
	}
	defer w.Close()

	for {
		op, err := wal.Next(w)
		if errors.Is(err, io.EOF) {
			// recovery succeeded
			return nil
		} else if err != nil {
			return fmt.Errorf("engine: recovery error: %w", err)
		}

		if err := validateOperation(*op); err != nil {
			if err := w.TruncateCurrentRecord(); err != nil {
				return fmt.Errorf("engine: recovery error: truncate invalid records: %w", err)
			}
			return fmt.Errorf("engine: recovery error: %w", err)
		}

		// Apply valid operation
		switch op.OpType {
		case operation.Set:
			memT.Put(op.Key, op.Value)

		case operation.Delete:
			memT.Delete(op.Key)
		}
	}
}
