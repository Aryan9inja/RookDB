# Architecture

## Storage architecture

RookDB stores operations in a WAL, applies them to a sorted in-memory MemTable, and flushes immutable state to SSTables:

```text
Operation → WAL → MemTable → SSTables
```

## Component boundaries

```text
Client / parser
      ↓ Operation
Storage engine ───── validates meaning and applies state
      ↓
WAL ──────────────── frames, persists, and recovers operations
      ↓
Persistent files
```

The operation is the boundary between client syntax and storage. The WAL stores database operations rather than client commands, so different protocols can produce the same operation type.

The storage engine owns semantic validation and coordinates the materialized state. The WAL owns serialization, record framing, integrity checks, and file offsets. A decoded but unsupported operation is a semantic failure in the engine; recovery stops at that record and asks the WAL to truncate it.

## Data directory

The caller supplies a database data directory independently of the executable's working directory. Storage paths are derived internally, keeping individual files and directories out of the CLI interface.

## Related documents

- [WAL](wal.md)
- [Storage engine](storage-engine.md)
- [LSM and MemTable](lsm.md)
- [SSTables](sstable.md)
