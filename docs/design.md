# RookDB Design

## Operation Model

RookDB represents database mutations as structured operations rather than persisting client commands directly.

An operation currently contains:

- Type
- Key
- Value

Supported operations:

- SET
- DELETE

For example:

```
SET name Aryan
```

is represented internally as:

```
Operation{
    OpType: Set,
    Key:    "name",
    Value:  "Aryan",
}
```

### Why structured operations?

The WAL stores database history, not client requests.

This keeps the storage layer independent of the protocol used by clients. A future HTTP, TCP, CLI, or binary protocol can produce the same `Operation` representation.

---

## WAL Record Format

A WAL record consists of:

```
[LENGTH][PAYLOAD][CRC32]
```

Where:

* `LENGTH` is a `uint32` representing the payload size.
* `PAYLOAD` is the serialized `Operation`.
* `CRC32` is calculated over `LENGTH + PAYLOAD`.

### Why include LENGTH in the checksum?

The length field participates in record framing and can itself be corrupted. If the checksum covered only the payload, corruption of the length field could go undetected.

Therefore:

```
CRC32(LENGTH + PAYLOAD)
```

---

## WAL Responsibilities

The WAL is responsible for persisting and recovering database operations.

Its primary responsibilities are:

* Serialize operations before persistence.
* Construct WAL records.
* Enforce the maximum physical WAL record size.
* Write records sequentially to disk.
* Synchronize writes to ensure durability.
* Sequentially scan the WAL during recovery.
* Validate record framing and integrity.
* Deserialize valid records back into operations.
* Truncate an invalid or incomplete tail during recovery.

The WAL should hide these implementation details from the storage engine.

The storage engine should only need to work with `Operation` values.

---

WAL API

For the initial version, the WAL exposes three primary operations:

```
Append(Operation)
Next()
TruncateCurrentRecord()
```

Append persists a single database operation.

Next sequentially reads and validates the next WAL record and returns the reconstructed Operation.

TruncateCurrentRecord truncates the record most recently returned by `Next()`. This operation is intended for recovery when the storage engine determines that a structurally valid operation is semantically invalid.

The WAL internally tracks the starting offset of the current record:

```
currentRecordOffset *int64
```

The offset is set when Next() begins processing a record and remains available after a successful Next() so that the storage engine can validate the returned operation.

The offset is cleared when:

* `Next()` reaches io.EOF.
* `Next()` returns an error.
* `TruncateCurrentRecord()` successfully truncates the current record.

Calling TruncateCurrentRecord() when no current record exists is an error.

The storage engine must validate and apply the operation returned by Next() before calling Next() again. This ensures that the WAL's current-record state always corresponds to the operation currently being processed.

Tail truncation remains a WAL responsibility. The storage engine does not directly manipulate WAL offsets or the underlying WAL file.

The storage engine drives recovery by repeatedly calling Next() until the WAL reaches the end of its valid history.

This keeps recovery streaming rather than requiring the WAL to load the entire history into memory.

---

## WAL File Ownership

The database/storage layer is responsible for determining the overall data directory and WAL path.

For example:

```
data/
    wal.log
```

The WAL is responsible for managing the `wal.log` file once its path has been provided.

This keeps the responsibilities separated:

```
Database
    ↓
determines data layout and WAL path
    ↓
WAL
    ↓
manages wal.log
```

The WAL should not need to know how the rest of RookDB organizes its data.

---

## Payload and Record Size Limits

The WAL enforces a maximum physical record size.

This protects WAL recovery from attempting to allocate or read an unreasonable amount of data when a corrupted length field is encountered.

For example, a corrupted record might contain:

```
LENGTH = 0xFFFFFFFF
```

The WAL must validate the declared length against its maximum supported record size before attempting to process the record.

The WAL's physical record-size limit is distinct from semantic database limits such as maximum key or value sizes.

The database/storage layer is responsible for validating whether an operation is semantically valid, while the WAL is responsible for ensuring that the serialized operation can safely fit within a WAL record.

---

## WAL Append Flow

Appending an operation follows this sequence:

```
Operation
    ↓
Serialize
    ↓
Validate payload size
    ↓
Determine LENGTH
    ↓
Calculate CRC32(LENGTH + PAYLOAD)
    ↓
Construct [LENGTH][PAYLOAD][CRC32]
    ↓
Write to WAL
    ↓
Sync
    ↓
Return success
```

The WAL must not report a successful append before the record has been synchronized according to the database's durability contract.

This establishes the following invariant:

> If `Append` reports success, the operation must be recoverable from the WAL after a process crash or normal system restart.

---

WAL Recovery

Recovery scans the WAL sequentially rather than loading the entire file into memory.

The recovery process is conceptually:

Open WAL
    ↓
Read LENGTH
    ↓
Validate LENGTH
    ↓
