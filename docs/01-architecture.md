# 01 · Architecture

## Overview

Multiplex is one Go binary with several roles, plus small platform helpers.

```
  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐
  │  mpx (TUI)   │   │ remote TUI   │   │ mobile app   │   clients
  └──────┬───────┘   └──────┬───────┘   └──────┬───────┘
         │ local socket     │ SSH tunnel       │ WebSocket (later)
         └──────────────────┼──────────────────┘
                            ▼
  ┌───────────────────────────────────────────────────────┐
  │                 mpx daemon (per user)                 │
  │                                                       │
  │  session manager   workspaces → sessions → panes      │
  │  pty host          xpty (Unix PTY / ConPTY)           │
  │  vt emulator       one terminal state per pane        │
  │  worktree manager  git worktrees, branches, merges    │
  │  approval broker   classify → gate → biometric        │
  │  review store      diffs, comments, summaries (SQLite)│
  │  event bus         state changes pushed to clients    │
  └───────▲─────────────────────▲─────────────────▲───────┘
          │                     │                 │
   `mpx hook ...`          `mpx mcp`        helpers/windows-hello
   (agent CLI hooks)  (MCP server for agents)  (biometric prompt)
          │                     │
  ┌───────┴─────────────────────┴───────┐
  │ agent CLIs in PTYs: Claude Code,    │
  │ later opencode, Codex, native agent │
  └─────────────────────────────────────┘
```

### Binary roles

| Command | Role |
|---|---|
| `mpx` | The TUI client. Starts the daemon if it is not running, then attaches. |
| `mpx daemon` | The long-running daemon. One per user. Survives the TUI closing. |
| `mpx hook <event>` | Called by an agent CLI's hook system (for example Claude Code `PreToolUse`). Forwards the event to the daemon and returns its decision. Must start and exit fast. |
| `mpx-hook <event>` | The same hook entry point as a separate ~3 MB binary that links only the hook path. The daemon points agents at it when it sits next to `mpx` (and falls back to `mpx hook`), because the full binary's package initialisation (the TUI's syntax highlighter above all) costs ~20 ms per call, twice the hook budget. |
| `mpx mcp` | A stdio MCP server the agent CLI launches. Gives agents tools such as `report_progress` and `note_to_user`, and forwards calls to the daemon. |
| `mpx bench ...` | Performance measurements (see [03-performance.md](03-performance.md)). |

Binary name `mpx` is a working name ([15-open-questions.md](15-open-questions.md)).

### Why a daemon

- **Persistence.** Agents keep running when the UI closes. Reattach is instant because the daemon holds each pane's terminal state.
- **Multiple clients.** A local TUI, a remote TUI and a phone can attach to the same sessions.
- **Cloud for free.** A cloud session is the same daemon running on a server.
- **One place to enforce safety.** Every hook call and approval goes through the daemon.

## Data model

```
Workspace   one repository root the user opened
 └─ Session one agent run: role, agent CLI, worktree, branch, state
     ├─ Pane: agent   the agent CLI's PTY
     ├─ Pane: shell   optional extra terminals in the same worktree
     ├─ Checkpoints   git refs captured before risky steps
     ├─ Summary       latest structured progress report
     ├─ Notes         "you should know" cards
     └─ Comments      review comments anchored to diff lines
```

### Session states

`starting → working ⇄ blocked → done → idle`, plus `error`.

- **blocked** means the agent is waiting on the user: an approval, a question or input.
- State comes from **hooks first** (Claude Code fires hooks on tool use, notifications and stop events), and only falls back to reading terminal output when a CLI has no hooks. This avoids the fragility of tools that guess state from output alone.

## Terminal hosting

Each pane is a real PTY (Unix PTY or Windows ConPTY through `xpty`). The daemon feeds the PTY output into a **VT emulator**, which keeps the screen state (cells, colours, cursor, scrollback). Clients render from that state.

The VT emulator is the hardest piece to get right. Small mistakes show up as broken rendering of the agent's own TUI. Choice between libghostty-vt and a pure-Go emulator is decided by capability check CC-02/CC-03 ([13-capability-checks.md](13-capability-checks.md)).

Client updates are sent as **changed cells only**, not full screens. This keeps local rendering cheap and remote attach usable on slow connections.

## Integration points with agent CLIs (wrapper mode)

The core never depends on a specific CLI. Each CLI gets an **adapter** that declares what it supports.

| Capability | Claude Code (v1) | Others (later) |
|---|---|---|
| Run in a PTY | yes | yes |
| Gate tool calls before they run | `PreToolUse` hook → `mpx hook` | per CLI, verify |
| Report state | hooks (tool use, notification, stop) | hooks or output heuristics |
| OS-level write restriction | its built-in sandbox on macOS, Linux and WSL2 (not native Windows) | per CLI |
| Structured summaries and notes | MCP server `mpx mcp` | MCP where supported |
| Side questions | its own `/btw` typed into the PTY | per CLI |
| Roles | launch flags and subagent files | per CLI |

Every row marked as relying on CLI behaviour has a capability check before we build on it.

### The MCP server

Agents get these tools from `mpx mcp`:

| Tool | Purpose | Doc |
|---|---|---|
| `report_progress` | Structured summary: done, files changed, decisions, open questions | [08](08-insights-and-steering.md) |
| `note_to_user` | Create a numbered insight card | [08](08-insights-and-steering.md) |
| `spec_get`, `task_update` | Read specs, tick tasks | [09](09-specs.md) |
| `session_spawn`, `session_status`, `session_wait` | Let an orchestrator start and coordinate other sessions | [10](10-agent-roles-and-skills.md) |

A short instruction (a skill or an appended system prompt, depending on the CLI) tells agents when to call them.

## Storage

| What | Where | Committed to the repo? |
|---|---|---|
| Specs, roles, workspace settings | `.multiplex/` in the repository | Yes, if the user wants |
| Sessions, comments, summaries, approvals log | SQLite in the user data directory | No |
| Worktrees | user data directory, outside the repository | No |
| Daemon socket | user runtime directory (named pipe on Windows) | No |

Worktrees live outside the repository so file watchers, search tools and other agents in the main checkout do not pick them up.

## Client to daemon protocol

- Transport: Unix domain socket on macOS and Linux, named pipe on Windows (`github.com/Microsoft/go-winio`).
- Messages: request/response for commands, plus a stream of events (state changes, cell updates).
- Encoding: to decide ([15-open-questions.md](15-open-questions.md)). JSON is easiest to debug. A binary encoding may be needed for cell streams.
- The same protocol runs over a WebSocket for remote and mobile clients later, behind authentication ([11](11-remote-mobile-cloud.md)).

## Native agent (v2)

Later, a native agent loop runs inside the daemon as another adapter. It talks to model providers directly (API keys, OpenAI-compatible endpoints, local models) and owns command execution, which is where command speed and safety are fully under our control. Claude subscriptions cannot be used by third-party agent loops under Anthropic's terms, so Claude Code stays supported as a wrapped CLI.
