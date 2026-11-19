# Go distributed evaluation

Run this only after correctness tests finish. It connects to the existing Go
broker and does not edit workerURLs.txt, start/stop servers, or write to out/.
Do not run another controller/test against the same broker during measurement.
Go 1.25+ is required by the existing simulation code.

## Automatic localhost run (recommended for a quick comparison)

After stopping correctness tests, run this from `csa-cw-gol` in your normal terminal:

```bash
python3 benchmarks/distributed/run.py
```

The script builds the current code in all three sibling repositories, starts
its own worker and broker processes on available localhost ports, measures
2 and 4 workers with three samples each, validates results, and writes a new
timestamped results directory. It generates `summary/summary.csv` and, when
pdflatex is available, `summary/runtime.pdf`. It stops only its own servers.
Your existing brokerURL.txt, workerURLs.txt and out/ are untouched.

Every process uses GOMAXPROCS=1; topology, source hashes and machine details
are recorded in environment.txt. Each worker still calculates its strip
serially, while separate worker processes can execute concurrently. This
measures localhost process scaling, not performance across AWS nodes. Keep
other programs idle. A failed run retains its logs and is not used for the
summary. The automatic run has a three-minute timeout per configuration.

## Manual setup (for existing servers or AWS)

Start the actual Go worker services and broker using their normal commands.
For each configuration, put exactly 2, 4, then 8 distinct worker addresses in
the broker's workerURLs.txt. Each address should belong to a separate process.
The broker rereads that file for each simulation. The controller's Threads
parameter does not determine the number of remote workers.

Use only worker counts available in your setup; 2 and 4 is a useful minimal
comparison. Do not include one worker: the current broker calculates locally
for that special case. Record the worker/broker source revisions, host CPUs,
Go versions, process placement, GOMAXPROCS settings and network type alongside
the data. Multiple processes on localhost do not measure multi-machine scaling.

## Measure

From csa-cw-gol, build once, outside the measured runs:

```bash
go build -o /tmp/cw1-distributed-benchmark ./benchmarks/distributed
mkdir -p benchmarks/distributed/runs
```

After configuring the broker with two workers, run:

```bash
/tmp/cw1-distributed-benchmark -workers 2 -count 3 \
  -setup 'localhost; REPLACE with actual CPU, placement and GOMAXPROCS settings' \
  -output benchmarks/distributed/runs/workers-2.csv
```

Repeat with 4 and 8 workers, changing both the broker configuration and the
`-workers`/`-output` arguments. Use the same accurate setup description for
all runs. `-workers` records a label; it cannot verify the remote configuration.
Output CSV files must not already exist. Default broker address comes from
brokerURL.txt; override it with `-broker host:port` when needed.

Each sample restarts the supplied 512x512 image and runs 100 turns. A CSV row
is written only if the final board and replayed turn stream both match
check/images/512x512x100.pgm, with exactly 100 ordered turn batches.
Errors abort the run; incomplete files must not be treated as complete datasets.
Use another reference fixture if changing the turn count or input image.

Timing starts before connecting to the broker and ends after receiving the
final result. It includes connection establishment, input transfer, broker
worker connections, all calculation/reduction, turn RPCs, decoding/storing
change batches and final-board transfer. It excludes process startup, fixture
reading, final reference validation, SDL and PGM output. It is a protocol-level
measurement and is not directly comparable to the existing local benchmark.
There are no RPC deadlines in this runner; stop a hung run and retain its
partial CSV/log as failed evidence rather than recording a successful time.

## Summarize and plot

After all runs pass, from csa-cw-gol:

```bash
python3 benchmarks/distributed/analyze.py \
  benchmarks/distributed/runs/workers-2.csv \
  benchmarks/distributed/runs/workers-4.csv \
  benchmarks/distributed/runs/workers-8.csv \
  --output benchmarks/distributed/runs/summary
cd benchmarks/distributed/runs/summary
pdflatex -interaction=nonstopmode -halt-on-error runtime.tex
```

Omit the 8-worker input if only two configurations were measured. The analyzer
requires exactly three samples per count and a consistent setup/workload.
It reports medians, minima, maxima, and relative speedup T(2)/T(n) when the
smallest measured count is two. This is relative to two workers, not a serial
or one-worker baseline. Copy verified numbers and runtime.pdf into the report
only after inspecting the data. Three repeats describe observed variation;
they do not establish statistically significant differences.
