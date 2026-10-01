# SSTable Design

## Goals and lifecycle

An SSTable is an immutable on-disk representation of a flushed MemTable. The design favors sequential writes, sorted records, bounded point lookups, local corruption detection, and a clear finalization boundary.

```text
MemTable iterator → buffered data blocks → sparse index → index checksum → footer → footer checksum
```

Files are usable only after complete finalization. A crash during block, index, or footer construction leaves an incomplete file that must not be discovered as persistent state. The WAL remains the recovery source for mutations not represented by finalized SSTables.

## File layout

```text
Data blocks
  [BLOCK_LEN:uint32][BLOCK_DATA][CRC32:uint32] ...
Sparse index
  [KEY_LEN:uint16][KEY][BLOCK_OFFSET:uint64] ...
Sparse index CRC32
Footer
  [MAGIC][INDEX_OFFSET:uint64][INDEX_SIZE:uint64]
Footer CRC32
```

All fixed-width integers use a consistent byte order. `BLOCK_LEN` counts only block data. The block CRC covers `BLOCK_DATA`. The index CRC covers all encoded index entries; `INDEX_SIZE` excludes its CRC. The footer CRC covers the footer fields.

The footer is fixed-width, allowing a reader to locate it from EOF. It contains the index offset and size. The footer checksum and index checksum are separate integrity domains from each data block.

## Records and blocks

The record encoding is:

```text
[TYPE:uint8][KEY_LEN:uint16][VALUE_LEN:uint16][KEY_BYTES][VALUE_BYTES]
```

Types are `SET = 0x01` and `DELETE = 0x02`; DELETE has a zero-length value. Records are sorted by key. The MemTable's ordered iterator avoids a separate sort.

The block limit is 16 records, roughly 2 KiB under the 128-byte average key/value size assumption. The writer buffers a complete block, computes its length and checksum, then writes it sequentially. The final block may be smaller.

## Sparse index and lookup

The writer records the first key and file offset for each block while writing. Each index offset points to the start of the block framing (`BLOCK_LEN`). The index is kept in memory during the flush and then written sequentially with its checksum.

Lookup binary-searches the sparse index for a candidate block, seeks to that block, verifies its checksum, then scans at most 16 records.

When a database has multiple SSTables, the lookup layer searches newest to oldest. SET is definitive; DELETE is definitive not-found; absence allows lookup to continue. See [LSM tombstones](lsm.md#tombstones).

## Finalization and crash behavior

The writer completes data blocks, writes the sparse index, writes its checksum, writes the footer, and writes the footer checksum. A valid complete footer marks finalization; bytes earlier in the file do not.

On startup, only files with valid finalized metadata participate in recovery. Incomplete files are discarded during discovery. File publication and synchronization preserve the finalization boundary across crashes.
