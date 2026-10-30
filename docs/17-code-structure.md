# 17 · Code structure

How the Go code is laid out, and the rule that keeps replaceable parts replaceable.

## The rule: neutral layers for anything replaceable

Anything that is likely to be swapped later (a library, an external tool, an operating-system API, an agent CLI, later a model provider) sits behind a **neutral layer**: a small Go interface in its own package, written in Multiplex's own terms. Callers import only that package. Each concrete implementation lives in a subpackage and is chosen in one place (the daemon's wiring).

Changing an implementation then means adding or editing one subpackage and one line of wiring, never rewriting callers.

| Concern | Neutral package (interface) | Implementations |
|---|---|---|
| Pseudo-terminals | `internal/pty` (`Spawner`, `Process`) | `internal/pty/xptyimpl` (Unix PTY, Windows ConPTY) |
| Terminal emulation | `internal/vt` (`Emulator`, `Factory`) | `internal/vt/charmvt` (pure Go); libghostty-vt later if CC-02/CC-03 pick it |
| Agent CLIs | `internal/agent` (`Adapter`, registry) | `internal/agent/claudecode`, `internal/agent/shell`; opencode, Codex and the native agent later |
| Storage | `internal/store` (`Store`) | `internal/store/sqlite`, `internal/store/memory` (tests) |
| Version control | `internal/vcs` (`VCS`) | `internal/vcs/gitcli` |
| Syntax highlighting | `internal/highlight` (`Highlighter`) | `internal/highlight/chromahl` |
| Human-present approval | `internal/approval` (`Verifier`) | `windowshello`, `polkit`, `typedcode` |
| Wire encoding | `internal/protocol` (`Codec`) | `protocol.JSON` (Q3 may add a binary codec) |
| Local IPC | `internal/transport` (`Listen`, `Dial`) | Unix socket, Windows named pipe (build tags) |
| Theme | `internal/ui/theme` (`Theme`, registry) | `DarkPlus` |
| Model providers (Phase 7) | `internal/provider` (to be written) | Anthropic API, OpenAI-compatible, local models |

The domain types every layer speaks (`Session`, `Note`, `Comment`, ...) live in `internal/core`, which depends on nothing but the standard library.

## One major function per file

Each file has one main exported function or type, and is named after it (`worktree_add.go`, `classify.go`, `panel.go`). Small private helpers that only serve that function stay in the same file. This keeps files short, easy to find, and easy to replace.

## Layout

```
cmd/mpx/                 the single binary: TUI, daemon, hook, mcp, bench, push
cmd/mpx-preview/         renders UI screens with fake data, for design work
internal/core/           domain types
internal/protocol/       daemon ⇄ client messages and codec
internal/transport/      local socket / named pipe
internal/config/         paths and settings
internal/daemon/         session manager, pane host, event bus, handlers
internal/client/         typed client for the daemon protocol
internal/term/           pane host: PTY + emulator + frame coalescing
internal/safety/         classifier, path policy, env scrubbing, gate
internal/diff/           unified diff model, word-level diff, comment anchoring
internal/spec/, role/    specs and roles from .multiplex/
internal/mcpserver/      `mpx mcp` tools
internal/ui/             Bubble Tea app, theme, kit primitives, views
capability-checks/       CC-xx programs and results (docs/13)
helpers/windows-hello/   C++/WinRT approval helper
```

## Conventions

- Paths shown to users or sent to agents are relative to the workspace root, slash-separated. Use `filepath` for filesystem work.
- Nothing in the render loop does I/O. The TUI talks to the daemon through commands that run off the loop.
- Every exported identifier has a doc comment.
