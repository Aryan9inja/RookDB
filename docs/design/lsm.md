# LSM Storage Engine and MemTable

## Architecture

```text
WAL → MemTable → SSTables
```

The WAL records durable mutations, the MemTable holds the latest state for the active generation, and SSTables hold finalized immutable state.

## MemTable responsibilities

The MemTable inserts and updates entries, looks them up, retains deletion state, maintains key order, exposes an iterator for flushing, estimates memory usage, and signals when it should be frozen. It hides its underlying data structure and knows nothing about SSTable encoding or disk I/O.

The MemTable uses a Skip List. It maintains sorted order during writes, supports expected `O(log n)` point operations, and provides `O(n)` full ordered iteration. A probability of `p = 1/2` and a maximum height of 20 are the parameters. The structure is chosen for implementation simplicity, efficient ordered traversal, and no tree rebalancing.

An iterator yields entries incrementally so flushing does not need to allocate a second full copy of the MemTable. The SSTable writer may buffer a block, but that buffering is outside the iterator.

## Flush policy

The approximate flush threshold is 80 MiB. It is a heuristic, not a cap on actual process memory. Capacity planning assumes roughly 128 bytes of logical key/value payload per entry; Skip List nodes and runtime allocation add overhead. The MemTable owns approximate accounting and the flush decision.

The transition is:

```text
active MemTable reaches threshold → freeze → flush to SSTable → install new active MemTable
```

Background flush allows writes to continue in a new MemTable. The WAL remains available during construction so a crash does not depend on a partial SSTable.

## Recovery model

Recovery loads valid finalized SSTables as the base state, then replays the WAL into a new MemTable.

## Tombstones

An LSM cannot represent deletion by removing the key from memory: an older SSTable could then return the deleted value. A DELETE must remain as a tombstone until it is safe to remove it during compaction.

The MemTable entry is:

```go
type Entry struct {
    Type  EntryType // SET or DELETE
    Key   string
    Value string
}
```

There is at most one entry per key in a MemTable generation, representing the latest operation. `SET A 1; SET A 2; DELETE A` leaves `A → DELETE`. A tombstone keeps the Skip List node and key; its value is empty. Repeated deletes are idempotent.

Lookup must distinguish absent, live, and deleted states. Conceptually, `Get(key)` returns an entry plus whether an entry exists; `exists` is true for a tombstone. On SSTable lookup, search newest to oldest: SET returns a value, DELETE returns not-found immediately, and absence continues to older files.

Approximate size accounting changes by the value-size delta on SET updates.

* SET→DELETE subtracts the old value length;
* DELETE→SET adds the new value length;
* DELETE→DELETE changes nothing.

Node and key storage remain while the tombstone is present.
