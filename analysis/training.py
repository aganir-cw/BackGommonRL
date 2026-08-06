import marimo

__generated_with = "0.23.8"
app = marimo.App(width="medium")


@app.cell
def _():
    import pandas as pd
    import marimo as mo
    import matplotlib.pyplot as plt
    import matplotlib.ticker as mticker

    return pd, plt


@app.cell
def _(pd):
    df = pd.read_csv("analysis/results/train_log.csv")
    df
    return (df,)


@app.cell
def _(df, plt):
    def plot_loss():
        fig, ax = plt.subplots(figsize=(8, 6), dpi=512)
        steps = df["step"]
        loss = df["loss"]
        plt.plot(steps, loss, label="Loss at step")
        return fig

    plot_loss()
    return


@app.cell
def _(l):
    l
    return


if __name__ == "__main__":
    app.run()
