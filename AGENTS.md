# Instructions for coding agents

These rules apply to any agent (Claude Code, Codex, opencode or others) working in this repository.

## Before you write code

1. Read `README.md`, then the docs relevant to your task. `docs/` is the source of truth.
2. If what you are about to build disagrees with a doc, stop and ask, or update the doc in the same change and say why.
3. If the feature depends on something listed in `docs/13-capability-checks.md` that has not passed yet, run or write that check first. Do not build on an unproven capability.

## Stack (see docs/02-tech-stack.md for reasons)

- Go 1.26. One module. Prefer the standard library.
- TUI: Bubble Tea v2 (`charm.land/bubbletea/v2`), Lip Gloss v2, Bubbles v2.
- PTYs: `github.com/charmbracelet/x/xpty` (Unix PTY and Windows ConPTY behind one interface).
- Storage: SQLite through `modernc.org/sqlite` (pure Go, no cgo).
- Windows Hello helper: a small separate C++/WinRT executable under `helpers/windows-hello/`.
- Any JavaScript (for example the browser extension) uses pnpm, never npm or yarn.
- Do not add a dependency without saying in the PR what it does and why the standard library or an existing dependency is not enough. Every dependency costs startup time and binary size.

## Platform rules

- The main developer works on **Windows**. Every feature must work on Windows, macOS and Linux. Windows is not a later port.
- CI must build and test on all three operating systems.
- Never assume a Unix shell. Agent commands may run in PowerShell.
- Use `filepath`, not string concatenation, for paths. Paths shown to users and agents are relative to the workspace root.

## Performance rules

- Respect the budgets in `docs/03-performance.md`. If a change may affect startup, rendering or hook latency, measure it with the bench commands and include the numbers in the PR.
- Never block the render loop on I/O.

## Safety rules

- Never weaken a default in the safety layer (`docs/04-safety.md`) to make a feature easier. If a safety rule blocks a feature, raise it.
- Never log secrets, tokens or the contents of credential files.
- Never commit secrets. Use `.env.example` for any configuration a capability check needs.

## Commits and PRs

- Small, incremental commits with clear messages.
- Do **not** add `Co-Authored-By` lines, session links or any AI attribution lines to commits or PR descriptions.
