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

## SSTable Design

### Overview

RookDB v2 introduces SSTables (Sorted String Tables) as the persistent
on-disk representation of flushed MemTables.

The v1 storage model is:

```
WAL → Map
```
The v2 storage model becomes:
```
WAL → MemTable → SSTable
```

An SSTable is immutable once finalized. It stores sorted key/value records
produced by flushing a MemTable.
The primary goals of the initial SSTable design are:
- sequential disk writes
- immutable on-disk files
- sorted records
- efficient point lookups
- corruption detection
- bounded lookup work through sparse indexing
- simple crash/finalization semantics

This design is intentionally a v2 starting point. Parameters such as block
size and memory assumptions can be refined empirically in v2.1.x.

### 1. MemTable → SSTable Flush
When a MemTable reaches its configured memory threshold, it is frozen and
flushed to an SSTable.
The active write path becomes conceptually:
```
SET
 ↓
WAL durable
 ↓
MemTable update
 ↓
MemTable reaches threshold
 ↓
freeze MemTable
 ↓
flush MemTable → SSTable
 ↓
create new MemTable
```

The MemTable is sorted through its Skip List, allowing the SSTable to be
written sequentially without sorting the entire dataset again.

The existing MemTable iterator provides the records in key order.

### 2. SSTable Immutability
An SSTable is immutable after it has been successfully finalized.

During construction, records and metadata are written sequentially.

Once the SSTable is finalized:
- its data blocks are not modified
- its sparse index is not modified
- its footer is not modified

Future updates to keys are represented by newer MemTables/SSTables rather
than modifying an existing SSTable.

### 3. Record Framing
SSTable records use length-prefixed encoding.

The record format is:
```
[KEY_LEN][VALUE_LEN][KEY_BYTES][VALUE_BYTES]
```
The exact integer widths/endian encoding are still to be finalized.

Length-prefixed records were chosen instead of delimiter-based framing because
keys and values may contain arbitrary content, including characters such as:
```
hello|world
hello\nworld
hello\0world
```

Therefore the record format must not depend on a special delimiter appearing
only outside the payload.

The lengths provide enough information for the reader to determine the exact
boundaries of the key and value.

### 4. Data Blocks
An SSTable is divided into data blocks.

A block contains multiple consecutive sorted records.

The initial target is:
```
16 records per block
```

This is a tunable starting parameter rather than a permanent storage-format
constraint.

The approximate key/value payload assumption used elsewhere in the v2 design
is around 128 bytes per record.

Therefore:
```
16 × ~128 bytes ≈ 2 KiB
```

This makes it practical to buffer one complete block in memory before writing
it to disk.

The final block may contain fewer than 16 records.

### 5. Block Buffering
Blocks are constructed in memory before being written.

The flush process is:
```
MemTable.Iterator()
      ↓
accumulate records
      ↓
16 records reached
      ↓
finalize block
      ↓
calculate BLOCK_LEN
      ↓
calculate CRC
      ↓
write block sequentially
      ↓
start next block
```

If the iterator reaches the end before 16 records are accumulated, the
remaining records form the final block.

Buffering a block avoids the need to know the final block length before the
block has been constructed.

It also avoids seeking backward to patch the block length after writing.

Because the expected block size is only a few KiB, the additional memory
overhead is considered acceptable.

### 6. Block Framing
The current block format is:
```
[BLOCK_LEN][BLOCK_DATA][CRC32]
```

Where:
- BLOCK_LEN is the length of BLOCK_DATA only
- BLOCK_DATA contains the serialized records
- CRC32 is a fixed 4-byte checksum

The CRC covers:
```
CRC32(BLOCK_DATA)
```

The block length does not include the CRC.

This allows the reader to parse a block as:
```
read BLOCK_LEN
    ↓
read exactly BLOCK_LEN bytes
    ↓
read fixed 4-byte CRC
    ↓
verify CRC(BLOCK_DATA)
    ↓
parse records
```

The exact integer width/endian encoding of BLOCK_LEN remains to be finalized.

### 7. Sorted Data
Records within an SSTable are sorted by key.

Because the MemTable already maintains sorted order through its Skip List,
flushing does not require an additional sorting pass.

