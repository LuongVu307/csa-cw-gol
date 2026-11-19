#!/usr/bin/env python3
"""Summarize three samples per worker count; emit CSV and a pgfplots figure."""
import argparse
import csv
import math
from pathlib import Path
from statistics import median

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('inputs', nargs='+', type=Path)
parser.add_argument('--output', required=True, type=Path, help='New directory for derived files')
args = parser.parse_args()
rows = []
for path in args.inputs:
    with path.open(newline='') as stream:
        rows.extend(csv.DictReader(stream))
if not rows or any(row['validation'] != 'PASS' for row in rows):
    parser.error('all input rows must have passed correctness validation')
settings = {(r['setup'], r['width'], r['height'], r['turns'], r['go_version']) for r in rows}
if len(settings) != 1:
    parser.error('use one setup, board size, turn count and Go version per comparison')
groups = {}
for row in rows:
    value = float(row['milliseconds'])
    if not math.isfinite(value) or value <= 0:
        parser.error('timings must be finite and positive')
    groups.setdefault(int(row['workers']), []).append(value)
if min(groups) < 2 or len(groups) < 2 or any(len(v) != 3 for v in groups.values()):
    parser.error('provide exactly three samples for each of at least two worker counts, all >= 2')
baseline = min(groups)
base_time = median(groups[baseline])
summary = [(n, median(v), min(v), max(v), base_time / median(v)) for n, v in sorted(groups.items())]
args.output.mkdir(parents=True, exist_ok=False)
with (args.output / 'summary.csv').open('w', newline='') as stream:
    writer = csv.writer(stream, lineterminator="\n")
    writer.writerow(['workers', 'median_ms', 'min_ms', 'max_ms', f'speedup_vs_{baseline}_workers'])
    writer.writerows(summary)
coords = '\n'.join(f'({n},{m:.3f}) += (0,{hi-m:.3f}) -= (0,{m-lo:.3f})' for n, m, lo, hi, _ in summary)
tex = r'''\documentclass[tikz,border=4pt]{standalone}
\usepackage{pgfplots}
\pgfplotsset{compat=1.18}
\begin{document}
\begin{tikzpicture}
\begin{axis}[width=10cm,height=6cm,xlabel={Remote worker processes},
ylabel={Runtime (ms)},xtick=data,ymin=0,grid=major]
\addplot+[mark=*,error bars/.cd,y dir=both,y explicit] coordinates {
''' + coords + r'''
};
\end{axis}
\end{tikzpicture}
\end{document}
'''
(args.output / 'runtime.tex').write_text(tex)
print(f'Wrote {args.output}/summary.csv and runtime.tex; baseline is {baseline} workers.')
print('Ranges are sample minima/maxima, not confidence intervals.')
