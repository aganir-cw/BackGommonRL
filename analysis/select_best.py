import glob
import os
import re
import shutil

import pandas as pd

df = pd.read_csv("analysis/results/eval_log.csv")
df_pip = df.loc[df["opponent"] == "pip"]
df_pip = df_pip.sort_values(by="winrate", ascending=False)
print(df_pip.head())

# Best eval point (by pip winrate). Its games_seen was sampled by eval_loop.py,
# which rarely lines up exactly with a checkpoint's games_seen (the trainer saves
# on its own ~20k boundaries), so we can't just build the filename directly.
target_games = int(df_pip.loc[df_pip["winrate"].idxmax(), "games_seen"])
print(f"best pip winrate {df_pip['winrate'].max():.4f} at games_seen={target_games}")

# Gap 3 fix: map to the checkpoint on disk whose games_seen is closest.
ckpts = glob.glob(os.path.join("checkpoints", "ckpt_*.pth"))
if not ckpts:
    raise SystemExit("no checkpoints found in checkpoints/ckpt_*.pth")


def ckpt_games(path: str) -> int:
    return int(re.search(r"ckpt_(\d+)\.pth$", os.path.basename(path)).group(1))


best_checkpoint_path = min(ckpts, key=lambda p: abs(ckpt_games(p) - target_games))
print(f"nearest checkpoint: {best_checkpoint_path} (games={ckpt_games(best_checkpoint_path)})")

shutil.copy(best_checkpoint_path, os.path.join("checkpoints", "best.pth"))
print("copied -> checkpoints/best.pth")