#!/usr/bin/env python3
"""Generate summary.md from a Mycelium benchmark run directory."""

import json
import glob
import os
import sys

run_dir = sys.argv[1]

# Load metadata
with open(os.path.join(run_dir, "prompts.json")) as f:
    meta = json.load(f)

models = meta["models"]
labels = [p["label"] for p in meta["prompts"]]
target = meta["target"]

# Collect all results (skip prompts.json)
results = {}
for f in sorted(glob.glob(os.path.join(run_dir, "*.json"))):
    name = os.path.basename(f).replace(".json", "")
    if name == "prompts":
        continue
    try:
        with open(f) as fh:
            results[name] = json.load(fh)
    except Exception:
        results[name] = {}


def get(d, *keys):
    for k in keys:
        if isinstance(d, dict):
            d = d.get(k, {})
        else:
            return "N/A"
    return d if d != {} else "N/A"


def fmt_cost(v):
    try:
        return f"${float(v):.4f}"
    except Exception:
        return "N/A"


def fmt_time(v):
    try:
        return f"{float(v)/1000:.1f}s"
    except Exception:
        return "N/A"


def fmt_tokens(d):
    try:
        usage = d.get("usage", {})
        inp = usage.get("input_tokens", 0)
        cache_read = usage.get("cache_read_input_tokens", 0)
        cache_create = usage.get("cache_creation_input_tokens", 0)
        out = usage.get("output_tokens", 0)
        total = inp + cache_read + cache_create + out
        return f"{total:,}"
    except Exception:
        return "N/A"


lines = []
lines.append("# Mycelium MCP Benchmark Results")
lines.append("")
lines.append(f"**Date:** {os.path.basename(run_dir)}")
lines.append(f"**Target:** {target}")
lines.append("")

for model in models:
    lines.append(f"## {model.title()}")
    lines.append("")
    lines.append(
        "| # | Prompt | MCP Tokens | No-MCP Tokens | MCP Cost | No-MCP Cost | MCP Time | No-MCP Time | MCP Turns | No-MCP Turns |"
    )
    lines.append(
        "|---|--------|-----------|--------------|----------|------------|----------|------------|-----------|-------------|"
    )

    total_mcp_cost = 0
    total_nomcp_cost = 0

    for idx, label in enumerate(labels):
        num = f"{idx+1:02d}"
        key_mcp = f"{num}-{label}-{model}-with-mcp"
        key_nomcp = f"{num}-{label}-{model}-without-mcp"

        d_mcp = results.get(key_mcp, {})
        d_nomcp = results.get(key_nomcp, {})

        mcp_cost = get(d_mcp, "total_cost_usd")
        nomcp_cost = get(d_nomcp, "total_cost_usd")

        try:
            total_mcp_cost += float(mcp_cost)
        except Exception:
            pass
        try:
            total_nomcp_cost += float(nomcp_cost)
        except Exception:
            pass

        lines.append(
            f"| {idx+1} | {label} "
            f"| {fmt_tokens(d_mcp)} | {fmt_tokens(d_nomcp)} "
            f"| {fmt_cost(mcp_cost)} | {fmt_cost(nomcp_cost)} "
            f'| {fmt_time(get(d_mcp, "duration_ms"))} | {fmt_time(get(d_nomcp, "duration_ms"))} '
            f'| {get(d_mcp, "num_turns")} | {get(d_nomcp, "num_turns")} |'
        )

    lines.append("")
    lines.append(
        f"**Total cost:** MCP = ${total_mcp_cost:.4f}, No-MCP = ${total_nomcp_cost:.4f}"
    )
    lines.append("")

lines.append("---")
lines.append("")
lines.append("## Answer Comparison")
lines.append("")

for model in models:
    for idx, label in enumerate(labels):
        num = f"{idx+1:02d}"
        lines.append(f"### {model.title()} - {label}")
        lines.append("")

        for mode in ["with-mcp", "without-mcp"]:
            txt_file = os.path.join(run_dir, f"{num}-{label}-{model}-{mode}.txt")
            try:
                with open(txt_file) as fh:
                    answer = fh.read().strip()
            except Exception:
                answer = "N/A"

            tag = "WITH MCP" if mode == "with-mcp" else "WITHOUT MCP"
            lines.append(f"<details><summary>{tag}</summary>")
            lines.append("")
            lines.append(answer)
            lines.append("")
            lines.append("</details>")
            lines.append("")

