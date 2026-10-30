# Bench results

Produced by `mpx bench startup|hook|render --out evidence/bench/` (docs/03-performance.md "How we measure"). Each JSON file records the machine, OS, CPU count and Go version.

Current files: Linux cloud VM (4 vCPU), release build (`-trimpath -ldflags "-s -w"`), Go 1.26.10, no terminal attached. The author's Windows laptop results are still to be recorded.

| Metric | Budget | Result (p50) |
|---|---|---|
| TUI first frame, cold (daemon not running) | < 150 ms | 36 ms |
| TUI first frame, attach to running daemon | < 50 ms | 3.8 ms |
| Hook round trip, daemon + socket | | 0.18 ms |
| Full `mpx-hook pre-tool` process | < 10 ms (Linux) | 5.8 ms (p99 8.4 ms) |
| Render: frame cost while streaming 10 MiB | 16 ms | 0.13 ms (p99 0.45 ms) |
| Render: emulator throughput | keep up with `cat` of 10 MB | ~1.6 MiB/s (misses; see docs/15 Q1) |
