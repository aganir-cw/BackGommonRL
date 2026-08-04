"""Local round-trip smoke test for the /score endpoint (no Go required).

Start the server first, in another terminal:
    uv run python -m server.serve
then run:
    uv run python server/smoke_test.py

Values are meaningless with a randomly-initialized model; this only proves the
byte protocol and shapes end-to-end.
"""

import urllib.request

import numpy as np

URL = "http://127.0.0.1:8000"
ENCODING_DIM = 198

# Start() position, per engine/board.go and engine/encode.go.
WHITE_POINTS = {5: 5, 7: 3, 12: 5, 23: 2}   # point index -> checker count
BLACK_POINTS = {0: 2, 11: 5, 16: 3, 18: 5}  # point index -> checker magnitude


def start_encoding() -> np.ndarray:
    """Replicate engine/encode.go for the opening position."""
    enc = np.zeros(ENCODING_DIM, dtype="<f4")
    for i, n in WHITE_POINTS.items():
        enc[4 * i + 0] = n >= 1
        enc[4 * i + 1] = n >= 2
        enc[4 * i + 2] = n >= 3
        enc[4 * i + 3] = max(0.0, (n - 3) / 2)
    for i, m in BLACK_POINTS.items():
        enc[4 * i + 96] = m >= 1
        enc[4 * i + 97] = m >= 2
        enc[4 * i + 98] = m >= 3
        enc[4 * i + 99] = max(0.0, (m - 3) / 2)
    enc[196] = 1.0  # White to move
    return enc


def score(vectors: np.ndarray) -> np.ndarray:
    count = vectors.shape[0]
    body = np.array([count], dtype="<u4").tobytes() + vectors.astype("<f4").tobytes()
    req = urllib.request.Request(
        URL + "/score",
        data=body,
        headers={"Content-Type": "application/octet-stream"},
    )
    with urllib.request.urlopen(req) as resp:
        return np.frombuffer(resp.read(), dtype="<f4")


def main() -> None:
    with urllib.request.urlopen(URL + "/healthz") as r:
        assert r.read() == b"ok", "healthz did not return ok"

    start = start_encoding()

    probs1 = score(start[None, :])  # count = 1
    assert probs1.shape == (1,), probs1.shape
    print(f"count=1 -> {probs1.tolist()}")

    batch = np.repeat(start[None, :], 3, axis=0)  # count = 3
    probs3 = score(batch)
    assert probs3.shape == (3,), probs3.shape
    print(f"count=3 -> {probs3.tolist()}")

    print("smoke test OK")


if __name__ == "__main__":
    main()