out_path = os.path.join(run_dir, "summary.md")
with open(out_path, "w") as f:
    f.write("\n".join(lines))

print(f"Summary written to {out_path}")

# --- Generate comparison charts ---
try:
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    import numpy as np

    # Collect averages per model
    metrics = {}  # {model: {mode: {metric: [values]}}}
    for model in models:
        metrics[model] = {"with-mcp": {"cost": [], "time": [], "turns": [], "tokens": []},
                          "without-mcp": {"cost": [], "time": [], "turns": [], "tokens": []}}
        for idx, label in enumerate(labels):
            num = f"{idx+1:02d}"
            for mode in ["with-mcp", "without-mcp"]:
                key = f"{num}-{label}-{model}-{mode}"
                d = results.get(key, {})
                try:
                    metrics[model][mode]["cost"].append(float(get(d, "total_cost_usd")))
                except Exception:
                    pass
                try:
                    metrics[model][mode]["time"].append(float(get(d, "duration_ms")) / 1000)
                except Exception:
                    pass
                try:
                    metrics[model][mode]["turns"].append(int(get(d, "num_turns")))
                except Exception:
                    pass
                try:
                    usage = d.get("usage", {})
                    tokens = (usage.get("input_tokens", 0) +
                              usage.get("cache_read_input_tokens", 0) +
                              usage.get("cache_creation_input_tokens", 0) +
                              usage.get("output_tokens", 0))
                    metrics[model][mode]["tokens"].append(tokens)
                except Exception:
                    pass

    def safe_avg(vals):
        return sum(vals) / len(vals) if vals else 0

    fig, axes = plt.subplots(2, 2, figsize=(12, 8))
    fig.suptitle("Mycelium MCP Benchmark", fontsize=16, fontweight="bold", y=0.98)

    chart_configs = [
        ("Cost ($)", "cost", axes[0, 0]),
        ("Time (s)", "time", axes[0, 1]),
        ("Turns", "turns", axes[1, 0]),
        ("Tokens", "tokens", axes[1, 1]),
    ]

    colors_mcp = "#4CAF50"
    colors_nomcp = "#FF5722"

    x = np.arange(len(models))
    width = 0.3

    for title, metric, ax in chart_configs:
        mcp_vals = [safe_avg(metrics[m]["with-mcp"][metric]) for m in models]
        nomcp_vals = [safe_avg(metrics[m]["without-mcp"][metric]) for m in models]

        bars1 = ax.bar(x - width/2, mcp_vals, width, label="With MCP", color=colors_mcp, edgecolor="white")
        bars2 = ax.bar(x + width/2, nomcp_vals, width, label="Without MCP", color=colors_nomcp, edgecolor="white")

        # Value labels on bars
        for bar in bars1:
            h = bar.get_height()
            if h > 0:
                label = f"${h:.2f}" if metric == "cost" else (f"{h:.1f}" if metric == "time" else f"{h:,.0f}")
                ax.text(bar.get_x() + bar.get_width()/2, h, label,
                        ha="center", va="bottom", fontsize=9, fontweight="bold")
        for bar in bars2:
            h = bar.get_height()
            if h > 0:
                label = f"${h:.2f}" if metric == "cost" else (f"{h:.1f}" if metric == "time" else f"{h:,.0f}")
                ax.text(bar.get_x() + bar.get_width()/2, h, label,
                        ha="center", va="bottom", fontsize=9, fontweight="bold")

        ax.set_title(title, fontsize=13, fontweight="bold")
        ax.set_xticks(x)
        ax.set_xticklabels([m.title() for m in models])
        ax.legend(fontsize=9)
        ax.spines["top"].set_visible(False)
        ax.spines["right"].set_visible(False)
        ax.set_ylim(bottom=0)

    plt.tight_layout(rect=[0, 0, 1, 0.95])
    chart_path = os.path.join(run_dir, "chart.png")
    plt.savefig(chart_path, dpi=150, bbox_inches="tight")
    plt.close()
    print(f"Chart written to {chart_path}")

except ImportError:
    print("matplotlib not installed — skipping chart generation (pip install matplotlib)")
