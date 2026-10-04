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

### RookDB v1.0 — Complete

The first version of RookDB provides a persistent key-value database with:

- Interactive command-line interface
- `SET`, `GET`, and `DELETE` operations
- In-memory materialized state
- Write-Ahead Log (WAL)
- Durable WAL writes using filesystem synchronization
- Length-prefixed WAL records
- CRC32 integrity checks
- WAL recovery after restart
- Detection of incomplete/corrupted WAL records
- Automatic truncation of invalid WAL tails
- Operation validation during recovery
- Persistent database data directory
- Clean database shutdown
- Unit tests covering normal operation and WAL corruption cases

The v1 storage model is intentionally simple:

```text
Client Command
      │
      ▼
   Parser
      │
      ▼
   Engine
    ┌─┴─┐
    ▼   ▼
  WAL  Map
   │    │
   ▼    ▼
Disk  Current State
```

The WAL acts as the durable history of mutations, while the in-memory map represents the current materialized state.

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
- [ ] Immutable memtables
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

For v1, the layout is:

```text
data/
└── rookdb.wal
```

This keeps the database's storage layout separate from the executable and leaves room for future storage components such as SSTables and snapshots.

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

RookDB v1.0 is deliberately small and is divided into a few focused components.

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
          │    WAL     │  │  In-Memory │
          │            │  │    Map     │
          └─────┬──────┘  └────────────┘
                │
                ▼
             Disk
```

### Parser

The parser is responsible for reading newline-delimited commands from the client and converting them into:

```text
command
key
value
```

It handles command tokenization and command canonicalization, but does not know anything about the storage engine.

### Engine

The storage engine owns the current database state and coordinates reads and writes.

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
 Update in-memory state
     │
     ▼
 Return success
```

This ordering ensures that an acknowledged mutation has corresponding durable WAL information that can be replayed after a crash.

`GET` is a read-only operation and does not generate a WAL record.

### Write-Ahead Log

The WAL is the durable history of database mutations.

RookDB v1 uses the following record format:

```text
┌───────────────┬───────────────┬───────────────┐
│ Length (4B)   │ Payload (N B) │ CRC32 (4B)    │
└───────────────┴───────────────┴───────────────┘
```

- Length is stored as a big-endian `uint32`.
- Payload is a serialized `Operation`.
- CRC32 covers the length and payload.
- The WAL is synchronized before a successful write is returned.

During startup, RookDB sequentially scans the WAL and reconstructs the in-memory state.

If the WAL ends with an incomplete or corrupted record, RookDB truncates the invalid tail and stops recovery at that point.

### In-Memory State

The current state of the database is maintained in memory as a Go map.

Conceptually:

```text
WAL = durable history
Map = current materialized state
```

The map can be reconstructed from the WAL, which allows the database to recover its state after restarting.

---

## Failure Model

RookDB v1 explicitly considers failures that can occur during a write.

The important ordering is:

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
Update memory
      │
      ▼
Return success
```

This avoids acknowledging a write before its durable WAL record exists.

During recovery, RookDB treats the WAL as the source of truth for mutations.

A structurally invalid WAL record is not skipped. Recovery stops at the invalid record and truncates the WAL from that point, preventing potentially unsafe later records from being replayed.

More detailed design reasoning and invariants are documented in:

```text
docs/design.md
```

---

## Project Structure

```text
rookdb/
├── cmd/
│   └── rookdb/
│       └── main.go
│
├── internal/
│   ├── operation/
│   │   └── operation.go
│   │
│   ├── wal/
│   │   └── wal.go
│   │
│   ├── engine/
│   │   └── engine.go
│   │
│   └── parser/
│       └── parser.go
│
├── docs/
│   └── design.md
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

## Design Documentation

The implementation is accompanied by a design document containing the reasoning behind major architectural decisions, including:

- WAL format
- Durability semantics
- Recovery behavior
- Record validation
- Corruption handling
- Truncation strategy
- Component responsibilities
- Failure assumptions
- Design tradeoffs

See:

```text
docs/design.md
```

---

## License

Rook DB is licensed under Apache 2.0

## Performance Baseline

RookDB v1.0 was benchmarked before beginning the LSM-based storage engine work.

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
Operation	  Time	            Memory	      Allocations

SET         ~2.15 µs/op	        229 B/op	  5 allocs/op
GET	        ~16.8 ns/op	         0 B/op	      0 allocs/op
DELETE	    ~2.07 µs/delete	    194 B/op	  5 allocs/op
```

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
