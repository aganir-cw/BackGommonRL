import marimo

__generated_with = "0.23.8"
app = marimo.App(width="medium")


@app.cell
def _():
    import pandas as pd
    import marimo as mo
    import matplotlib.pyplot as plt
    import matplotlib.ticker as mticker

    return mticker, pd, plt


@app.cell
def _(pd):
    df = pd.read_csv("results/sweep.csv")
    df
    return (df,)


@app.cell
def _(df, mticker, plt):
    def plot_throughput_concurrency():    
        fig, ax = plt.subplots(figsize=(7, 4.5), dpi=512)
        timeouts = sorted(df["timeout_ms"].unique())
        colors = plt.cm.viridis([0.15, 0.5, 0.82])  # one distinct color per timeout
        for timeout, color in zip(timeouts, colors):
            sub = df.loc[df["timeout_ms"] == timeout].sort_values("concurrency")
            ax.plot(
                sub["concurrency"], sub["games_per_s"],
                marker="o", markersize=5, linewidth=2.2, color=color, zorder=3,
                label=f"{timeout:g} ms",
            )
        ax.set_xscale("log", base=2)
        ax.set_xticks(sorted(df["concurrency"].unique()))
        ax.xaxis.set_major_formatter(mticker.ScalarFormatter())  # 32, 128… not 2^5, 2^7
        ax.set_ylim(bottom=0)       
        ax.axvline(512, ls=":", color="#999")# honest saturation shape
    
        ax.set_xlabel("concurrency (simultaneous games)")
        ax.set_ylabel("throughput (games / s)")
        ax.set_title("Self-play throughput vs concurrency", fontsize=12, fontweight="bold")
        ax.spines[["top", "right"]].set_visible(False)
        ax.grid(True, linestyle="--", alpha=0.5, color="#cccccc", zorder=0)
        ax.legend(title="batch flush timeout", frameon=False)
        fig.tight_layout()
    
        peak = df.loc[df["games_per_s"].idxmax()]
        ax.scatter(
            peak["concurrency"], peak["games_per_s"],
            s=140, facecolor="none", edgecolor="#d1495b", linewidth=2, zorder=5,
        )
        ax.annotate(
            f"peak {peak['games_per_s']:.0f} games/s\n"
            f"c={int(peak['concurrency'])}, {peak['timeout_ms']:g} ms",
            xy=(peak["concurrency"], peak["games_per_s"]),
            xytext=(12, -28), textcoords="offset points",
            fontsize=9, color="#d1495b",
            arrowprops=dict(arrowstyle="->", color="#d1495b", lw=1.2),
        )
        return fig
    plot_throughput_concurrency()
    return


@app.cell
def _(df, mticker, plt):
    def plot_mean_achieved_batch_concurrency():    
        fig, ax = plt.subplots(figsize=(7, 4.5), dpi=512)
        timeouts = sorted(df["timeout_ms"].unique())
        colors = plt.cm.viridis([0.15, 0.5, 0.82])  # one distinct color per timeout
        for timeout, color in zip(timeouts, colors):
            sub = df.loc[df["timeout_ms"] == timeout].sort_values("concurrency")
            ax.plot(
                sub["concurrency"], sub["mean_batch"],
                marker="o", markersize=5, linewidth=2.2, color=color, zorder=3,
                label=f"{timeout:g} ms",
            )
        ax.set_xscale("log", base=2)
        ax.set_xticks(sorted(df["concurrency"].unique()))
        ax.xaxis.set_major_formatter(mticker.ScalarFormatter())  # 32, 128… not 2^5, 2^7
        ax.set_ylim(bottom=0)       
    
        ax.set_xlabel("concurrency (simultaneous games)")
        ax.set_ylabel("throughput (games / s)")
        ax.set_title("Self-play throughput vs concurrency", fontsize=12, fontweight="bold")
        ax.spines[["top", "right"]].set_visible(False)
        ax.grid(True, linestyle="--", alpha=0.5, color="#cccccc", zorder=0)
        ax.legend(title="batch flush timeout", frameon=False)
        fig.tight_layout()
        return fig

    plot_mean_achieved_batch_concurrency()
    return


@app.cell
def _():
    return


if __name__ == "__main__":
    app.run()