This allows the SSTable writer to perform a sequential traversal:
```
MemTable
   ↓
Iterator
   ↓
Block 1
   ↓
Block 2
   ↓
Block 3
   ↓
...
```

Sorted data is necessary for efficient lookup using the sparse index.

### 8. Sparse Index
An SSTable maintains a sparse index containing one entry per data block.

Each entry conceptually contains:
```
[first_key_of_block → block_offset]
```

For example:
```
"alice" → 0
"bob"   → 2184
"charlie" → 4312
"dan"   → 6497
```

The block offset points to the beginning of the block framing, meaning the
location where BLOCK_LEN is stored.

The index allows lookup to avoid scanning the entire SSTable.

Conceptually:
```
GET(key)
   ↓
binary search sparse index
   ↓
identify candidate block
   ↓
seek to block offset
   ↓
read block
   ↓
scan records within block
```

Since each block contains at most 16 records, the final scan is bounded by
the configured block record count.

### 9. Building the Sparse Index
The sparse index is built while the SSTable is being written.

Before writing each block, the writer already knows:
1. the first key of the block
2. the current file offset

Therefore it can record:
```
firstKeyOfBlock → currentFileOffset
```

and then write the block.

Conceptually:
```
current file offset ──────────┐
                              ↓
first key of block ───────→ sparse index
                              ↓
                    write [LEN][DATA][CRC]
```

The complete sparse index is kept in memory during the flush and written to
disk after all data blocks have been written.

This preserves sequential data-block writing.

### 10. Why Not Reconstruct the Index by Seeking?
An alternative would be to:
1. write all data blocks
2. seek back to the beginning of the SSTable
3. read each block's length
4. skip the block
5. inspect the first record
6. extract the first key
7. reconstruct the sparse index

This would require a second traversal of the SSTable and would require the
index-building logic to parse enough of each block to discover its first key.

The chosen design instead records the information during the original flush.

Therefore the initial design uses:
```
single sequential data-writing pass
+
in-memory sparse index construction
+
one sequential index write
```

The additional memory required by the sparse index is considered acceptable.

### 11. MemTable Memory Budget and Flush Overhead
The initial MemTable flush threshold is:
```
80 MiB
```

The threshold is intentionally not treated as an exact measurement of process
memory.

During a flush, additional temporary memory may be required for:
- the block buffer
- sparse index entries
- serialization buffers
- other SSTable construction metadata

The design therefore accepts some memory overhead beyond the MemTable's
logical size.

The current assumption is that this additional overhead is small enough to
fit comfortably within the intended memory budget.

The actual parameters will be refined using real workload measurements in
v2.1.x.

### 12. SSTable Footer
The SSTable uses a footer at the end of the file.

The footer acts as the finalization/commit marker for the SSTable.

The reader can locate the footer from the end of the file because the footer
has a fixed size.

Conceptually:
```
file
┌───────────────────────┐
│ Data Block 1          │
│ Data Block 2          │
│ ...                   │
│ Data Block N          │
├───────────────────────┤
│ Sparse Index          │
├───────────────────────┤
│ Footer                │
└───────────────────────┘
                         ↑
                         EOF
```

The footer is expected to contain metadata required to locate structures such
as the sparse index.

The exact footer fields are not yet finalized.

### 13. Footer Validation
The footer will contain a magic value and version information so that RookDB
can recognize the expected SSTable format before interpreting its metadata.

The footer will also have integrity protection.

Conceptually:
```
Footer
├── Magic
├── Version
├── Index Offset
├── Index Size
├── ...
└── CRC
```

The exact fields and checksum coverage remain to be finalized.

The important design principle is:
* An SSTable is considered valid only after its final footer has been
successfully written and validated.

A partially constructed SSTable must not be treated as a finalized SSTable
during recovery.

### 14. Crash Consistency
A crash can occur while an SSTable is being constructed.

For example:
```
write block 1
write block 2
write block 3
      ↓
    CRASH
```

The file may therefore contain partial data.

The footer provides a finalization boundary.

Conceptually:
```
data blocks
   ↓
sparse index
   ↓
footer
   ↓
SSTable becomes finalized
```

