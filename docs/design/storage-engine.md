# Storage Engine

## Operation model

Mutations are represented as structured operations rather than persisted client commands. An operation contains a type, key, and optional value. The supported mutation types are `SET` and `DELETE`; `GET` is a read and is not written to the WAL.

For example, `SET name Aryan` becomes:

```go
Operation{OpType: Set, Key: "name", Value: "Aryan"}
```

This keeps persistence independent of the CLI and client protocol.

## State and mutation ordering

The storage engine validates operations, replays durable state at startup, appends mutations before changing in-memory state, and answers reads from the MemTable and SSTables.

The write ordering is:

```text
validate → WAL append and sync → update map → return success
```

If append fails, in-memory state is not updated. A successful append means the WAL has synchronized the record according to its durability contract.

`SET` requires a non-empty key and value. `DELETE` requires a non-empty key and no value; deleting an absent key succeeds as a no-op. `GET` returns `ErrKeyNotFound` when the key is absent.

## Recovery boundary

The engine requests operations from the WAL one at a time, validates and applies each before requesting the next. If a structurally valid operation is unsupported or semantically invalid, recovery stops and truncates that current record through the WAL API. Recovery does not skip an operation and continue with later history.

The detailed record and corruption rules are in [WAL](wal.md).

## State lookup

Reads check the MemTable first, then search SSTables from newest to oldest. A live entry returns its value; a tombstone returns not-found; an absent key in one file allows lookup to continue. See [LSM and MemTable](lsm.md) and [SSTables](sstable.md).
