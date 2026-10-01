# RookDB Design Documentation

The design notes are split by subsystem so each document has one job. This page is kept as the stable entry point for existing links.

## Documents

- [Architecture and status](design/overview.md) — system shape, version boundary, and component ownership.
- [WAL](design/wal.md) — record format, durability, recovery, and truncation.
- [Storage engine](design/storage-engine.md) — operations, state mutation, and the WAL/engine boundary.
- [LSM and MemTable](design/lsm.md) — sorted in-memory state, flush policy, recovery model, and tombstones.
- [SSTables](design/sstable.md) — immutable file layout, indexing, integrity, and finalization.