Read PAYLOAD
    ↓
Read CRC32
    ↓
Verify CRC32(LENGTH + PAYLOAD)
    ↓
Deserialize PAYLOAD
    ↓
Return Operation
    ↓
Storage Engine validates Operation
    ↓
Storage Engine applies Operation
    ↓
Next()

Recovery stops at the first invalid or incomplete record.

The invalid tail is truncated because records after the first invalid record cannot be safely interpreted as part of the database's history.

Valid records before the invalid tail remain part of the recoverable database state.

If `Next()` detects a structural or serialization failure itself, the WAL truncates the invalid record internally.

If `Next()` successfully returns an Operation but the storage engine determines that the operation is semantically invalid, the storage engine requests truncation through TruncateCurrentRecord().

The WAL remains responsible for the actual truncation operation.

---

Recovery Invariants

The WAL follows these invariants during recovery:

* Records are processed sequentially.
* A record must have a valid length.
* The declared length must be within the supported record-size limit.
* The complete payload must be readable.
* The complete checksum must be readable.
* CRC32 must match LENGTH + PAYLOAD.
* The payload must deserialize successfully into an Operation.
* Recovery stops at the first invalid or incomplete record.
* The invalid tail is truncated.
* Valid records before the invalid tail are retained.

Additionally:

* At most one record is considered the current recovery record at a time.
* currentRecordOffset identifies the starting offset of the operation most recently returned by Next().
* TruncateCurrentRecord() is only valid while a current record exists.
* Successfully truncating the current record clears currentRecordOffset.

---

## Storage Engine and WAL Boundary

The WAL is responsible for persistence and recovery.

The storage engine is responsible for maintaining the current database state.

The intended relationship is:

```
Client
   ↓
Protocol / Parser
   ↓
Operation
   ↓
Storage Engine
   ↓
WAL
   ↓
Persistent Storage
```

During recovery, the direction is reversed:

```
WAL
   ↓
Recovered Operations
   ↓
Storage Engine
   ↓
Reconstructed Database State
```

This separation allows the WAL to remain independent of how the database stores its current in-memory state.

An operation can be structurally valid but semantically invalid.

For example, the WAL may contain valid JSON that decodes successfully into an
Operation, but whose operation type is not supported by the current RookDB
version.

Semantic validation belongs to the storage engine rather than the WAL. However,
during recovery, an operation that cannot be accepted by the current database
version is treated as an invalid WAL record. Recovery stops at that record and
the WAL is truncated from its starting offset.

This ensures that RookDB does not skip an operation it cannot interpret and then
continue replaying later operations as though the database history were intact.

## Storage Engine

The storage engine maintains the current materialized state of the database.

For the initial version, the state is represented by an in-memory Go map:

```
map[string]string
```

The WAL remains the durable source of truth, while the map represents the current state reconstructed from that history.

The initial storage engine is conceptually:

```
type Engine struct {
    wal   *wal.WAL
    store map[string]string
}
```

The storage engine is responsible for:

Maintaining the current in-memory database state.
Validating operation semantics.
Applying valid operations to the in-memory state.
Driving WAL recovery during startup.
Appending mutations to the WAL before modifying in-memory state.

The storage engine does not directly manipulate the WAL file or its offsets.

### Storage Engine Write Ordering

A successful mutation follows this sequence:

```
Operation
    ↓
WAL.Append(Operation)
    ↓
WAL synchronizes the record
    ↓
Update in-memory state
    ↓
Return success
```

`WAL.Append()` owns the durability guarantee.

If `WAL.Append()` fails, the storage engine must not modify the in-memory state.

This establishes the invariant:
```
An operation is applied to the current in-memory state only after its WAL record has been successfully persisted according to the WAL durability contract.
```
### Storage Engine Recovery

When the storage engine is initialized, it opens the WAL and reconstructs the current state by replaying operations sequentially.

Conceptually:
```
NewEngine
    ↓
NewWAL
    ↓
Next()
    ↓
Validate operation semantics
    ↓
Apply operation to map
    ↓
Next()
    ↓
...
    ↓
io.EOF
    ↓
Database ready for normal operations
```

The storage engine must validate and apply each operation before requesting the next operation from the WAL.

If semantic validation fails, the current WAL record is considered invalid for the current RookDB version. The storage engine requests TruncateCurrentRecord() and recovery stops.

### Initial Operation Semantics

The initial storage engine supports:

```
SET key value
DELETE key
GET key
```

* `SET` persists the key/value pair.
* `DELETE` removes the key if it exists. Deleting a key that does not exist is a successful no-op.
* `GET` returns the current value for an existing key.

If a requested key does not exist, `GET` returns `ErrKeyNotFound`.

The storage engine does not need to inspect whether a `DELETE` actually changes the current state before writing it to the WAL. A `DELETE` operation is replayable even when the key is already absent.

