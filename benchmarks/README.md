# Parallel benchmarks

The files at this level describe the latest full run (18 November 2026).
The report reads `runtime.pdf`, `speedup.pdf`, `memory.pdf` and `allocations.pdf`
from here. `confirmation.pdf` is the separate repeat comparison.

- `parallel-raw.txt`: 36 configurations, five samples each (180 samples).
- `confirmation-raw.txt`: double buffer and map-reduce at 8, 12 and 16 workers
  on 512×512, five samples each (30 samples).
- `summary.csv`, `confirmation-summary.csv`, `results.md`: derived results.
- `environment.txt`: recorded machine, settings and source/input hashes.
- `*.tex`, `*.pdf`, `*.png`: plot sources, vector figures and preview images.
- `validation/`: earlier correctness and race-check records, separate from timing.
- `archive/`: earlier measurements and figures; retained for transparency.

## What is measured

Independent variables: strategy (heap allocation, double buffer, map-reduce),
workers (1, 2, 4, 8, 12, 16), and board size (128, 512).
Each operation starts the supplied image afresh and simulates 100 turns.
Go repeats operations to reach roughly one second per sample; `-count=5`
collects five samples per case. Each sample reports the average time per
100-turn operation; graphs show the median of those five sample averages; runtime bars show sample
minimum and maximum, not confidence intervals. Speedup uses each strategy's
own one-worker median at the same size.

The harness in `gol/parallel_benchmark_test.go` calls the local distributors
with in-memory I/O and event draining. Timing includes setup, board allocation,
pixel transfers, all turns/events and final alive-list construction. It excludes
input-file reading, actual disk output, SDL rendering and reference validation.
B/op is cumulative allocated bytes, not peak RAM; harness allocations are included.
The two sizes use different patterns, and one-worker execution paths differ
between strategies. These comparisons do not isolate size or memory policy alone.

## Run benchmarks

Run all commands from the repository root (`csa-cw-gol`). Requirements: Go 1.25
or newer (the implementation uses `sync.WaitGroup.Go`), Python 3, and, for
figures, `pdflatex` with pgfplots and `pdftoppm`.
Keep the machine otherwise idle and use the same power settings for all cases.
Run correctness/race checks separately from timing:

```bash
GOCACHE=/tmp/cw1-go-cache GOMAXPROCS=12 go test ./gol -run '^TestParallelBenchmarkCorrectness$' -count=1 -timeout=5m
GOCACHE=/tmp/cw1-go-cache GOMAXPROCS=12 go test ./gol -run '^TestParallelBenchmarkCorrectness$' -race -count=1 -timeout=5m
```

These checks compare final boards with an independent reference. They do not
replace the course correctness/keyboard/SDL suite for each strategy.

Create a fresh directory so previous measurements are preserved. Choose a new
name for each run. The block stops if the directory already exists or a
command fails, preventing an accidental overwrite:

```bash
benchmark_run=benchmarks/runs/new-run
(
set -e
mkdir -p benchmarks/runs
mkdir "$benchmark_run"
{
  date -u
  go version
  uname -srmo
  lscpu
  uptime
  git rev-parse HEAD
  git status --short
  sha256sum gol/parallel_benchmark_test.go gol/distributor.go gol/gol.go images/128x128.pgm images/512x512.pgm
  echo 'GOMAXPROCS=12; 100 turns/op; 5 samples/case; benchtime=1s; normal GC'
} > "$benchmark_run/environment.txt"
GOCACHE=/tmp/cw1-go-cache GOMAXPROCS=12 go test ./gol -run '^$' -bench '^BenchmarkParallel$' -benchmem -benchtime=1s -count=5 -timeout=20m > "$benchmark_run/parallel-raw.txt"
GOCACHE=/tmp/cw1-go-cache GOMAXPROCS=12 go test ./gol -run '^$' -bench '^BenchmarkParallel/size=512/strategy=(double|mapreduce)/workers=(8|12|16)$' -benchmem -benchtime=1s -count=5 -timeout=10m > "$benchmark_run/confirmation-raw.txt"
python3 benchmarks/analyze.py --directory "$benchmark_run"
)
```

Inspect both raw files for `PASS`. The analyzer requires all 180 full-run samples
and, when a confirmation file exists, all 30 confirmation samples. It writes
summaries and plot sources into the selected directory. Do not run benchmark
cases concurrently or use race-instrumented measurements for performance graphs.

## Generate figures

In the same shell, after the commands above:

```bash
(
  cd "$benchmark_run" || exit
  for figure in runtime speedup memory allocations confirmation; do
    pdflatex -interaction=nonstopmode -halt-on-error "$figure.tex" || exit
    pdftoppm -singlefile -r 150 -png "$figure.pdf" "$figure" || exit
  done
)
```

To regenerate summaries and plot sources for the current top-level dataset,
use `python3 benchmarks/analyze.py`; use `benchmark_run=benchmarks` in the
figure commands to render that dataset. New runs stay separate until deliberately
selected for the report. Update report numbers whenever selecting new figures.

## Interpretation

Five samples describe observed variation; close medians do not prove a winner.
Fixed execution order, WSL host load, power and temperature can affect results.
Allocation differences do not establish garbage-collection costs without profiles.
Keep anomalous runs and report the distinction between observations and explanations.
