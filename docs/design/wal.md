# Write-Ahead Log (WAL)

## Responsibilities

The WAL serializes operations, frames records, enforces the physical record limit, writes records sequentially, synchronizes appends, scans records during recovery, checks integrity, deserializes operations, and truncates an invalid tail. The storage engine does not manipulate the WAL file or its offsets directly.

The WAL receives a path from the storage layer, which owns the overall data directory. The WAL manages that file once opened.

## Record format

```text
[LENGTH:uint32, big-endian][PAYLOAD][CRC32:uint32, big-endian]
```

`PAYLOAD` is the JSON encoding of an `Operation`. The CRC32 covers the encoded length followed by the payload:

```text
CRC32(LENGTH + PAYLOAD)
```

Including the length detects corruption in the field that controls record framing. The current implementation limits the complete physical record to 512 bytes.

## Append and durability

```text
Operation → JSON → validate physical size → frame and checksum → write → sync → success
```

Append must not report success before synchronization completes. The engine appends before applying an operation to its in-memory state.

## Streaming recovery

Recovery reads one record at a time:

1. Read and validate the length before allocating the payload.
2. Read the complete payload and checksum.
3. Verify the checksum and decode the operation.
4. Return the operation to the engine for semantic validation and application.
5. Repeat only after the engine has handled the current operation.

Recovery stops at the first invalid or incomplete record. Valid preceding records remain usable; the invalid tail and anything after it are discarded because later framing cannot be trusted.

The WAL truncates structural or decoding failures itself. If it returns a decoded operation that the engine rejects semantically, the engine calls `TruncateCurrentRecord()` and stops recovery. This preserves the boundary between WAL-format validity and database-operation validity.

## API and current-record state

The recovery-facing API is:

```text
Append(Operation)
Next()
TruncateCurrentRecord()
```

After a successful `Next()`, the WAL retains the starting offset of that record until the next read or successful truncation. `TruncateCurrentRecord()` is only valid while that offset exists. EOF, a read failure, or successful truncation clears it. The caller must validate and apply the returned operation before calling `Next()` again.

## Recovery invariants

- Records are processed sequentially and recovery stops at the first invalid record.
- Declared size is bounded before payload allocation.
- The payload, checksum, CRC, and operation decoding must all be valid.
- The invalid tail is truncated; preceding valid records are retained.
- At most one returned record is current for semantic validation and truncation.

## WAL generations
Each active write generation has its own WAL. The WAL records the mutations that populate that generation's MemTable.

A generation transitions through:
```text
WAL + current MemTable
        ↓
WAL + frozen MemTable
        ↓
published SSTable
        ↓
WAL reclaimed
````

The WAL remains the durable recovery source until the corresponding SSTable has been successfully published.

On startup, WALs belonging to generations without a published SSTable are replayed in generation order. Recovery reconstructs the corresponding MemTable state and can repeat the normal flush process.