This keeps the v1 write and recovery paths simple and deterministic.

### Database Data Directory

RookDB separates the location of its executable from the location of its persistent database state.

The executable's working directory is not used to determine where database files are stored because the process may be launched from any directory. Instead, the database data directory is explicitly provided to RookDB.

For v1, the data directory contains the WAL:

```text
data/
└── wal.log
```

The CLI therefore operates conceptually as:

```text
rookdb <data-directory>
```

RookDB derives its internal storage paths from this directory rather than exposing individual storage files such as the WAL as part of the CLI interface.

This also leaves room for the storage layout to evolve without changing the meaning of the CLI argument:

```text
data/
├── wal/
├── sstables/
├── snapshots/
└── ...
```

#### Design rationale

* The executable and persistent state have independent lifecycles.
* The process may be started from any working directory.
* Users specify where database state should live.
* Individual storage files remain an internal implementation detail.
* Future storage components can be added without changing the external database-directory concept.

---

# LSM Storage Engine

RookDB v2 evolves the storage engine from a WAL-backed in-memory map into an LSM-based storage engine.

The v1 storage model is:
```text
WAL → Map
```

The WAL provides durable history while the map represents the current materialized state.

This design is intentionally simple, but the entire current database state must remain in memory and the WAL continues to grow as operations accumulate.

RookDB v2 introduces an LSM storage model:
```text
WAL → MemTable → SSTables
```

The MemTable represents the current mutable state, while SSTables provide immutable persistent storage.

The goal is to allow RookDB to keep the active working set in memory while progressively moving older state to disk.

## LSM Architecture

The high-level v2 architecture is:
```
      RAM
       │
┌──────┴──────┐
│             │
WAL         MemTable
│             │
│           flush
│             ↓
│          SSTable
│             │
└─────────────┴──────→ DISK
```

A mutation follows:
```
SET
 ↓
WAL.Append()
 ↓
WAL durable
 ↓
MemTable update
 ↓
Continue accepting writes
```

When the MemTable reaches its flush threshold:
```
Active MemTable
      ↓
Freeze
      ↓
Flush sorted entries
      ↓
SSTable
      ↓
Persist to disk
```

A new MemTable can then become the active mutable structure while the previous one is being flushed.

WAL reclamation is intentionally not defined by this initial design. It requires additional crash-consistency rules around durable SSTables and will be addressed as the LSM implementation develops.

## Why a MemTable?
The v1 storage engine keeps the entire current state in a Go map:
```
WAL → map[string]string
```

This has several limitations:
- The current state must fit in memory.
- The WAL grows continuously.
- Recovery requires replaying the WAL history.
- The WAL acts as both durable history and the long-term record of changes.

The LSM design separates these responsibilities.
```
WAL
 ↓
Recent durable mutations

MemTable
 ↓
Recent mutable state

SSTables
 ↓
Older persistent state
```

The MemTable therefore acts as the mutable in-memory layer between the WAL and persistent SSTables.

## MemTable

The MemTable stores the current mutable set of database entries before they are flushed into an SSTable.

Its responsibilities are:

- Insert or update key/value pairs.
- Look up keys.
- Delete keys.
- Maintain entries in sorted order.
- Provide sequential iteration for SSTable flushing.
- Track approximate memory usage.
- Determine when the MemTable should be flushed.

The MemTable should expose database-level behavior rather than expose its underlying data structure.

Conceptually:
```
Engine
  ↓
MemTable
  ↓
Skip List
```

The storage engine should not need to know how the Skip List represents levels, nodes, or forward pointers.

### MemTable Interface
The initial MemTable abstraction requires four primary capabilities:
```
Put(key, value)
Get(key)
Delete(key)
Iterator()
```

`Put` inserts a new key/value pair or updates an existing key.

`Get` retrieves the current value associated with a key.

`Delete` removes a key from the current MemTable state.

`Iterator` provides ordered traversal of the MemTable for SSTable generation.

The MemTable should not know about SSTable file formats or disk I/O. Those responsibilities belong to the storage layer responsible for flushing the MemTable.

### Skip List
RookDB will initially implement the MemTable using a Skip List.

The primary reason for this choice is that a Skip List maintains entries in sorted order while providing efficient point operations and sequential traversal.

A hash map would require sorting its keys before every SSTable flush:
```
SET
 ↓
Hash Map
 ↓
Unordered entries
 ↓
Sort keys
 ↓
SSTable
```

Sorting introduces an additional `O(n log n)` step during the flush.

A Skip List maintains sorted order as entries are inserted:
```
SET
 ↓
Skip List
 ↓
Already sorted
 ↓
Sequential iteration
 ↓
SSTable
```

