#!/usr/bin/env python3
"""Summarise repeated Go benchmarks and produce standalone pgfplots sources."""
import argparse
import csv
import re
import statistics
from collections import defaultdict
from pathlib import Path

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--directory', type=Path, default=Path(__file__).resolve().parent,
                    help='Read raw data and write summaries and plot sources in this directory.')
ROOT = parser.parse_args().directory.resolve()
PATTERN = re.compile(
    r'^BenchmarkParallel/size=(\d+)/strategy=(\w+)/workers=(\d+)-\d+\s+'
    r'(\d+)\s+([\d.]+) ns/op\s+(\d+) B/op\s+(\d+) allocs/op'
)
STRATEGIES = ('heap', 'double', 'mapreduce')
LABELS = {'heap': 'Heap allocation', 'double': 'Double buffer', 'mapreduce': 'Map-reduce'}
WORKERS = (1, 2, 4, 8, 12, 16)
samples = defaultdict(list)
for line in (ROOT / 'parallel-raw.txt').read_text().splitlines():
    match = PATTERN.match(line)
    if match:
        size, strategy, workers, iterations, ns, allocated, allocs = match.groups()
        samples[int(size), strategy, int(workers)].append(
            (float(ns), int(allocated), int(allocs), int(iterations)))
expected = {(size, strategy, workers) for size in (128, 512)
            for strategy in STRATEGIES for workers in WORKERS}
if set(samples) != expected or any(len(v) != 5 for v in samples.values()):
    raise SystemExit('Expected all 36 configurations with exactly five samples; collection is incomplete.')
rows = []
for size, strategy, workers in sorted(samples):
    values = samples[size, strategy, workers]
    times = [v[0] / 1e6 for v in values]
    median = statistics.median(times)
    baseline = statistics.median(v[0] / 1e6 for v in samples[size, strategy, 1])
    rows.append(dict(size=size, strategy=strategy, workers=workers, samples=len(values),
                     median_ms=median, min_ms=min(times), max_ms=max(times),
                     mean_ms=statistics.mean(times), stdev_ms=statistics.stdev(times),
                     cv_percent=100 * statistics.stdev(times) / statistics.mean(times),
                     speedup=baseline / median,
                     median_bytes=statistics.median(v[1] for v in values),
                     median_allocs=statistics.median(v[2] for v in values)))
with (ROOT / 'summary.csv').open('w', newline='') as f:
    writer = csv.DictWriter(f, fieldnames=list(rows[0]))
    writer.writeheader()
    writer.writerows(rows)

for metric, ylabel in [('runtime', 'Time per 100-turn simulation (ms)'),
                        ('speedup', 'Speedup (from one-worker baseline)'),
                        ('memory', 'Allocated memory per simulation (MiB)'),
                        ('allocations', 'Allocations per simulation (thousands)')]:
    tex = [r'\documentclass[tikz,border=4pt]{standalone}',
           r'\usepackage{pgfplots}', r'\pgfplotsset{compat=1.18}',
           r'\usepgfplotslibrary{groupplots}', r'\begin{document}',
           r'\begin{tikzpicture}',
           r'\begin{groupplot}[group style={group size=2 by 1,horizontal sep=20mm},',
           r'width=8cm,height=6cm,xlabel={Worker goroutines},',
           'ylabel={' + ylabel + '},',
           r'xtick={1,2,4,8,12,16},xmin=0.5,xmax=16.5,ymin=0,grid=major,',
           r'legend style={font=\small,at={(0.5,-0.28)},anchor=north}]']
    for size in (128, 512):
        tex.append(r'\nextgroupplot[title={' + f'{size} x {size}, 100 turns' + '}]')
        for strategy, color, mark in zip(STRATEGIES, ('blue', 'red', 'teal'), ('*', 'square*', 'triangle*')):
            selected = sorted((r for r in rows if r['size'] == size and r['strategy'] == strategy),
                              key=lambda r: r['workers'])
            coords = []
            for row in selected:
                value = {'runtime': row['median_ms'], 'speedup': row['speedup'],
                         'memory': row['median_bytes'] / 2**20,
                         'allocations': row['median_allocs'] / 1000}[metric]
                coord = f"({row['workers']},{value:.6f})"
                if metric == 'runtime':
                    coord += f" += (0,{row['max_ms'] - value:.6f}) -= (0,{value - row['min_ms']:.6f})"
                coords.append(coord)
            style = f'color={color},mark={mark},thick'
            if metric == 'runtime':
                style += ',error bars/.cd,y dir=both,y explicit'
            tex.append(r'\addplot+[' + style + '] coordinates {' + ' '.join(coords) + '};')
            tex.append(r'\addlegendentry{' + LABELS[strategy] + '}')
    tex += [r'\end{groupplot}', r'\end{tikzpicture}', r'\end{document}']
    (ROOT / f'{metric}.tex').write_text('\n'.join(tex) + '\n')

lines = ['# Parallel benchmark results', '',
         'Each operation is a fresh 100-turn simulation with in-memory I/O and event draining.',
         'GOMAXPROCS=12; five samples per configuration. Runtime values below are medians.',
         'The runtime plot shows the observed sample minimum and maximum, not confidence intervals.',
         'Speedup uses each strategy\'s own one-worker median for the same board size.', '',
         '## Fastest measured configurations', '',
         '| Board | Strategy | Workers | Median ms | Sample range ms | Speedup |',
         '|---|---|---:|---:|---:|---:|']
