from server.smoke_test import start_encoding, score
import time

def main() -> None:
    x = start_encoding()[None, :]
    prev = None
    while True:
        v = float(score(x)[0])
        delta = "" if prev is None else f"{v - prev:.2f}"
        print(f"opening V(white)={v:.6f}{delta}", flush=True)

        prev = v
        time.sleep(2.0)
    

if __name__ == "__main__":
    main()