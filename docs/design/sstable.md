# SSTable Design

## Goals and lifecycle

An SSTable is an immutable on-disk representation of a flushed MemTable. The design favors sequential writes, sorted records, bounded point lookups, local corruption detection, and a clear finalization boundary.

```text
MemTable iterator → buffered data blocks → sparse index → Bloom filter → footer
```

Files are usable only after complete finalization. A crash during block, index, Bloom filter, or footer construction leaves an incomplete file that must not be discovered as persistent state. The WAL remains the recovery source for mutations not represented by finalized SSTables.

## File layout

```text
Data blocks
  [BLOCK_LEN:uint32][BLOCK_DATA][CRC32:uint32] ...
Sparse index
  [KEY_LEN:uint16][KEY][BLOCK_OFFSET:uint64] ...
Sparse index CRC32
Bloom filter
  [HASH_COUNT:uint8][BITSET]
Bloom filter CRC32
Footer
  [BLOOM_OFFSET:uint64][BLOOM_SIZE:uint64]
  [INDEX_OFFSET:uint64][INDEX_SIZE:uint64]
Footer CRC32
```

All fixed-width integers use the same byte order. `BLOCK_LEN` counts only block data. A data-block CRC covers `BLOCK_DATA`. The sparse-index CRC covers all encoded index entries; `INDEX_SIZE` excludes its CRC. The Bloom filter CRC covers its complete encoding, including `HASH_COUNT`; `BLOOM_SIZE` excludes that CRC. The footer CRC covers all footer fields, including `MAGIC`.

The footer fields before its CRC occupy 32 bytes, so the reader locates the footer at a fixed offset from EOF. The footer CRC immediately follows those fields. For finding the footer the reader can simply seek 36 bytes (Footer + CRC) back from EOF.

The index and Bloom filter each have separate integrity checks from the data blocks and footer. `BLOOM_OFFSET` and `INDEX_OFFSET` are absolute offsets from the beginning of the file. Each size field covers only its corresponding encoded region and excludes its CRC.

## Records and blocks

The record encoding is:

```text
[TYPE:uint8][KEY_LEN:uint16][VALUE_LEN:uint16][KEY_BYTES][VALUE_BYTES]
```

Types are `SET = 0x01` and `DELETE = 0x02`; DELETE has a zero-length value. Records are sorted by key. The MemTable's ordered iterator avoids a separate sort.

The block limit is 16 records, roughly 2 KiB under the 128-byte average key/value size assumption. The writer buffers a complete block, computes its length and checksum, then writes it sequentially. The final block may be smaller.

## Sparse index

The writer records the first key and file offset for each block while writing. Each index offset points to the start of the block framing (`BLOCK_LEN`). The index is kept in memory during the flush and then written sequentially with its checksum.

## Bloom filter

Each SSTable contains a Bloom filter for every distinct key in that file, including keys whose latest entry is a DELETE tombstone. A Bloom filter may report a false positive, but it must not report a false negative for an inserted key.

The chosen false-positive probability is 5%. For `n` distinct keys and probability `p`, the bit count `m` and hash count `k` are calculated as:

```text
m = -(n × ln(p)) / (ln(2))²
k = round((m / n) × ln(2))
```

For `p = 0.05`, this is approximately 6.24 bits per key and 4 hash functions. Round the calculated bit count up to a whole number of bytes for the bitset; its effective bit count is its byte length multiplied by eight. The Bloom filter encoding stores `HASH_COUNT` as one byte followed by the bitset. The reader derives the bitset length from `BLOOM_SIZE - 1`, and therefore derives the effective bit count without loading SSTable data blocks.

For an SSTable with about 419,000 keys, the bitset is approximately 327 KB, plus the one-byte hash count and four-byte checksum. This estimate uses the MemTable's 80 MiB flush threshold and approximate 128-byte average key/value payload model.

## Lookup

SSTables are searched newest to oldest. For each file:

1. Ask its Bloom filter about the key. A definite miss proceeds to the next older SSTable.
2. For a possible match, binary-search the sparse index for a candidate block, seek to it, verify its checksum, and scan at most 16 records.
3. If the key is absent from that file, proceed to the next older SSTable.
4. If a matching `SET` is found, return its value. If a matching `DELETE` is found, return not-found immediately and do not search older files.

The MemTable is checked before SSTables. A MemTable `SET` returns its value; a tombstone returns not-found without consulting SSTables.

```text
GET key
  ↓
MemTable ── SET → return value
  │
  ├── DELETE → return not-found
  └── absent
        ↓
SSTables newest to oldest
  ├── Bloom says absent → next older SSTable
  └── Bloom says possible → index → block → record
                              ├── SET → return value
                              ├── DELETE → return not-found
                              └── absent → next older SSTable
```

## Naming and discovery

SSTables use the filename pattern `sstable-<generation>.sst`, where `<generation>` is an unsigned decimal number zero-padded to 10 digits, for example `sstable-0000000042.sst`. Generation numbers increase monotonically and are never reused within a database. Generation allocation fails rather than wrapping at the `uint64` limit.

At startup, discover files matching this pattern, sort by generation in descending order, open each file, validate its footer and metadata, and load its sparse index and Bloom filter into memory. File data blocks remain on disk and are read on demand. Files without a valid finalized footer are incomplete and are excluded from recovery.

## Finalization and crash behavior

The writer completes and checksums all data blocks, writes the sparse index and its checksum, writes the Bloom filter and its checksum, then writes the footer and its checksum. A valid complete footer marks finalization; bytes earlier in the file do not.

File publication and synchronization preserve the finalization boundary across crashes. Incomplete files are excluded during discovery. The WAL remains the recovery source for mutations that are not represented by finalized SSTables.