The existence of data bytes alone does not make an SSTable valid.

The exact recovery/discovery procedure for partially written SSTables is
still to be designed.

### 15. Integrity Domains
The current design uses checksums at multiple levels.

Data blocks have their own CRC:
```
[BLOCK_LEN][BLOCK_DATA][CRC]
```

The sparse index is also intended to have integrity protection.

The footer will have integrity protection as well.

The purpose is to keep corruption detection local where possible:
```
corrupted block
      ↓
detect while reading that block
```

rather than requiring the entire SSTable to be treated as one large integrity
domain.

The exact checksum coverage for the sparse index and footer remains to be
finalized.

### 16. Current SSTable Architecture
The current design can be summarized as:
```
                                  MemTable
                                     │
                                  Iterator
                                     │
                                     ▼
                            ┌─────────────────┐
                            │  Block Buffer   │
                            │   ≤ 16 records  │
                            └────────┬────────┘
                                     │
                            record first key +
                            current file offset
                                     │
                                     ▼
                            ┌─────────────────┐
                            │   Data Block    │
                            │ [LEN][DATA][CRC]│
                            └────────┬────────┘
                                     │
                                     ▼
                                  next block
                                     │
                                    ...
                                     │
                                     ▼
                            ┌─────────────────┐
                            │  Sparse Index   │
                            └────────┬────────┘
                                     │
                                     ▼
                            ┌─────────────────┐
                            │     Footer      │
                            └─────────────────┘
```

Lookup:
```
GET(key)
   │
   ▼
Sparse Index
   │
   │ binary search
   ▼
Candidate Block
   │
   │ seek to block offset
   ▼
[BLOCK_LEN][BLOCK_DATA][CRC]
   │
   │ verify CRC
   ▼
scan ≤ 16 records
   │
   ▼
result
```

### 17. Decisions Still Open
The following have intentionally not been locked yet:
- exact integer widths for key/value lengths
- endian encoding for SSTable fields
- exact sparse-index entry representation
- exact sparse-index on-disk format
- sparse-index checksum coverage
- exact footer fields
- footer checksum coverage
- SSTable file naming/discovery
- recovery behavior for incomplete SSTables
- tombstone representation
- multiple-SSTable lookup ordering
- Bloom filters
- compaction
- SSTable versioning details
- 
These should be designed incrementally rather than introducing them all at
once.

### 18. Initial Design Principles
The current SSTable design follows these principles:
1. Sequential writes over random writes
2. Buffer small blocks rather than backpatch metadata
3. Exploit the MemTable's existing sorted order
4. Keep sparse indexing lightweight
5. Use length-prefixed framing for arbitrary key/value data
6. Use local checksums for corruption detection
7. Use a footer as the SSTable finalization boundary
8. Spend a small amount of memory to simplify the write path
9. Keep initial parameters tunable
10. Measure and refine parameters empirically in v2.1.x

## SSTable File Layout and Finalization

The SSTable layout is extended with an explicit sparse-index integrity
boundary and a fixed-size footer.

The final layout is:

```text
┌──────────────────────────────┐
│ Data Blocks                  │
│                              │
│ [BLOCK_LEN][BLOCK_DATA][CRC] │
│ [BLOCK_LEN][BLOCK_DATA][CRC] │
│ ...                          │
├──────────────────────────────┤
│ Sparse Index                 │
│                              │
│ [KEY_LEN][KEY][BLOCK_OFFSET] │
│ [KEY_LEN][KEY][BLOCK_OFFSET] │
│ ...                          │
├──────────────────────────────┤
│ Sparse Index CRC32           │
├──────────────────────────────┤
│ Footer                       │
│   MAGIC                      │
│   INDEX_OFFSET               │
│   INDEX_SIZE                 │
├──────────────────────────────┤
│ Footer CRC32                 │
└──────────────────────────────┘
```

The footer is fixed-width, so a separate footer-size field is unnecessary.

The reader can locate the footer by seeking backwards from EOF by the known
footer size plus its fixed-size CRC.

### Sparse Index Integrity
The sparse index has its own CRC32.

The checksum covers all sparse-index entry bytes:
```
SparseIndexCRC = CRC32(all sparse-index entries)
```

