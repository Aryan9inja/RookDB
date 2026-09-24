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