for size in (128, 512):
    for strategy in STRATEGIES:
        best = min((r for r in rows if r['size'] == size and r['strategy'] == strategy),
                   key=lambda r: r['median_ms'])
        lines.append(f"| {size}x{size} | {LABELS[strategy]} | {best['workers']} | {best['median_ms']:.2f} | {best['min_ms']:.2f}–{best['max_ms']:.2f} | {best['speedup']:.2f}x |")
lines += ['', '## Allocation at 12 workers (same configuration across strategies)', '',
          '| Board | Strategy | Median MiB allocated/op | Median allocations/op |',
          '|---|---|---:|---:|']
for row in rows:
    if row['workers'] == 12:
        lines.append(f"| {row['size']}x{row['size']} | {LABELS[row['strategy']]} | {row['median_bytes'] / 2**20:.2f} | {row['median_allocs']:,.0f} |")
noisiest = max(rows, key=lambda r: r['cv_percent'])
lines += ['', '## Interpretation limits', '',
          '- Results describe these supplied patterns over their first 100 turns on this WSL machine.',
          '- The two sizes have different starting patterns; this is not an isolated board-size experiment.',
          '- All strategies include channel setup, pixel transfers, event handling and final result construction; allocation values include the harness.',
          '- Host load, laptop power/temperature, frequency changes, and benchmark order can affect results.',
          '- Five samples describe observed variability; close medians do not establish a significant winner.',
          '- Allocation differences do not prove garbage collection caused runtime differences without profiling.',
          f"- Highest observed runtime coefficient of variation: {noisiest['cv_percent']:.1f}% ({noisiest['size']}, {noisiest['strategy']}, {noisiest['workers']} workers).",
          '', 'See `summary.csv` for all 36 cases, `parallel-raw.txt` for raw output, and `environment.txt` for machine details.']
confirmation_path = ROOT / 'confirmation-raw.txt'
if confirmation_path.exists():
    repeated = defaultdict(list)
    for line in confirmation_path.read_text().splitlines():
        match = PATTERN.match(line)
        if match:
            size, strategy, workers, iterations, ns, allocated, allocs = match.groups()
            repeated[int(size), strategy, int(workers)].append(float(ns) / 1e6)
    if len(repeated) != 6 or any(len(v) != 5 for v in repeated.values()):
        raise SystemExit('Confirmation measurement is incomplete; expected six cases with five samples each.')
    lines += ['', '## Targeted confirmation run', '',
              'To assess repeatability, a separate run repeats double buffer and map-reduce',
              'on 512x512 at 8, 12 and 16 workers, with five samples per configuration.',
              'Both datasets are retained; confirmation does not replace full-run samples.', '',
              '| Strategy | Workers | Full-run median ms | Confirmation median ms | Confirmation range ms |',
              '|---|---:|---:|---:|---:|']
    confirmation_rows = []
    for (size, strategy, workers), values in sorted(repeated.items()):
        original = next(r for r in rows if r['size'] == size and r['strategy'] == strategy and r['workers'] == workers)
        median = statistics.median(values)
        lines.append(f"| {LABELS[strategy]} | {workers} | {original['median_ms']:.2f} | {median:.2f} | {min(values):.2f}–{max(values):.2f} |")
        confirmation_rows.append(dict(size=size, strategy=strategy, workers=workers,
                                      median_ms=median, min_ms=min(values), max_ms=max(values)))
    with (ROOT / 'confirmation-summary.csv').open('w', newline='') as f:
        writer = csv.DictWriter(f, fieldnames=list(confirmation_rows[0]))
        writer.writeheader()
        writer.writerows(confirmation_rows)
    tex = [r'\documentclass[tikz,border=4pt]{standalone}',
           r'\usepackage{pgfplots}', r'\pgfplotsset{compat=1.18}',
           r'\usepgfplotslibrary{groupplots}', r'\begin{document}',
           r'\begin{tikzpicture}',
           r'\begin{groupplot}[group style={group size=2 by 1,horizontal sep=20mm},',
           r'width=8cm,height=6cm,xlabel={Worker goroutines},',
           r'ylabel={Time per 100-turn simulation (ms)},',
           r'xtick={8,12,16},xmin=7.5,xmax=16.5,ymin=0,ymax=650,grid=major,',
           r'legend style={font=\small,at={(0.5,-0.28)},anchor=north}]']
    for title, dataset in [('Full run', rows), ('Confirmation run', confirmation_rows)]:
        tex.append(r'\nextgroupplot[title={' + title + ', 512 x 512}]')
        for strategy, color, mark in [('double', 'red', 'square*'), ('mapreduce', 'teal', 'triangle*')]:
            selected = sorted((r for r in dataset if r['size'] == 512 and r['strategy'] == strategy and r['workers'] in (8, 12, 16)), key=lambda r: r['workers'])
            coords = []
            for row in selected:
                value = row['median_ms']
                coords.append(f"({row['workers']},{value:.6f}) += (0,{row['max_ms']-value:.6f}) -= (0,{value-row['min_ms']:.6f})")
            tex.append(r'\addplot+[color=' + color + ',mark=' + mark + r',thick,error bars/.cd,y dir=both,y explicit] coordinates {' + ' '.join(coords) + '};')
            tex.append(r'\addlegendentry{' + LABELS[strategy] + '}')
    tex += [r'\end{groupplot}', r'\end{tikzpicture}', r'\end{document}']
    (ROOT / 'confirmation.tex').write_text('\n'.join(tex) + '\n')
    lines += ['', 'Differences between runs limit claims of a stable performance ranking.',
              'Host conditions and runtime scheduling were not instrumented, so their causes are not established.']
(ROOT / 'results.md').write_text('\n'.join(lines) + '\n')
print(f'Summarised {sum(map(len, samples.values()))} samples across {len(rows)} configurations.')