The checksum is stored immediately after the sparse index and before the
footer.

INDEX_SIZE in the footer refers only to the sparse-index entries. It does
not include the sparse-index CRC.

Therefore:
```
INDEX_OFFSET
      ↓
[ sparse index entries ... ]
      ↑
      └── INDEX_SIZE bytes

[ sparse index CRC ]
[ footer ]
[ footer CRC ]
```

This keeps the index data and its integrity metadata as separate, clearly
defined regions.

### Footer
The footer contains the metadata required to locate the sparse index:
```
Footer
├── MAGIC
├── INDEX_OFFSET
└── INDEX_SIZE
```

The footer itself is protected by a separate CRC32 stored immediately after
the footer.

The footer checksum covers the footer fields:
```
FooterCRC = CRC32(MAGIC + INDEX_OFFSET + INDEX_SIZE)
```

The footer does not contain its own size because its layout and field widths
are fixed by the SSTable format.

### SSTable Finalization
An SSTable is considered finalized only after the complete sequence of storage
structures has been successfully written:
```
Data Blocks
    ↓
Sparse Index
    ↓
Sparse Index CRC
    ↓
Footer
    ↓
Footer CRC
```

The footer is therefore the finalization boundary for the SSTable.

The presence of earlier data in the file does not by itself make the SSTable
valid.

A process crash during any earlier stage can leave a partially written
SSTable:
```
Blocks
   ↓
partial block / index / footer
   ↓
CRASH
```

Such a file is not considered a finalized SSTable because it does not have a
complete valid footer.

### Block Finalization
Each data block is constructed completely in memory before being written.

The block finalization sequence is:
```
accumulate records
      ↓
construct BLOCK_DATA
      ↓
calculate CRC32(BLOCK_DATA)
      ↓
write [BLOCK_LEN][BLOCK_DATA][CRC32]
      ↓
block finalized
```

The writer does not move to the next block until the current block has been
constructed and its checksum calculated.

This keeps each block as an independent integrity domain.

### Crash During SSTable Construction
If the process crashes while an SSTable is being constructed, the incomplete
SSTable is not used as persistent state.

Examples:
#### Crash during a data block
```
Block 1 ✓
Block 2 ✓
Block 3 → CRASH
```

The SSTable has no finalized footer and is therefore not accepted as a
completed SSTable.

#### Crash during sparse-index writing
```
All data blocks ✓
Sparse index → CRASH
```

Again, no finalized footer exists, so the SSTable is not accepted.

#### Crash during footer writing
```
All data blocks ✓
Sparse index ✓
Footer → CRASH
```

The SSTable is still incomplete because its finalization marker was not
successfully completed.

### WAL as Recovery Source
The WAL remains the durable source of mutation history.

A partially constructed SSTable does not need to be repaired or completed
after a crash.

Recovery instead uses:
```
valid finalized SSTables
        +
WAL replay
        ↓
reconstructed MemTable
```

The incomplete SSTable can subsequently be discarded during SSTable
discovery/cleanup.

This preserves the fundamental storage invariant:
```
WAL → durable mutation history
SSTable → finalized materialized state
```

### Background SSTable Flush
SSTable flushing is intended to occur asynchronously after a MemTable reaches
its flush threshold.

The active MemTable is frozen and handed to the SSTable writer:
```
                    WAL
                     │
                     ▼
               Active MemTable
                     │
              reaches threshold
                     │
                     ▼
               freeze MemTable
                     │
                     ├──────────────► background flush
                     │
                     ▼
               New MemTable
                     │
                     ▼
                new writes
```

The frozen MemTable remains immutable while its contents are written to the
SSTable.

The new MemTable can accept subsequent writes while the previous MemTable is
being flushed.

The WAL continues recording these new mutations.

If a crash occurs during the background flush, recovery can reconstruct the
necessary state from the WAL rather than depending on the incomplete SSTable.

### SSTable Validity Invariant
The resulting invariant is:

**An SSTable becomes part of persistent storage only after all of its data
blocks and metadata have been successfully written and a valid final footer
has been written.**

