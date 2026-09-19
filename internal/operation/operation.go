package operation

type OperationType string

// Currently, two operations are supported
// SET - needs both key and value
// DELETE - needs only key
const (
	Set    OperationType = "SET"
	Delete OperationType = "DELETE"
)

// Operation represents a database operation persisted in the WAL.
type Operation struct {
	OpType OperationType `json:"type"`
	Key    string        `json:"key"`
	Value  string        `json:"value,omitempty"`
}
