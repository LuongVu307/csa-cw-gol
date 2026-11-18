# Parallel benchmark results

Each operation is a fresh 100-turn simulation with in-memory I/O and event draining.
GOMAXPROCS=12; five samples per configuration. Runtime values below are medians.
The runtime plot shows the observed sample minimum and maximum, not confidence intervals.
Speedup uses each strategy's own one-worker median for the same board size.

## Fastest measured configurations

| Board | Strategy | Workers | Median ms | Sample range ms | Speedup |
|---|---|---:|---:|---:|---:|
| 128x128 | Heap allocation | 12 | 28.58 | 28.37–29.22 | 1.89x |
| 128x128 | Double buffer | 12 | 27.41 | 27.00–28.56 | 2.84x |
| 128x128 | Map-reduce | 12 | 27.58 | 27.30–28.84 | 2.71x |
| 512x512 | Heap allocation | 12 | 315.74 | 298.54–372.17 | 2.56x |
| 512x512 | Double buffer | 8 | 326.90 | 318.38–333.64 | 3.85x |
| 512x512 | Map-reduce | 12 | 307.36 | 306.33–351.14 | 4.46x |

## Allocation at 12 workers (same configuration across strategies)

| Board | Strategy | Median MiB allocated/op | Median allocations/op |
|---|---|---:|---:|
| 128x128 | Double buffer | 0.73 | 23,189 |
| 128x128 | Heap allocation | 3.31 | 40,559 |
| 128x128 | Map-reduce | 1.92 | 24,931 |
| 512x512 | Double buffer | 9.90 | 382,215 |
| 512x512 | Heap allocation | 38.42 | 437,604 |
| 512x512 | Map-reduce | 25.92 | 394,784 |

## Interpretation limits

- Results describe these supplied patterns over their first 100 turns on this WSL machine.
- The two sizes have different starting patterns; this is not an isolated board-size experiment.
- All strategies include channel setup, pixel transfers, event handling and final result construction; allocation values include the harness.
- Host load, laptop power/temperature, frequency changes, and benchmark order can affect results.
- Five samples describe observed variability; close medians do not establish a significant winner.
- Allocation differences do not prove garbage collection caused runtime differences without profiling.
- Highest observed runtime coefficient of variation: 26.3% (512, double, 12 workers).

See `summary.csv` for all 36 cases, `parallel-raw.txt` for raw output, and `environment.txt` for machine details.

## Targeted confirmation run

The original 512x512 double-buffer result at 12 workers was anomalous and variable.
We repeated double-buffer and map-reduce at 8, 12, and 16 workers in a separate run.
The original dataset and figures remain unchanged; this is not a replacement of slow samples.

| Strategy | Workers | Original median ms | Confirmation median ms | Confirmation range ms |
|---|---:|---:|---:|---:|
| Double buffer | 8 | 326.90 | 348.53 | 313.28–412.14 |
| Double buffer | 12 | 583.94 | 290.14 | 288.23–316.56 |
| Double buffer | 16 | 443.70 | 303.17 | 290.86–310.36 |
| Map-reduce | 8 | 310.29 | 316.83 | 302.40–346.64 |
| Map-reduce | 12 | 307.36 | 329.11 | 322.58–341.56 |
| Map-reduce | 16 | 317.34 | 344.26 | 319.72–346.87 |

Differences between runs limit claims of a stable performance ranking.
Host conditions and runtime scheduling were not instrumented, so their causes are not established.
