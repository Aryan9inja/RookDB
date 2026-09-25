package engine

import (
	"errors"

	"github.com/Aryan9inja/RookDB/internal/operation"
)

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
