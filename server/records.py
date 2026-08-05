"""Reader for self-play trajectory shards written by engine/record.go.

Wire format (little-endian throughout, matching Go's binary.LittleEndian), one
record per finished game:

    uint32 nPlies | nPlies * ENCODING_DIM float32 | uint8 whiteWon

The writer rotates to a new shard file at ~64MB, always between whole records,
so concatenating shards in name order reconstructs the full record stream.

Both readers here are tolerant of a truncated trailing record: a shard may be
read while a concurrent writer is still appending, so a short read at a record
boundary (or mid-record) is treated as a clean end-of-stream rather than an
error. A tailer can therefore re-read the same file later and pick up records
that have since been completed. This mirrors ReadRecords in engine/record.go.
"""

import numpy as np

# Must stay in lockstep with EncodingDim in engine/encode.go (and ENCODING_DIM
# in server/serve.py).
ENCODING_DIM = 198

_HEADER_BYTES = 4  # uint32 nPlies
_STRIDE_BYTES = ENCODING_DIM * 4  # one board encoding: ENCODING_DIM float32
_OUTCOME_BYTES = 1  # uint8 whiteWon


def _read_exact(f, n):
    """Read exactly n bytes from f, or return None on a short read.

    A short read means the stream ended (or a record is only partially written),
    so callers stop cleanly. n == 0 returns an empty bytes object (never None).
    """
    if n == 0:
        return b""
    buf = f.read(n)
    if len(buf) < n:
        return None
    return buf


def iter_records(f):
    """Yield (encs, white_won) for each complete record in the open binary file f.

    encs is an (nPlies, ENCODING_DIM) float32 numpy array (a fresh copy safe to
    keep); white_won is a Python bool. Iteration stops without error at a clean
    EOF or at the first truncated/partial trailing record.
    """
    while True:
        header = _read_exact(f, _HEADER_BYTES)
        if header is None:
            return  # clean EOF, or partial header from an in-progress append
        n_plies = int(np.frombuffer(header, dtype="<u4")[0])

        payload = _read_exact(f, n_plies * _STRIDE_BYTES)
        if payload is None:
            return  # record still being appended; stop cleanly

        outcome = _read_exact(f, _OUTCOME_BYTES)
        if outcome is None:
            return  # missing trailing outcome byte; stop cleanly

        encs = np.frombuffer(payload, dtype="<f4").reshape(n_plies, ENCODING_DIM).copy()
        yield encs, outcome[0] == 1


def read_records(path):
    """Read every complete record from the shard file at path.

    Returns a list of (encs, white_won) tuples. Convenience wrapper over
    iter_records for callers that want the whole file at once.
    """
    with open(path, "rb") as f:
        return list(iter_records(f))
