"""Minimal single-threaded Flask inference server for ValueNet.

Protocol (little-endian throughout, matching Go's binary.LittleEndian):
  POST /score    body: uint32 count | count*198 float32   -> resp: count float32
  POST /weights  body: torch.save(state_dict) bytes        -> loads into live model
  GET  /healthz                                            -> "ok"

Run from the repo root as a module (keeps the absolute import working):
    uv run python -m server.serve
or directly as a script:
    uv run server/serve.py
"""

import io
import os

import numpy as np
import torch
from flask import Flask, request

try:  # `python -m server.serve` from repo root
    from server.model import ValueNet
except ModuleNotFoundError:  # `python server/serve.py` (server/ is on sys.path)
    from model import ValueNet

# Must stay in lockstep with EncodingDim in engine/encode.go.
ENCODING_DIM = 198


def pick_device() -> torch.device:
    """Prefer CUDA, then Apple MPS, then CPU."""
    if torch.cuda.is_available():
        return torch.device("cuda")
    if torch.backends.mps.is_available():
        return torch.device("mps")
    return torch.device("cpu")


device = pick_device()
model = ValueNet().to(device)
model.eval()

app = Flask(__name__)


@app.route("/score", methods=["POST"])
def score():
    body = request.get_data()
    if len(body) < 4:
        return "score: body shorter than 4-byte count header", 400

    count = int(np.frombuffer(body[:4], dtype="<u4")[0])
    expected = 4 + count * ENCODING_DIM * 4
    if len(body) != expected:
        return (
            f"score: body length {len(body)} != expected {expected} "
            f"for count={count}",
            400,
        )

    # frombuffer yields a read-only view; copy so torch.from_numpy is happy.
    x = np.frombuffer(body[4:], dtype="<f4").reshape(count, ENCODING_DIM).copy()
    with torch.inference_mode():
        t = torch.from_numpy(x).to(device)
        probs = model(t).squeeze(-1)  # (count, 1) -> (count,)

    resp = probs.to("cpu").numpy().astype("<f4").tobytes()
    return resp, 200, {"Content-Type": "application/octet-stream"}


@app.route("/weights", methods=["POST"])
def weights():
    sd = torch.load(io.BytesIO(request.get_data()), map_location=device)
    model.load_state_dict(sd)
    model.eval()
    return "ok"


@app.route("/healthz", methods=["GET"])
def healthz():
    return "ok"


if __name__ == "__main__":
    port = int(os.environ.get("PORT", "8000"))
    # Single-threaded so a /weights swap can never race a /score call.
    app.run(host="127.0.0.1", port=port, threaded=False)
