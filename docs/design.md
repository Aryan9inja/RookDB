## Operation Model

RookDB represents database mutations as structured operations
rather than persisting client commands directly.

An operation currently contains:

- Type
- Key
- Value

Supported operations:

- SET
- DELETE

For example:

    SET name Aryan

is represented internally as:

    Operation{
        OpType: Set,
        Key: "name",
        Value: "Aryan",
    }

### Why structured operations?

The WAL stores database history, not client requests.

This keeps the storage layer independent of the protocol used
by clients. A future HTTP, TCP, CLI, or binary protocol can
produce the same Operation representation.

## WAL Record Format

A WAL record consists of:

    [LENGTH][PAYLOAD][CRC32]

Where:

- LENGTH is a uint32 representing payload size.
- PAYLOAD is the serialized Operation.
- CRC32 covers LENGTH + PAYLOAD.

### Why include LENGTH in the checksum?

The length field participates in record framing and can itself
be corrupted. If the checksum covered only the payload, corruption
of the length field could go undetected.

Therefore:

    CRC32(LENGTH + PAYLOAD)