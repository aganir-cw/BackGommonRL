import marimo

__generated_with = "0.23.8"
app = marimo.App(width="medium")


@app.cell
def _():
    from pathlib import Path

    import numpy as np
    import torch

    from model import ValueNet
    from records import read_records

    return Path, ValueNet, np, read_records, torch


@app.cell
def _(torch):
    def pick_device() -> torch.device:
        """Prefer CUDA, then Apple MPS, then CPU."""
        if torch.cuda.is_available():
            return torch.device("cuda")
        if torch.backends.mps.is_available():
            return torch.device("mps")
        return torch.device("cpu")

    return (pick_device,)


@app.cell
def _(pick_device):
    device = pick_device()
    print(f"device: {device}")
    return (device,)


@app.cell
def _(Path, read_records):
    # Step 0 unit check: a handful of games so positions are near-unique and the
    # tiny MLP can memorize them -> loss should fall close to zero.
    record_dir = "records"
    file_path = Path(record_dir) / "178596576127438600000000.bin"
    records_list = read_records(file_path)[:10]
    print(f"loaded {len(records_list)} games")
    return (records_list,)


@app.cell
def _(X, device, np, records_list, torch, y):
    # Flatten: stack every game's (nPlies, 198) afterstates into one (N, 198)
    # matrix. Broadcast: repeat each game's single outcome across its plies so
    # every position carries its game's label (V(s) = P(white wins)).
    encs_list = [encs for encs, _ in records_list]
    _X = np.concatenate(encs_list, axis=0).astype(np.float32)

    lengths = [len(e) for e in encs_list]
    outcomes = np.array([float(won) for _, won in records_list], dtype=np.float32)
    _y = np.repeat(outcomes, lengths)

    assert _X.shape[0] == _y.shape[0]  # one label per position
    assert _X.shape[1] == 198

    Xt = torch.from_numpy(_X).to(device)  # (N, 198), already float32
    yt = torch.from_numpy(_y).to(device)  # (N,)

    print(f"X: {X.shape}  y: {y.shape}")
    return Xt, yt


@app.cell
def _(ValueNet, Xt, device, torch, yt):
    # Overfit loop: recompute loss from a fresh forward pass every step, then
    # zero_grad -> backward -> step. A single cell holds the whole loop so
    # marimo isn't asked to thread mutable training state across cells.
    model = ValueNet().to(device)
    opt = torch.optim.Adam(model.parameters(), lr=1e-3)

    for step in range(2000):
        opt.zero_grad()
        pred = model(Xt).squeeze(-1)  # (N, 1) -> (N,)
        loss = torch.nn.functional.mse_loss(pred, yt)
        loss.backward()
        opt.step()
        if step % 100 == 0:
            print(f"step={step:4d}  loss={loss.item():.4f}")

    print(f"final loss={loss.item():.4f}")
    return


@app.cell
def _():
    CAP, DIM = 500_000, 198

    return (CAP,)


@app.cell
def _(CAP, X, cap, np, write_ptr, y):
    class ReplayBuffer:
        def __init__(self):
            CAP, DIM = 500000, 198
            self.X = np.zeros((CAP, DIM), dtype = np.float32)
            self.y = np.zeros(CAP, dtype=np.float32)
            self.write_ptr = 0
            self.filled = 0
        def add(self, encs, outcome):
            n = len(encs)
            label = 1.0 if outcome else 0.0
            end = self.write_ptr + n

            if end <= CAP:
                self.X[self.write_ptr: end] = encs
                self.y[self.write_ptr: end] = label
            else:
                first = CAP - write_ptr
                X[write_ptr:CAP] = encs[:first]  # tail of the array
                y[write_ptr:CAP] = label
                X[0:n - first]   = encs[first:]  # wrap to the front
                y[0:n - first]   = label
            self.write_ptr = end % cap
            self.filled = min(self.filled + n, CAP)

    return


@app.cell
def _():
    return


@app.cell
def _():
    return


if __name__ == "__main__":
    app.run()
