# analysis/eval_loop.py  (proposed)
import csv, subprocess, time, os

TRAIN_LOG = "analysis/results/train_log.csv"
EVAL_LOG  = "analysis/results/eval_log.csv"
EVERY     = 20_000
OPPONENTS = [("random", 500), ("pip", 1000)]

def latest_games_seen() -> int:
    try:
        with open(TRAIN_LOG) as f:
            rows = list(csv.DictReader(f))
        return int(rows[-1]["games_seen"]) if rows else 0
    except (FileNotFoundError, IndexError, KeyError):
        return 0

def run_eval(opponent, pairs, games):
    print(f"[eval] games={games} vs {opponent} ({pairs} pairs)")
    subprocess.run(
        ["go", "run", "./cmd/eval", "-a", "greedy", "-b", opponent,
         "-pairs", str(pairs), "-games", str(games), "-out", EVAL_LOG],
        check=True,
    )

def main():
    next_at = EVERY
    while True:
        g = latest_games_seen()
        if g >= next_at:
            for opp, pairs in OPPONENTS:
                run_eval(opp, pairs, g)
            next_at = (g // EVERY + 1) * EVERY
        time.sleep(30)

if __name__ == "__main__":
    main()