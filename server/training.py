"""
Building blocks:

    Step 0  flatten_records / unit_check  -- reader+label+model wiring
    Step 1  pick_device                   -- CUDA -> MPS -> CPU
    Step 2  ReplayBuffer                   -- preallocated numpy ring
    Step 3  ShardTailer                    -- consume only new complete records

The __main__ block wires the live loop:

    Step 4  tail -> sample -> MSE step
    Step 5  POST state_dict to serve.py /weights every SYNC_EVERY steps
    Step 6  log (step, loss, buffer_fill, games_seen, syncs) to stdout + CSV

Run from the repo root as a module so the absolute imports resolve:

    uv run python -m server.training
"""

import csv
import glob
import os
import io
import time
import numpy as np
import torch
import urllib.request
from server.model import ValueNet
from server.records import iter_records, read_records

ENCODING_DIM = 198
BATCH_SIZE =  8192
POLL_EVERY = 50  # tail the shard dir every N steps, not every step
SYNC_EVERY = 500  # push weights to the inference server every N steps
LOG_EVERY = 100  # append a metrics row (stdout + CSV) every N steps
LOG_PATH = os.path.join("analysis", "results", "train_log.csv")

SERVER_URL = "http://localhost:8000"


def pick_device() -> torch.device:
    """Prefer CUDA, then Apple MPS, then CPU."""
    if torch.cuda.is_available():
        return torch.device("cuda")
    if torch.backends.mps.is_available():
        return torch.device("mps")
    return torch.device("cpu")


def flatten_records(records_list):
    """Flatten [(encs, won), ...] into (X, y) training arrays.

    Stacks every game's (nPlies, 198) afterstates into one (N, 198) matrix and
    broadcasts each game's single outcome across its plies, so every position
    carries its game's label: V(s) = P(white wins), 1.0 if whiteWon else 0.0.
    This is the Monte-Carlo value target -- no per-ply labels, no perspective
    flip.
    """
    encs_list = [encs for encs, _ in records_list]
    X = np.concatenate(encs_list, axis=0).astype(np.float32)

    lengths = [len(e) for e in encs_list]
    outcomes = np.array([float(won) for _, won in records_list], dtype=np.float32)
    y = np.repeat(outcomes, lengths)

    assert X.shape[0] == y.shape[0], "one label per position"
    assert X.shape[1] == ENCODING_DIM
    return X, y


class ReplayBuffer:
    """Preallocated numpy ring buffer of (state, outcome) rows.

    Bounded memory, O(1) add, and uniform sampling over the most recent `cap`
    positions -- old trajectories age out naturally as the policy improves.
    """

    def __init__(self, cap: int = 500_000, dim: int = ENCODING_DIM):
        self.cap = cap
        self.X = np.zeros((cap, dim), dtype=np.float32)
        self.y = np.zeros(cap, dtype=np.float32)
        self.write_ptr = 0  # next slot to write
        self.filled = 0  # rows containing real data, capped at cap

    def add(self, encs, outcome) -> None:
        """Write len(encs) rows, wrapping across the end of the ring if needed."""
        n = len(encs)
        if n == 0:
            return
        label = 1.0 if outcome else 0.0
        cap = self.cap
        start = self.write_ptr
        end = start + n

        if end <= cap:
            # Fits without wrapping: one contiguous slice.
            self.X[start:end] = encs
            self.y[start:end] = label
        else:
            # Splits across the boundary: fill the tail, then wrap to the front.
            first = cap - start
            self.X[start:cap] = encs[:first]
            self.y[start:cap] = label
            rest = n - first
            self.X[0:rest] = encs[first:]
            self.y[0:rest] = label

        self.write_ptr = end % cap
        self.filled = min(self.filled + n, cap)

    def sample(self, n: int):
        """Uniformly sample n rows from the filled region. Warm up before calling."""
        idx = np.random.randint(0, self.filled, size=n)
        return self.X[idx], self.y[idx]


class ShardTailer:
    """Consume only the new, complete records from a growing dir of *.bin shards.

    Keeps a per-file byte offset. Each poll seeks to where it left off and feeds
    the file object to iter_records, which stops cleanly at a partial trailing
    record (a game still being appended). f.tell() then lands exactly after the
    last complete record, so the next poll resumes there -- no re-reading from 0
    (which would be O(n^2) and double-count) and no reading half-written games.
    """

    def __init__(self, record_dir: str):
        self.record_dir = record_dir
        self.offsets: dict[str, int] = {}
        self.games_seen = 0

    def poll(self, buffer: ReplayBuffer) -> int:
        """Ingest all new complete records into buffer. Returns games added this poll."""
        added = 0
        for path in sorted(glob.glob(os.path.join(self.record_dir, "*.bin"))):
            with open(path, "rb") as f:
                f.seek(self.offsets.get(path, 0))
                for encs, won in iter_records(f):
                    buffer.add(encs, won)
                    added += 1
                self.offsets[path] = f.tell()
        self.games_seen += added
        return added