More concretely:
```
No complete valid footer
        ↓
SSTable is incomplete
        ↓
do not use as finalized persistent state

Complete valid footer
        ↓
SSTable is finalized
        ↓
may participate in recovery
```

The footer therefore serves as the durable boundary between an SSTable that is
still being constructed and one that RookDB may treat as persistent state.

## New Locked format for SSTable
```
SSTable
│
├── Data Blocks
│   └── [BLOCK_LEN:uint32][BLOCK_DATA][CRC32:uint32]
│
├── Sparse Index
│   └── [KEY_LEN:uint16][KEY][BLOCK_OFFSET:uint64]
│
├── Sparse Index CRC32
│
├── Footer
│   ├── MAGIC
│   ├── INDEX_OFFSET:uint64
│   └── INDEX_SIZE:uint64
│
└── Footer CRC32
```

Record format is also updated:
```
[TYPE:uint8]
[KEY_LEN:uint16]
[VALUE_LEN:uint16]
[KEY_BYTES]
[VALUE_BYTES]
```

with types:
```
SET    = 0x01
DELETE = 0x02
```

### DELETE SEMANTICS
When we want to delete a value which is not the part of MemTable
We will Set it in MemTable

A DELETE has VALUE_LEN = 0.

The important semantic rule is:
```
newest SSTable → oldest SSTable

SET       → definitive result
DELETE    → definitive "not found"
absent    → continue searching older SSTable
```

## MemTable Entry Model & Tombstones
Why the original model was insufficient

The initial MemTable was designed around a simple key-value model:
```
key → value
```

This worked while the MemTable only represented live values.

However, an LSM tree cannot physically remove a key when a DELETE occurs.

Consider:
```
Older SSTable:
    user:42 → "Aryan"

Current MemTable:
    user:42 → [removed]
```

If the MemTable simply removed the key, flushing it would produce no record for `user:42`. During a lookup, the older SSTable could then return `"Aryan"` again.

This is known as resurrection of deleted data.

Therefore, a DELETE must itself become persistent state.

### Entry abstraction
The fundamental logical record stored by the MemTable is now an Entry:
```Go
type Entry struct {
    Type  EntryType
    Key   string
    Value string
}
```

The entry type distinguishes between a live value and a tombstone:
```Go
type EntryType uint8

const (
    SetEntry    EntryType = 0x01
    DeleteEntry EntryType = 0x02
)
```

The semantics are:
```
SET:
    Type  = SET
    Key   = "user:42"
    Value = "Aryan"

DELETE:
    Type  = DELETE
    Key   = "user:42"
    Value = ""
```

A DELETE entry is therefore a tombstone.

The value is empty for a DELETE because the operation only needs to record that the key was explicitly deleted.

### MemTable invariant
The MemTable maintains:
* At most one Entry exists for each key within a MemTable generation, and that Entry represents the latest operation for that key in that generation.

For example:
```
SET A 1
SET A 2
DELETE A
```

results in:
```
A → DELETE
```

rather than three separate entries.

The WAL still contains the mutation history, but the MemTable represents the latest materialized state.

This gives the WAL and MemTable distinct responsibilities:
```
WAL
 ↓
mutation history

MemTable
 ↓
latest state of each key in the current generation
```

### DELETE does not remove the Skip List node
A DELETE operation must **not physically remove the key's node from the Skip List.**
Instead:
```
Before:

A → SET("hello")

DELETE A

After:

A → DELETE
```

The node remains because the tombstone must eventually be flushed into an SSTable.

This also means that the previous logical deleteNode() behavior is no longer appropriate for normal DELETE operations.

The MemTable should instead have one logical entry-update path capable of handling both SET and DELETE.

Conceptually:
```
apply Entry
    │
    ├── SET
    │
    └── DELETE
```

### Entry state transitions
Every operation updates the existing Entry when the key is already present.


| Existing state | New operation | Result |
|---|---|---|
| Absent | SET | Create `SET` Entry |
| SET | SET | Update existing Entry's value |
| Absent | DELETE | Create `DELETE` tombstone |
| SET | DELETE | Change Entry type to `DELETE`, clear value |
| DELETE | SET | Change Entry type to `SET`, assign new value |
| DELETE | DELETE | No change |

