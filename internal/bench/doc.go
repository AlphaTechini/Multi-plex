// Package bench implements `mpx bench` (docs/03-performance.md "How we
// measure"): startup, hook and render measurements. Each prints a table, can
// print JSON, and can write a result file with the machine description to
// evidence/bench/<name>.json.
//
// Contract with the TUI for `mpx bench startup`: when the environment variable
// MPX_BENCH_FIRST_FRAME=1 is set, the TUI renders its first frame, prints one
// line "first_frame=<Go duration>" (for example "first_frame=84.3ms") to
// standard error (standard output belongs to the renderer), and exits with status 0. The duration runs from the start
// of the process's main function to the first frame being written to the
// terminal. The bench starts the TUI in a pseudo-terminal, once with no daemon
// running ("cold": the TUI starts one) and several times with the daemon
// already running ("warm"), each in a private MPX_HOME.
package bench

import "time"

// EnvFirstFrame is the environment variable that asks the TUI to exit after
// its first frame and print how long it took.
const EnvFirstFrame = "MPX_BENCH_FIRST_FRAME"

// FirstFramePrefix starts the line the TUI prints.
const FirstFramePrefix = "first_frame="

// Budgets from docs/03-performance.md.
const (
	BudgetColdStart   = 150 * time.Millisecond
	BudgetWarmAttach  = 50 * time.Millisecond
	BudgetHookLinux   = 10 * time.Millisecond
	BudgetHookWindows = 25 * time.Millisecond
)
