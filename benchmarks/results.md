# Parallel benchmark results

Each operation is a fresh 100-turn simulation with in-memory I/O and event draining.
GOMAXPROCS=12; five samples per configuration. Runtime values below are medians.
The runtime plot shows the observed sample minimum and maximum, not confidence intervals.
Speedup uses each strategy's own one-worker median for the same board size.

## Fastest measured configurations

| Board | Strategy | Workers | Median ms | Sample range ms | Speedup |
|---|---|---:|---:|---:|---:|
| 128x128 | Heap allocation | 12 | 28.54 | 28.35–29.26 | 1.83x |
| 128x128 | Double buffer | 12 | 27.45 | 27.05–27.86 | 2.71x |
| 128x128 | Map-reduce | 12 | 27.79 | 27.71–28.54 | 2.56x |
| 512x512 | Heap allocation | 16 | 302.70 | 299.87–307.30 | 2.61x |
| 512x512 | Double buffer | 12 | 306.95 | 304.55–409.39 | 3.84x |
| 512x512 | Map-reduce | 16 | 333.71 | 325.17–334.65 | 3.37x |

## Allocation at 12 workers (same configuration across strategies)

| Board | Strategy | Median MiB allocated/op | Median allocations/op |
|---|---|---:|---:|
| 128x128 | Double buffer | 0.73 | 23,189 |
| 128x128 | Heap allocation | 3.31 | 40,559 |
| 128x128 | Map-reduce | 1.92 | 24,931 |
| 512x512 | Double buffer | 9.90 | 382,214 |
| 512x512 | Heap allocation | 38.42 | 437,601 |
| 512x512 | Map-reduce | 25.92 | 394,785 |

## Interpretation limits

- Results describe these supplied patterns over their first 100 turns on this WSL machine.
- The two sizes have different starting patterns; this is not an isolated board-size experiment.
- All strategies include channel setup, pixel transfers, event handling and final result construction; allocation values include the harness.
- Host load, laptop power/temperature, frequency changes, and benchmark order can affect results.
- Five samples describe observed variability; close medians do not establish a significant winner.
- Allocation differences do not prove garbage collection caused runtime differences without profiling.
- Highest observed runtime coefficient of variation: 13.9% (512, double, 12 workers).

See `summary.csv` for all 36 cases, `parallel-raw.txt` for raw output, and `environment.txt` for machine details.

## Targeted confirmation run

To assess repeatability, a separate run repeats double buffer and map-reduce
on 512x512 at 8, 12 and 16 workers, with five samples per configuration.
Both datasets are retained; confirmation does not replace full-run samples.

| Strategy | Workers | Full-run median ms | Confirmation median ms | Confirmation range ms |
|---|---:|---:|---:|---:|
| Double buffer | 8 | 336.71 | 369.56 | 319.88–390.35 |
| Double buffer | 12 | 306.95 | 335.39 | 334.56–349.16 |
| Double buffer | 16 | 320.10 | 333.35 | 304.79–339.17 |
| Map-reduce | 8 | 344.57 | 337.99 | 331.93–341.30 |
| Map-reduce | 12 | 342.00 | 383.69 | 353.38–394.09 |
| Map-reduce | 16 | 333.71 | 397.18 | 376.56–440.83 |

Differences between runs limit claims of a stable performance ranking.
Host conditions and runtime scheduling were not instrumented, so their causes are not established.
