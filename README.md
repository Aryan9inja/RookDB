# RookDB

A persistent/distributed key-value database written from scratch in Go.

RookDB is a learning-focused database project built to understand how storage engines, write-ahead logging, crash recovery, replication, and distributed systems work from the ground up.

The goal is not to build another production-ready database, but to progressively implement the core ideas behind modern databases while keeping the system understandable and the design decisions explicit.

---

## Goals

RookDB is built around a few principles:

- Build systems from first principles instead of hiding complexity behind libraries.
- Understand **why** database components exist, not just how to implement them.
- Make durability and failure handling explicit design concerns.
- Keep components small and independently understandable.
- Document design decisions, invariants, tradeoffs, and failure assumptions alongside the implementation.
- Progress from a simple local database toward distributed-system concepts.

---

## Current Status

### RookDB v2.0 — Complete

This version of RookDB intoduces:

- Skip-list-based in-memory MemTable
- Immutable, sorted String Table (SSTable) storage
- SSTable data blocks with CRC32 integrity checks
- Sparse indexing for efficient SSTable lookups
- Bloom filters to reduce unnecessary disk reads
- Tombstone-based deletion handling
- Active and frozen generation management
- Automatic MemTable flushing to SSTables
- Atomic SSTable publication through temporary files and rename
- WAL reclamation after successful SSTable publication
- Recovery across multiple WAL generations and SSTables
- Background flush worker and result handling
- Graceful shutdown with pending flush processing
- Flush failure handling that preserves WAL data for recovery
- Unit and integration tests covering storage, recovery, flushing, and failure scenarios

The v2 storage model:

```text
    Client Command
          │
          ▼
        Parser
          │
          ▼
        Engine
    ┌──────┴──────┐
    │             │
  WRITE           READ
    │             │
    ▼            ▼
   WAL         MemTable
    │             │
    ▼             │
   Disk           │
    │             │
    ▼            ▼
  MemTable ◄── SSTables
    │             │
    ▼            ▼
Current State   Persistent State
```

The SSTable acts as the persistent state of entries, while the memtable represents the current state.

---

## Roadmap

RookDB will evolve incrementally from a simple persistent key-value store into a distributed storage system.

### Phase 1 — Persistent Storage

- [x] Basic key-value operations
- [x] Write-Ahead Log
- [x] Durable writes
- [x] WAL recovery
- [x] Corruption detection
- [x] WAL tail truncation

### Phase 2 — LSM Storage Engine

- [x] Memtable
- [x] SSTables
- [x] Immutable memtables
- [x] Flush pipeline
- [ ] Compaction
- [x] Bloom filters
- [ ] Snapshots

### Phase 3 — Crash Recovery & Reliability

- [ ] Improved recovery model
- [ ] Snapshot-based recovery
- [ ] Fault injection
- [ ] Recovery testing
- [ ] Performance benchmarks

### Phase 4 — Distributed RookDB

- [ ] Node-to-node communication
- [ ] Replication
- [ ] Leader election
- [ ] Raft consensus
- [ ] Log replication
- [ ] Fault recovery
- [ ] Network failure handling

### Phase 5 — Advanced Features

- [ ] Transactions
- [ ] Concurrency
- [ ] Metrics
- [ ] Observability
- [ ] Administrative tooling
- [ ] Performance tuning

The roadmap is intentionally incremental. Each stage builds on concepts introduced by the previous one. Also, the roadmap is prone to change.

---

## Running

### Build

```bash
go build -o rookdb ./cmd/rookdb
```

### Start RookDB

RookDB expects a **data directory** rather than a direct WAL path:

```bash
./rookdb ./data
```

RookDB manages its internal storage files inside that directory.

For v2, the layout is:

```text
data/
└── 000004.wal - Current generation
└── 000003.sst
└── 000002.sst
└── 000001.sst
```

The `.wal` file is there for persistent in memory state while `.sst` files are on disk key value storage of older data.

### Commands

```text
SET <key> <value>
GET <key>
DELETE <key>
```

Example:

```text
$ ./rookdb ./data

RookDB v1.0.0

>> SET name Aryan
OK

>> GET name
Aryan

>> DELETE name
OK

>> GET name
key not found
```

Press `Ctrl+D` to exit the interactive shell.

### CLI Options

```text
-h, --help       Show help
-v, --version    Show version
```

---

## Architecture

RookDB v2.0 is built around a few focused components that work together to provide durable storage using a WAL, an in-memory MemTable, and immutable SSTables.

```text
                 ┌──────────────┐
                 │    Client    │
                 └──────┬───────┘
                        │
                        ▼
                 ┌──────────────┐
                 │    Parser    │
                 └──────┬───────┘
                        │
                        ▼
                 ┌──────────────┐
                 │    Engine    │
                 └──────┬───────┘
                        │
                 ┌──────┴───────┐
                 ▼              ▼
          ┌────────────┐  ┌────────────┐
          │    WAL     │  │  MemTable  │
          └─────┬──────┘  └─────┬──────┘
                │               │
                ▼               ▼
              Disk          Background
                              Flush
                                │
                                ▼
                         ┌────────────┐
                         │  SSTables  │
                         └─────┬──────┘
                               │
                               ▼
                              Disk
```

### Parser

The parser reads newline-delimited commands from the client and converts them into:

```text
command
key
value
```

It handles command tokenization and command canonicalization without knowing anything about the storage engine.

