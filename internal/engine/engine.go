package engine

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

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

	frozenGen      string
	frozenWal      *wal.WAL
	frozenMemTable *memtable.MemTable
	frozenSSTable  *sstable.Handler

	sstables []*sstable.Handler

	flushJobs    chan flushJob
	flushResults chan flushResult

	closeOnce sync.Once
	wg        sync.WaitGroup

	lifecycleMu sync.Mutex
	closing     bool
	activeOps   sync.WaitGroup

	flushErrMu sync.Mutex
	flushErr   error

	closeErr error
}

type flushJob struct {
	dataDir    string
	generation string
	wal        *wal.WAL
	memTable   *memtable.MemTable
}

type flushResult struct {
	generation string
	tempPath   string
	err        error
}

var ErrKeyNotFound = errors.New("key not found")
var ErrUnsupportedCommand = errors.New("unsupported command")
var ErrStorageFailure = errors.New("storage failure")
var ErrCannotRotateGen = errors.New("cannot rotate generation")
var ErrEngineClosed = errors.New("engine is closing or closed")

func NewEngine(dataDir string) (*Engine, error) {
	handlers, err := sstable.Discover(dataDir)
	if err != nil {
		return nil, fmt.Errorf("new engine: discover sstables: %w", err)
	}

	engine := &Engine{
		dataDir:      dataDir,
		memTable:     memtable.NewMemTable(),
		sstables:     handlers,
		flushJobs:    make(chan flushJob),
		flushResults: make(chan flushResult),
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

	// spawn go routine to flush in background
	engine.wg.Add(2)
	go engine.flushWorker()
	go engine.flushResConsumer()

	return engine, nil
}

func (engine *Engine) Close() error {
	engine.lifecycleMu.Lock()
	engine.closing = true
	engine.lifecycleMu.Unlock()

	engine.activeOps.Wait()

	engine.closeOnce.Do(func() {
		var closeErrs []error

		// Stop submissions and drain the flush pipeline.
		close(engine.flushJobs)
		engine.wg.Wait()

		// Report background flush failures.
		if err := engine.getFlushError(); err != nil {
			closeErrs = append(closeErrs, err)
		}

		// Close all registered SSTable handlers.
		for _, h := range engine.sstables {
			if err := h.Close(); err != nil {
				closeErrs = append(closeErrs, err)
			}
		}

		// Close the frozen WAL, but preserve its file on disk.
		if engine.frozenWal != nil {
			if err := engine.frozenWal.Close(); err != nil {
				closeErrs = append(closeErrs, fmt.Errorf("close frozen WAL: %w", err))
			}
		}

		// Close the active WAL.
		if engine.wal != nil {
			if err := engine.wal.Close(); err != nil {
				closeErrs = append(closeErrs, fmt.Errorf("close active WAL: %w", err))
			}
		}

		engine.closeErr = errors.Join(closeErrs...)
	})

	return engine.closeErr
}

func (engine *Engine) Flush() error {
	if err := engine.beginOperation(); err != nil {
		return err
	}
	defer engine.endOperation()

	err := engine.CreateNewGeneration()
	if errors.Is(err, ErrCannotRotateGen) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("error flushing to storage: %w", err)
	}

	job := flushJob{
		dataDir:    engine.dataDir,
		generation: engine.frozenGen,
		wal:        engine.frozenWal,
		memTable:   engine.frozenMemTable,
	}

	engine.flushJobs <- job
	return nil
}

func (engine *Engine) flushWorker() {
	defer engine.wg.Done()
	defer close(engine.flushResults)

	for job := range engine.flushJobs {
		result := job.flushGeneration()
		engine.flushResults <- *result
	}
}

func (engine *Engine) flushResConsumer() {
	defer engine.wg.Done()

	for res := range engine.flushResults {
		if err := engine.handleFlushResult(res); err != nil {
			engine.setFlushError(err)
		}
	}
}

func (engine *Engine) setFlushError(err error) {
	engine.flushErrMu.Lock()
	defer engine.flushErrMu.Unlock()

	engine.flushErr = errors.Join(engine.flushErr, err)
}

func (engine *Engine) getFlushError() error {
	engine.flushErrMu.Lock()
	defer engine.flushErrMu.Unlock()

	return engine.flushErr
}

func (engine *Engine) handleFlushResult(res flushResult) error {
	if res.err != nil {
		return fmt.Errorf("flush generation %s: %w", res.generation, res.err)
	}

	if res.generation != engine.frozenGen {
		return fmt.Errorf(
			"expected generation %s, got %s",
			engine.frozenGen,
			res.generation,
		)
	}

	if engine.frozenSSTable == nil {
		if err := engine.publishFrozenSSTable(res); err != nil {
			return err
		}
	}

	return engine.reclaimFrozenWAL()
}

func (engine *Engine) publishFrozenSSTable(res flushResult) error {
	finalPath := filepath.Join(
		engine.dataDir,
		fmt.Sprintf("%s.sst", res.generation),
	)

	if _, err := os.Stat(finalPath); errors.Is(err, os.ErrNotExist) {
		if err := os.Rename(res.tempPath, finalPath); err != nil {
			return fmt.Errorf("publish SSTable: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("stat published SSTable: %w", err)
	}

	handler, err := sstable.Open(finalPath)
	if err != nil {
		return fmt.Errorf("open published SSTable: %w", err)
	}

	engine.sstables = append([]*sstable.Handler{handler}, engine.sstables...)
	engine.frozenSSTable = handler

	return nil
}

func (engine *Engine) reclaimFrozenWAL() error {
	walPath := filepath.Join(
		engine.dataDir,
		fmt.Sprintf("%s.wal", engine.frozenGen),
	)

	if engine.frozenWal != nil {
		if err := engine.frozenWal.Close(); err != nil {
			return fmt.Errorf("close frozen WAL: %w", err)
		}
		engine.frozenWal = nil
	}

	if err := os.Remove(walPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove frozen WAL: %w", err)
	}

	engine.frozenMemTable = nil
	engine.frozenGen = ""
	engine.frozenSSTable = nil

	return nil
}

func (job *flushJob) flushGeneration() *flushResult {
	tempFilePath := filepath.Join(job.dataDir, fmt.Sprintf("%s.sst.tmp", job.generation))

	if err := sstable.WriteSSTable(job.memTable, tempFilePath); err != nil {
		return &flushResult{
			generation: job.generation,
			tempPath:   "",
			err:        fmt.Errorf("error flushing in background: %w", err),
		}
	}

	return &flushResult{
		generation: job.generation,
		tempPath:   tempFilePath,
		err:        nil,
	}
}

func (engine *Engine) beginOperation() error {
	engine.lifecycleMu.Lock()
	defer engine.lifecycleMu.Unlock()

	if engine.closing {
		return ErrEngineClosed
	}

	engine.activeOps.Add(1)
	return nil
}

func (engine *Engine) endOperation() {
	engine.activeOps.Done()
}

func (engine *Engine) CreateNewGeneration() error {
	if engine.frozenWal != nil || engine.frozenMemTable != nil {
		return fmt.Errorf("%w: frozen generation still exists", ErrCannotRotateGen)
	}

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
	engine.frozenGen = engine.currGen
	engine.frozenWal = engine.wal
	engine.frozenMemTable = engine.memTable

	// assign new wal and memTable
	engine.wal = newWal
	engine.memTable = memtable.NewMemTable()
	engine.currGen = nextGen

	return nil
}

func (engine *Engine) Execute(command, key, value string) (string, error) {
	if err := engine.beginOperation(); err != nil {
		return "", err
	}
	defer engine.endOperation()

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