def unit_check(path: str, n_games: int = 10, steps: int = 2000) -> float:
    """Step 0 gate: overfit a handful of games to near-zero loss.

    Proves the reader + label broadcast + model wiring end-to-end. If a tiny MLP
    can't memorize a few hundred positions, the reader/labels are wrong, not the
    model. Returns the final loss; asserts it is small.
    """
    torch.manual_seed(0)
    device = pick_device()
    print(f"device: {device}")

    records_list = read_records(path)[:n_games]
    print(f"loaded {len(records_list)} games")

    X, y = flatten_records(records_list)
    print(f"X: {X.shape}  y: {y.shape}")

    Xt = torch.from_numpy(X).to(device)
    yt = torch.from_numpy(y).to(device)

    model = ValueNet().to(device)
    opt = torch.optim.Adam(model.parameters(), lr=1e-3)

    loss = torch.tensor(float("inf"))
    for step in range(steps):
        opt.zero_grad()
        pred = model(Xt).squeeze(-1)  # (N, 1) -> (N,)
        loss = torch.nn.functional.mse_loss(pred, yt)
        loss.backward()
        opt.step()
        if step % 100 == 0:
            print(f"step={step:4d}  loss={loss.item():.4f}")

    final = loss.item()
    print(f"final loss={final:.4f}")
    assert final < 1e-2, f"overfit failed: loss={final:.4f} (check reader/labels)"
    return final


if __name__ == "__main__":
    #from pathlib import Path

    #unit_check(str(Path("records") / "178596576127438600000000.bin"))

    # loop prerequisites:
    device = pick_device()

    tailer = ShardTailer(record_dir="records")
    buffer = ReplayBuffer()
    model = ValueNet().to(device)

    opt = torch.optim.Adam(model.parameters(), lr=1e-3)

    step = 0
    syncs = 0

    # Step 6: open the metrics CSV up front and flush every row so a crash keeps
    # partial data. Columns mirror the analysis/results/sweep.csv style.
    os.makedirs(os.path.dirname(LOG_PATH), exist_ok=True)
    log_file = open(LOG_PATH, "w", newline="")
    log_writer = csv.writer(log_file)
    log_writer.writerow(["step", "loss", "buffer_fill", "games_seen", "syncs"])
    log_file.flush()

    while True:
        # During warm-up step stays 0, so this polls every iteration until the
        # buffer fills; after that it polls once every POLL_EVERY steps.
        if step % POLL_EVERY == 0:
            tailer.poll(buffer)
        if buffer.filled < BATCH_SIZE:
            time.sleep(0.1)
            continue
        xb, yb = buffer.sample(BATCH_SIZE)
        xbt = torch.from_numpy(xb).to(device)
        ybt = torch.from_numpy(yb).to(device)
        pred = model(xbt).squeeze(-1)
        loss = torch.nn.functional.mse_loss(pred, ybt)
        opt.zero_grad()
        loss.backward()
        opt.step()
        step += 1

        # Step 5: push weights to the inference server (count it before logging).
        if step % SYNC_EVERY == 0:
            cpu_sd = {k: v.cpu() for k, v in model.state_dict().items()}
            buf = io.BytesIO()
            torch.save(cpu_sd, buf)
            req = urllib.request.Request(
                SERVER_URL + "/weights",
                data=buf.getvalue(),
                headers={"Content-Type": "application/octet-stream"},
            )
            with urllib.request.urlopen(req) as resp:
                resp.read()
            syncs += 1

        # Step 6: log every LOG_EVERY steps to stdout and the CSV.
        if step % LOG_EVERY == 0:
            loss_val = loss.item()
            print(
                f"step={step:6d}  loss={loss_val:.4f}  "
                f"buffer_fill={buffer.filled}  games_seen={tailer.games_seen}  "
                f"syncs={syncs}"
            )
            log_writer.writerow(
                [step, f"{loss_val:.6f}", buffer.filled, tailer.games_seen, syncs]
            )
            log_file.flush()