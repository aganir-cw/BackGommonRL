import marimo

__generated_with = "0.23.8"
app = marimo.App(width="medium")


@app.cell
def _():
    import pandas as pd
    import marimo as mo
    import matplotlib.pyplot as plt
    import matplotlib.ticker as mticker

    import numpy as np

    return pd, plt


@app.cell
def _(pd, plt):
    def plot_loss():
        df = pd.read_csv("analysis/results/train_log.csv")
        df
        fig, ax = plt.subplots(figsize=(8, 6), dpi=512)
        steps = df["step"]
        loss = df["loss"]
        plt.plot(steps, loss, label="Loss at step")
        return fig

    plot_loss()
    return


@app.cell
def _(pd, plt):
    df = pd.read_csv("analysis/results/eval_log.csv")
    fig, ax = plt.subplots(figsize=(8, 6))
    for opp, g in df.groupby("opponent"):
        ax.errorbar(g["games_seen"], g["winrate"], yerr=g["ci"],
                    marker="o", capsize=3, label=f"vs {opp}")
    ax.axhline(0.5, ls="--", c="gray"); ax.axhline(0.95, ls=":", c="green")
    ax.set_xlabel("games trained on"); ax.set_ylabel("winrate (greedy = A)")
    ax.legend();

    fig
    return


@app.cell
def _():
    return


if __name__ == "__main__":
    app.run()