For example:
```
SET A 100
    ↓
A → SET("100")

DELETE A
    ↓
A → DELETE

SET A 200
    ↓
A → SET("200")
```

Two consecutive DELETE operations require no additional state change:
```
DELETE A
DELETE A
    ↓
A → DELETE
```

### MemTable lookup semantics
The previous lookup model:
```
Get(key) → (value, bool)
```

cannot distinguish between:
```
key is absent
```

and:
```
key exists as a tombstone
```

The lookup therefore becomes:
```Go
Get(key string) (value string, typ EntryType, ok bool)
````

The meaning of ok is:
* ok == true means an Entry exists for the key.

It does not mean that the key has a live value.

Therefore:
```
Absent:
    "",       0,      false

SET:
    "Aryan",  SET,    true

DELETE:
    "",       DELETE, true
```

This gives the storage engine the three states it needs:
```
             Entry?
              │
       ┌──────┴──────┐
      no             yes
      │               │
    ABSENT       ┌────┴────┐
                 │         │
                SET      DELETE
                 │         │
              value     tombstone
```

### Memory accounting
The existing approximate MemTable memory accounting can be extended naturally because the Skip List node remains allocated for both SET and DELETE.

For a normal SET entry:
```
node overhead
+ key
+ value
+ next pointers
```

For a DELETE entry:
```
node overhead
+ key
+ next pointers
```

The value payload is the only part removed when transitioning:
```
SET → DELETE
```

Therefore:
```
SET → DELETE
    sizeDelta = -len(oldValue)
```

The node overhead, key, and next-pointer storage remain unchanged.

Conversely:
```
DELETE → SET
    sizeDelta = +len(newValue)
```

And:
```
DELETE → DELETE
    sizeDelta = 0
```
The complete accounting model is:
| Transition | Approximate size delta |
|---|---:|
| Absent → SET | Node + key + value + pointers |
| Absent → DELETE | Node + key + pointers |
| SET → SET | `len(newValue) - len(oldValue)` |
| SET → DELETE | `-len(oldValue)` |
| DELETE → SET | `+len(newValue)` |
| DELETE → DELETE | `0` |

This preserves the existing philosophy of approxSize: it is an approximate logical memory footprint used as a flush heuristic, not an exact measurement of Go's runtime memory usage.

### Direct translation to SSTables
The Entry abstraction also aligns the MemTable naturally with the SSTable record format.

The pipeline becomes:
```
Command
   ↓
Operation
   ↓
Entry
   ↓
MemTable
   ↓
SSTable Record
```

A SET entry:
```
Entry{
    Type:  SET,
    Key:   "user:42",
    Value: "Aryan",
}
```

becomes an SSTable SET record.

A DELETE entry:
```
Entry{
    Type:  DELETE,
    Key:   "user:42",
    Value: "",
}
```

becomes an SSTable DELETE record:
```
[TYPE=DELETE]
[KEY_LEN]
[VALUE_LEN=0]
[KEY_BYTES]
```

This means the SSTable writer does not need to invent a separate representation for deletion. It serializes the same logical Entry that the MemTable already stores.

### Tombstones during SSTable lookup
SSTables are searched from newest to oldest.

Suppose:
```
SSTable-002 (newer)
    user:42 → DELETE

SSTable-001 (older)
    user:42 → "Aryan"
```

Lookup proceeds:
```
SSTable-002
    ↓
user:42 → DELETE
    ↓
STOP
    ↓
return "not found"
```

The older SSTable must not be searched after finding the tombstone.

If the key is absent from the newer SSTable entirely, lookup can continue to older SSTables.

Therefore:
```
SET       → definitive value, stop
DELETE    → definitive not-found, stop
ABSENT    → continue searching older SSTables
```

This is what prevents deleted keys from being resurrected.

### Resulting design invariant
The key invariant introduced by this change is:
* A DELETE is represented as a tombstone Entry in the MemTable and eventually as a DELETE record in an SSTable. Tombstones are never represented by physically removing the key from the MemTable.

This preserves deletion semantics across MemTable flushes and multiple SSTables while allowing MemTable Entries to translate directly into SSTable records.