### Engine

The storage engine coordinates reads, writes, generation management, and background flushing.

Mutating operations follow:

```text
SET / DELETE
     │
     ▼
 Append operation to WAL
     │
     ▼
     Sync WAL
     │
     ▼
 Update MemTable
     │
     ▼
 Return success
```

When the MemTable reaches its flush threshold, it is frozen and asynchronously written to an SSTable. New writes continue in a fresh generation.

`GET` checks the active MemTable first, followed by SSTables from newest to oldest. Tombstones ensure that deleted values are not resurrected from older SSTables.

### Write-Ahead Log

The WAL is the durable history of database mutations.

RookDB v2 retains the following record format:

```text
┌───────────────┬───────────────┬───────────────┐
│ Length (4B)   │ Payload (N B) │ CRC32 (4B)    │
└───────────────┴───────────────┴───────────────┘
```

- Length is stored as a big-endian `uint32`.
- Payload is a serialized `Operation`.
- CRC32 covers the length and payload.
- The WAL is synchronized before a successful write is returned.

During startup, RookDB discovers existing WAL generations and SSTables, validates them, and replays unreconciled WAL records to reconstruct the current state.

### MemTable and SSTables

The MemTable is an in-memory, skip-list-based structure that maintains the latest state of a generation.

When frozen, its contents are written in sorted order to an immutable SSTable.

SSTables use:

- Data blocks with CRC32 integrity checks.
- Sparse indexes for locating data blocks.
- Bloom filters to avoid unnecessary disk reads.
- Tombstones to preserve deletion semantics across storage generations.

The WAL is reclaimed only after the corresponding SSTable has been successfully published.

---

## Failure Model

RookDB v2 maintains the same durability ordering for mutations:

```text
Client request
      │
      ▼
Write WAL
      │
      ▼
Sync WAL
      │
      ▼
Update MemTable
      │
      ▼
Return success
```

The WAL remains the source of truth for unflushed mutations. Once a generation is successfully flushed and its SSTable published, the corresponding WAL can be reclaimed.

Incomplete or corrupted WAL tails are detected during recovery and truncated. Invalid SSTables cause startup to fail rather than silently falling back to potentially stale data.

More detailed design reasoning and invariants are documented in:

```text
docs/design.md
```

## Project Structure

```text
rookdb/
├── cmd/
│   └── rookdb/
│       └── main.go         -- Exposes CLI to talk to DB
│
├── internal/
│   ├── operation/          -- Defines a rookdb operation
│   ├── wal/                -- Defines implementation of wal
│   ├── engine/             -- Connects all components in one
│   └── parser/             -- Helps in CLI parsing
│   └── memtable/           -- Skip list based memtable implementation
│   └── sstable/            -- Contain SSTable internals (blocks, reader, writer, bloom filter, etc)
│
├── docs/
│   └── changelog/          -- Old version README's
│   └── design/             -- Internal design docs
│   └── design.md           -- Links all design docs
│
├── go.mod
└── README.md
```

The `internal` packages intentionally keep RookDB's implementation details private while the CLI remains the entry point for interacting with the database.

---

## Why RookDB?

Databases are usually consumed through APIs, drivers, and abstractions that hide most of the machinery underneath.

RookDB takes the opposite approach.

Instead of starting with a feature-rich database and learning how to use it, RookDB starts with almost nothing and gradually builds the machinery required to make a database reliable.

That means dealing with questions such as:

- What does it actually mean for a write to be durable?
- What happens if a process crashes halfway through a WAL record?
- How do we detect corrupted data?
- Where should recovery happen?
- Which layer owns validation?
- What should happen when the final WAL record is incomplete?
- How does an in-memory state become persistent?
- How does a single-node database evolve into a replicated system?
- How do multiple machines agree on the same state?

The project is therefore as much about **systems engineering and reasoning about failure** as it is about writing Go code.

RookDB is being built one layer at a time, with the design decisions and tradeoffs documented along the way.

---

## License

Rook DB is licensed under Apache 2.0

## Performance Baseline

RookDB v2.0 was benchmarked after v2.0 was completed to accept SET, GET and DELETE commands.

The benchmarks measure the core `Engine` directly rather than going through the CLI, avoiding terminal I/O and command parsing overhead.

### Environment

- CPU: AMD Ryzen 5 5600H
- Architecture: amd64
- OS: Linux
- Go: 1.27.x
- Benchmark command:

```bash
go test ./internal/engine -bench=. -benchmem
```

### Results

```
Operation       v1                  v2        Regression
SET            2,147ns/op	    2,500ns/op      16.4%
GET	           16.8 ns/op	    178.4 ns/op	    10.62×
DELETE	       2,070 ns/op	    2,394 ns/op	    15.7%
```

The drop in performance can be due to new advance data structures and also no optimizations.

#### Workloads

##### SET

- 10,000 pre-generated key/value pairs
- Keys and values are cycled through during the benchmark
- Each operation is WAL-backed and synchronized to disk

##### GET

- 10,000 keys preloaded directly into the in-memory store
- Keys are cycled through during the benchmark
- No WAL operation is involved

##### DELETE

- Existing keys from a 10,000-key pool
- Keys are repopulated outside the timed region
- DELETE operations are WAL-backed and synchronized to disk

These numbers are intended as a baseline for comparing future storage-engine implementations rather than as absolute performance claims.
