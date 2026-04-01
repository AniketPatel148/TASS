#!/usr/bin/env python3
"""
TASS Results Plotter
Generates charts from simulation output files.

Usage:
  python scripts/plot_results.py --dir out/
  python scripts/plot_results.py --dir out/ --compare   # Compare multiple policies
"""

import argparse
import csv
import json
import os
import sys

def load_requests_csv(path):
    """Load per-request CSV into a list of dicts."""
    rows = []
    with open(path) as f:
        reader = csv.DictReader(f)
        for row in reader:
            rows.append({
                'request_id': int(row['request_id']),
                'tier': row['tier'],
                'context_tokens': int(row['context_tokens']),
                'output_tokens': int(row['output_tokens']),
                'arrival_ms': float(row['arrival_ms']),
                'queue_delay_ms': float(row['queue_delay_ms']),
                'ttft_ms': float(row['ttft_ms']),
                'total_latency_ms': float(row['total_latency_ms']),
                'tokens_per_sec': float(row['tokens_per_sec']),
            })
    return rows

def load_summary(path):
    """Load run summary JSON."""
    with open(path) as f:
        return json.load(f)

def try_import_matplotlib():
    """Try to import matplotlib, return None if not available."""
    try:
        import matplotlib
        matplotlib.use('Agg')
        import matplotlib.pyplot as plt
        return plt
    except ImportError:
        return None

def plot_latency_distribution(requests, output_path, plt):
    """Plot latency distribution by tier."""
    tiers = sorted(set(r['tier'] for r in requests))
    fig, axes = plt.subplots(1, len(tiers), figsize=(5 * len(tiers), 4), squeeze=False)

    for i, tier in enumerate(tiers):
        tier_lats = [r['total_latency_ms'] for r in requests if r['tier'] == tier]
        axes[0][i].hist(tier_lats, bins=30, alpha=0.7, edgecolor='black')
        axes[0][i].set_title(f'{tier} tier')
        axes[0][i].set_xlabel('Total Latency (ms)')
        axes[0][i].set_ylabel('Count')

    plt.tight_layout()
    plt.savefig(output_path, dpi=150)
    plt.close()
    print(f"  Saved: {output_path}")

def plot_ttft_by_tier(requests, output_path, plt):
    """Plot TTFT distribution by tier."""
    tiers = sorted(set(r['tier'] for r in requests))
    data = [[r['ttft_ms'] for r in requests if r['tier'] == t] for t in tiers]

    fig, ax = plt.subplots(figsize=(8, 5))
    ax.boxplot(data, labels=tiers)
    ax.set_ylabel('Time to First Token (ms)')
    ax.set_title('TTFT by Tier')
    ax.grid(axis='y', alpha=0.3)

    plt.tight_layout()
    plt.savefig(output_path, dpi=150)
    plt.close()
    print(f"  Saved: {output_path}")

def plot_comparison(summaries, output_path, plt):
    """Plot comparison bar chart across policies."""
    policies = [s['SchedulerName'] for s in summaries]
    throughputs = [s['ThroughputTokSec'] for s in summaries]
    p95s = [s['OverallP95Ms'] for s in summaries]
    
    # Calculate average SLA violation across tiers
    sla_viols = []
    for s in summaries:
        if 'TierMetrics' in s and s['TierMetrics']:
            viols = [tm['SLATotalViol'] * 100 for tm in s['TierMetrics']]
            sla_viols.append(sum(viols) / len(viols))
        else:
            sla_viols.append(0)

    fig, (ax1, ax2, ax3) = plt.subplots(1, 3, figsize=(16, 5))

    ax1.bar(policies, throughputs, color='steelblue', edgecolor='black')
    ax1.set_ylabel('Throughput (tok/s)')
    ax1.set_title('Throughput by Policy')
    ax1.tick_params(axis='x', rotation=30)

    ax2.bar(policies, p95s, color='coral', edgecolor='black')
    ax2.set_ylabel('P95 Latency (ms)')
    ax2.set_title('P95 Latency by Policy')
    ax2.tick_params(axis='x', rotation=30)
    
    ax3.bar(policies, sla_viols, color='crimson', edgecolor='black')
    ax3.set_ylabel('Avg SLA Violation (%)')
    ax3.set_title('SLA Violation by Policy')
    ax3.tick_params(axis='x', rotation=30)

    plt.tight_layout()
    plt.savefig(output_path, dpi=150)
    plt.close()
    print(f"  Saved: {output_path}")

def print_text_summary(summary):
    """Print summary as text table (no matplotlib needed)."""
    print(f"\n{'='*60}")
    print(f"  Scheduler: {summary['SchedulerName']}")
    print(f"  Requests:  {summary['TotalRequests']}")
    print(f"  Tokens:    {summary['TotalTokensGen']}")
    print(f"  Throughput: {summary['ThroughputTokSec']:.1f} tok/s")
    print(f"  Latency P50: {summary['OverallP50Ms']:.1f}ms  P95: {summary['OverallP95Ms']:.1f}ms  P99: {summary['OverallP99Ms']:.1f}ms")
    print(f"  Fairness:  {summary['FairnessIndex']:.3f}")
    print(f"{'='*60}")

    if 'TierMetrics' in summary and summary['TierMetrics']:
        print(f"  {'Tier':<12} {'Count':>5}  {'TTFT P95':>10}  {'Lat P95':>10}  {'TTFT%V':>7}  {'Lat%V':>7}")
        for tm in summary['TierMetrics']:
            print(f"  {tm['Tier']:<12} {tm['Count']:>5}  {tm['P95TTFTMs']:>9.1f}ms  {tm['P95LatencyMs']:>9.1f}ms  {tm['SLATTFTViol']*100:>6.1f}%  {tm['SLATotalViol']*100:>6.1f}%")
    print()

def main():
    parser = argparse.ArgumentParser(description='Plot TASS simulation results')
    parser.add_argument('--dir', required=True, help='Output directory from simulation')
    parser.add_argument('--compare', action='store_true', help='Compare multiple policies (expects subdirs)')
    args = parser.parse_args()

    plt = try_import_matplotlib()

    if args.compare:
        # Look for policy subdirectories
        summaries = []
        for name in sorted(os.listdir(args.dir)):
            summary_path = os.path.join(args.dir, name, 'summary.json')
            if os.path.isfile(summary_path):
                s = load_summary(summary_path)
                summaries.append(s)
                print_text_summary(s)

        if plt and summaries:
            plot_comparison(summaries, os.path.join(args.dir, 'comparison.png'), plt)
        elif not plt:
            print("matplotlib not installed — skipping charts. Install: pip install matplotlib")
    else:
        summary_path = os.path.join(args.dir, 'summary.json')
        if os.path.isfile(summary_path):
            s = load_summary(summary_path)
            print_text_summary(s)

        requests_path = os.path.join(args.dir, 'requests.csv')
        if plt and os.path.isfile(requests_path):
            requests = load_requests_csv(requests_path)
            plot_latency_distribution(requests, os.path.join(args.dir, 'latency_dist.png'), plt)
            plot_ttft_by_tier(requests, os.path.join(args.dir, 'ttft_by_tier.png'), plt)
        elif not plt:
            print("matplotlib not installed — skipping charts. Install: pip install matplotlib")

if __name__ == '__main__':
    main()
