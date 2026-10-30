# 03 · Performance

## What "fast" means here

A terminal does not make a command run faster. `go test` uses the same CPU time in any terminal, and agents do not run their commands through a terminal renderer at all. What we can control is **everything around the command**:

1. How fast Multiplex starts and attaches.
2. How much latency we add between an agent deciding to run something and it running.
3. How fast output reaches the screen.
4. How much setup overhead each command pays (shell startup, profile loading, antivirus scanning, cold caches).
5. In v2 (native agent), how commands are executed and how output is fed back to the model.

## Budgets

These are targets to measure against, not guesses. Measure release builds on the author's Windows laptop and on Linux CI.

| Metric | Budget |
|---|---|
| Cold start of `mpx` to first frame (daemon not running) | < 150 ms |
| Attach to a running daemon to first frame | < 50 ms |
| Switch session or main-view mode | < 16 ms (one frame) |
| Added latency, keystroke → agent PTY | < 1 ms |
| `mpx hook` round trip for an allowed tool call | < 10 ms on Linux, < 25 ms on Windows |
| Render throughput | Keep up with `cat` of a 10 MB log without the UI freezing; drop intermediate frames, never input |
| Idle CPU with 5 sessions open | ~0% when nothing changes on screen |
| Memory, daemon + TUI, 5 sessions | < 150 MB excluding the agents themselves |
| Binary size | Track it. No hard cap, but every dependency must justify itself |

## Where an agent's time actually goes

1. **Model round-trips.** Usually the largest cost by far. We cannot speed up the model, but v2 can reduce the number of round-trips (parallel tool calls, batching).
2. **Process and shell startup per command.** A fresh shell that loads a heavy profile (nvm, conda, oh-my-posh) can cost hundreds of milliseconds every time.
3. **Waiting heuristics.** Fixed timeouts or polling instead of reacting to exit status.
4. **Huge outputs.** Thousands of lines of build output sent to the model cost time and tokens.
5. **Cold caches** in new worktrees (dependency installs, build caches).

## Levers

### v1 (wrapper): what we control

- **Our own overhead.** Daemon architecture means the TUI only renders, and attaching is cheap.
- **Hook latency.** `mpx hook` is a tiny binary that connects to the daemon, sends one message and exits. The fast path (allow-listed commands) never touches disk.
- **The environment we launch agents in.** We start agent CLIs, so we choose:
  - a lean shell configuration for the agent (for example PowerShell with `-NoProfile`, or a minimal profile written by us), measured in CC-12;
  - environment variables that point tools at shared caches;
  - worktrees on a fast volume (see Windows below).
- **Command rewriting (experimental).** Claude Code's `PreToolUse` hooks can replace a tool's input (`updatedInput`). In principle this lets us route shell commands through a persistent shell owned by the daemon instead of a fresh shell per command. It changes how commands run, so it stays behind a flag until measured and proven safe (CC-15).
- **Warm worktrees.** pnpm's shared store and Go's build cache make new worktrees cheap. Worktree setup runs installs once, in the background.
- **Rendering.** Only changed cells are redrawn. PTY output bursts are coalesced to at most one frame per 16 ms.

### v2 (native agent): what we additionally control

- One persistent shell per session instead of a new shell per command.
- Exit-status-driven completion, no polling.
- Output streaming with smart truncation (head, tail, errors) before it reaches the model.
- Parallel execution of independent tool calls.
- Preferring fast tools (ripgrep, fd) and shipping them if missing.

## Windows specifics

- **PowerShell startup** can be slow, especially with a profile. Measure `pwsh -NoProfile` versus the default, and a persistent session versus a new process per command (CC-12).
- **Process creation** is more expensive on Windows than Linux. This matters for `mpx hook`, which runs once per tool call.
- **Microsoft Defender real-time scanning** slows process launches and heavy file I/O (installs, builds). Windows 11's **Dev Drive** (ReFS with Defender performance mode) exists for exactly this. Recommend putting worktrees on a Dev Drive and measure the effect (CC-13).
- **Long paths**: enable `core.longpaths` for worktrees with deep `node_modules`.

## How we measure

- `mpx bench startup`: cold and warm start to first frame.
- `mpx bench hook`: N hook round trips, p50/p95/p99.
- `mpx bench render`: replay a recorded PTY stream and measure frame times.
- `mpx bench shell`: per-command overhead of the configured agent shell.

Results go in `evidence/bench/` with the machine, OS and terminal used. Any change that may affect these numbers includes before/after results in the PR.