The expected operations are approximately:
```
| Operation       | Expected Complexity |
| --------------- | ------------------- |
| Get             | O(log n)            |
| Put             | O(log n)            |
| Delete          | O(log n)            |
| Create iterator | O(log n)            |
| Next            | O(1)                |
| Full iteration  | O(n)                |
```

The `O(log n)` iterator initialization locates the starting position. Once positioned, traversal follows the bottom-level linked list, making complete iteration `O(log n + n)`, which simplifies to `O(n)`.

**RookDB uses a Skip List probability of `p = 1/2`. This provides an expected logarithmic search structure while keeping the implementation simple. The probability represents a memory-versus-search tradeoff: lower probabilities reduce forward-pointer overhead but produce a sparser upper-level structure. The Skip List has a hard maximum height of 20 levels; individual nodes receive probabilistic heights and typically contain far fewer than 20 forward pointers.**

#### Why Skip List instead of a balanced tree?
Balanced trees such as AVL or Red-Black trees can also maintain sorted data with `O(log n)` operations.

A Skip List was selected for the initial implementation because:

- The implementation is comparatively simple.
- Sorted traversal is naturally represented through the bottom-level linked list.
- Insertions do not require tree rotations or rebalancing.
- The structure has potential for future concurrent access patterns.

The current RookDB engine is not concurrent, so concurrency is not a requirement for v2. The primary motivation remains the combination of sorted storage, efficient point operations, sequential iteration, and implementation simplicity.

### MemTable Iterator
The MemTable exposes an iterator rather than returning all entries at once.

A conceptual iterator is:
```
iterator := memtable.Iterator()

iterator.Next()
iterator.Key()
iterator.Value()

iterator.Next()
iterator.Key()
iterator.Value()
```

The iterator allows the SSTable writer to consume entries incrementally.

Returning all entries as a slice would require an additional `O(n)` memory allocation:
```
MemTable
   +
[]Entry
```
This is undesirable because the MemTable is already the primary in-memory working structure.

The iterator therefore keeps traversal state and exposes entries one at a time.

The iterator itself does not need to know anything about SSTable formats or disk writes.

The separation is:

```
MemTable
    ↓
Iterator
    ↓
SSTable Writer
    ↓
Disk
```

The SSTable writer may internally batch entries into larger buffers for efficient disk I/O, but batching is not the responsibility of the MemTable iterator.

### MemTable Memory Budget
The initial v2 implementation uses an approximate memory model rather than attempting to precisely reproduce Go runtime heap usage.

The initial design parameters are:

- **Flush threshold**: 80 MiB
- **Average key/value payload assumption**: 128 bytes
- **Skip List maximum height**: 20 levels

The 80 MiB value is a flush threshold rather than a claim that the MemTable physically consumes exactly 80 MiB.

The approximate accounting is intended to provide a practical trigger for flushing while leaving room for estimation error caused by factors such as allocator overhead and Skip List node structure.

Conceptually:
```
Put()
  ↓
Update Skip List
  ↓
Update approximate memory usage
  ↓
Memory ≥ 80 MiB?
  ├── No → Continue accepting writes
  │
  └── Yes → Freeze MemTable
               ↓
            Flush
               ↓
            SSTable
```

#### Key/Value Assumption

For initial capacity planning, RookDB assumes an average logical key/value payload of approximately `128 bytes` per entry.

This is a planning assumption rather than a restriction on key or value size.

Actual memory consumption is larger because each Skip List node also contains structural metadata and forward pointers.

The initial implementation will therefore use approximate accounting instead of attempting to model every Go allocator detail.

#### Skip List Maximum Height
The Skip List has a hard maximum height of `20 levels`.

This is a maximum rather than the height of every node. Individual nodes receive probabilistic heights, with most nodes expected to contain relatively few forward pointers.

The hard ceiling provides a predictable upper bound while keeping the implementation simple.

#### MemTable Memory Accounting
Memory accounting belongs to the MemTable rather than the Skip List.

The Skip List is a general ordered data structure and should not need to know the memory policy chosen by RookDB.

The separation is:
```
MemTable
├── Skip List
└── Approximate memory usage
```

The MemTable therefore owns the flush policy.

The initial implementation deliberately favors a simple approximate calculation over exact heap accounting.

The calculation can later be calibrated against actual runtime measurements.

## v2 Recovery Model
With the introduction of SSTables, the recovery model becomes:

```
SSTables
   ↓
Persistent database state
   │
   +
WAL
   ↓
Recent durable mutations
   ↓
Replay
   ↓
MemTable
   ↓
Database ready
```

The SSTables establish the persisted state that existed before the remaining WAL operations.

The WAL is then replayed to reconstruct the current mutable MemTable.

The exact rules for WAL reclamation after a successful MemTable flush are intentionally deferred until SSTable durability and crash-consistency behavior are implemented.