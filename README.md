# Multiplex

A fast, safety-first terminal workbench for running, watching and reviewing several AI coding agents at once, each in its own git worktree.

> **Status:** first working build, tested on Linux only. Built: daemon and TUI with detach/reattach, sessions in worktrees running Claude Code, the diff viewer with review comments, the safety gate (classifier, path policy, credential scrubbing, checkpoints, approvals, audit log, push broker), insight cards and summaries over MCP, follow mode, specs, roles, orchestration tools and conflict prediction. Not built yet: the skills catalog, worktree setup recipes, Claude Code's own sandbox on macOS/Linux and the Windows write restriction (CC-11), Touch ID, and everything from Phase 6 on ([roadmap](docs/14-roadmap.md)). The checks that need the author's Windows machine (CC-01 ConPTY, CC-02 against real agent TUIs, CC-05 Windows Hello, CC-06 hook enforcement, CC-09 key delivery) are still open. `docs/` remains the source of truth.

## What it is

Multiplex is a terminal application (TUI) backed by a small background daemon. You open it in a repository and it lets you:

- **Run several agent sessions in parallel**, each in its own git worktree and branch, with a sidebar showing which agents are working, blocked, done or idle.
- **Watch and review their work** in the main view: a live summary of what each agent has done, a diff viewer built for reading one file at a time, and a follow mode that shows code changing as the agent writes it.
- **Comment on a diff** and send those comments to the agent as precise context (`path:L12-L30` plus the snippet).
- **Stop destructive actions** unless you approve them with a biometric check (Windows Hello, Touch ID) or an OS password where no biometric hardware exists.
- **Start from specs** (requirements, design, tasks) in the style of Kiro, so agents work against written acceptance criteria.

Version 1 is a **wrapper**: it hosts existing agent CLIs (Claude Code first) in real terminal panes and adds sessions, worktrees, review and safety around them. A native agent loop comes later (see [roadmap](docs/14-roadmap.md)).

## Build and run

Requires Go 1.26 and Git 2.38 or newer. [Claude Code](https://code.claude.com) on `PATH` for agent sessions.

```sh
go build -o bin/ ./cmd/mpx ./cmd/mpx-hook   # keep mpx-hook next to mpx: it is the fast hook entry point
bin/mpx --demo                              # the full UI on sample data, no daemon or agent needed
cd your-repo && /path/to/bin/mpx            # starts the daemon if needed and opens this repository
bin/mpx doctor                              # paths, daemon, git, agents found, approval method
```

On Windows build `helpers/windows-hello` too and put `mpx-hello.exe` next to `mpx.exe` (see its README).

Keys: the prefix is `Ctrl+\` (keys without it go to the focused pane, so the agent works normally). `Ctrl+\ p` opens the command palette, `Ctrl+\ n` starts a session, `Ctrl+\ ?` lists every key, `Ctrl+\ q` detaches; sessions keep running in the daemon.

Other commands: `mpx push` (push a session's branch after a human-present approval), `mpx bench startup|hook|render` (results in [evidence/bench](evidence/bench/README.md)), `mpx daemon`, `mpx version`.

## Priorities, in order

1. **Speed**: instant startup, no added latency on agent commands, rendering that keeps up.
2. **Safety**: nothing irreversible happens without a human-present approval.
3. **Review experience**: the diff viewer and summaries are the main way you read agent work.
4. **Multi-agent management**: sessions, worktrees, conflicts and roles.
5. **Comfort**: a UI that is pleasant to look at all day (VS Code Dark+ colours by default).

## Hard requirements (do not ship without)

- Starts fast and stays light (budgets in [docs/03-performance.md](docs/03-performance.md)).
- Works in any modern terminal (Windows Terminal, Ghostty, iTerm2, WezTerm, Kitty, Alacritty) on Windows, macOS and Linux.
- Irreversible actions are blocked without explicit, human-present approval.
- Proper agent workflow management: sessions survive closing the UI and can be reattached.

## Docs

| # | Doc | What it covers |
|---|---|---|
| 00 | [Vision](docs/00-vision.md) | Why this exists, principles, inspirations, non-goals |
| 01 | [Architecture](docs/01-architecture.md) | Daemon and clients, wrapper mode, data model, integration points |
| 02 | [Tech stack](docs/02-tech-stack.md) | Every stack decision, the reason, and what was rejected |
| 03 | [Performance](docs/03-performance.md) | What "fast" means here, budgets, levers, how we measure |
| 04 | [Safety](docs/04-safety.md) | Threat model, the reversibility rule, enforcement layers, approvals |
| 05 | [Sessions and worktrees](docs/05-sessions-and-worktrees.md) | Session lifecycle, worktrees, conflict prediction, resolver agent |
| 06 | [UI and layout](docs/06-ui-layout.md) | Layout, views, follow mode, keys, theme, terminal compatibility |
| 07 | [Diff viewer](docs/07-diff-viewer.md) | One-file-at-a-time review, comments, anchoring, structural diffs |
| 08 | [Insights and steering](docs/08-insights-and-steering.md) | Clickable "you should know" cards, ask vs steer, summaries |
| 09 | [Specs](docs/09-specs.md) | Spec-driven development: requirements, design, tasks |
| 10 | [Agent roles and skills](docs/10-agent-roles-and-skills.md) | Custom roles, orchestrator, skills directory |
| 11 | [Remote, mobile and cloud](docs/11-remote-mobile-cloud.md) | Attaching from elsewhere, phone app, cloud daemon |
| 12 | [Browser](docs/12-browser.md) | Browser access through an extension instead of a built-in browser |
| 13 | [Capability checks](docs/13-capability-checks.md) | Things we must prove work before building on them |
| 14 | [Roadmap](docs/14-roadmap.md) | Phases, scope and exit criteria |
| 15 | [Open questions](docs/15-open-questions.md) | Decisions not made yet |
| 16 | [Visual design](docs/16-visual-design.md) | Design system: colour tokens, glyphs, screen mockups, motion, responsive rules |
| 17 | [Code structure](docs/17-code-structure.md) | Package layout and the neutral-layer rule for replaceable parts |

## For coding agents

Read [AGENTS.md](AGENTS.md) before changing anything.
