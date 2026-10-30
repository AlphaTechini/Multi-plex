# 06 · UI and layout

## Layout

The agent panel is a **side panel**: a tall, narrow column beside the main view, full height (like Zed's agent panel). It is **not** a bottom panel under the code.

> Wording: "horizontal" and "vertical" split mean opposite things in different tools (Vim's vertical split is side by side; tmux calls side by side a horizontal split). In this project we only say **side panel** and **bottom panel**.

```
┌──────────┬──────────────────────────────────────────┬─────────────────────┐
│ SIDEBAR  │ MAIN VIEW                                │ AGENT PANEL         │
│          │ [Overview] [Diff] [Code] [Specs] [Term]  │                     │
│ Sessions │                                          │ the agent CLI's own │
│ ● auth   │  mode content:                           │ terminal (real PTY) │
│ ◐ ui     │  summary, diff, followed code, spec,     │                     │
│ ✓ docs   │  or a shell in this session's worktree   │ tool calls shown as │
│          │                                          │ one short line each │
│ Specs    │                                          │                     │
│ Steering │                                          │                     │
│ Roles    │                                          │ › input             │
│ Skills   │                                          │                     │
│ MCP      │                                          │                     │
├──────────┴──────────────────────────────────────────┴─────────────────────┤
│ status: workspace · session · branch · state · approvals · keys hint       │
└────────────────────────────────────────────────────────────────────────────┘
```

### Agent panel

- Default width about **30%** of the terminal, resizable, minimum about **60 columns**. Narrower than that, code snippets and long commands wrap too much.
- In wrapper mode this is the agent CLI's real terminal, rendered through our VT emulator. Its own slash commands and keys still work.
- In native mode (v2) we render the conversation ourselves and show each tool call as one line (`Ran: Test-Path MEMORY.md ✓`). Expanding a tool call opens its full command and output in the main view, not in the narrow panel.
- The panel can be collapsed, or swapped to the left side.

### Main view modes

| Mode | Shows |
|---|---|
| **Overview** | The session's latest structured summary, numbered insight cards, conflict warnings, spec task progress ([08](08-insights-and-steering.md)) |
| **Diff** | The diff viewer ([07](07-diff-viewer.md)) |
| **Code** | Read-only code view. With follow mode on, it shows the file and lines the agent is editing as it edits them |
| **Specs** | The workspace's specs and their task lists ([09](09-specs.md)) |
| **Term** | A shell in this session's worktree, taking the full main view |

One key switches between modes. The main view always belongs to the session selected in the sidebar.

### Sidebar (inspired by Kiro)

Quick access to:

- **Sessions** grouped by workspace, with state indicators.
- **Specs**.
- **Steering and global instructions**: `AGENTS.md`, `CLAUDE.md`, the user's global agent instruction files, and `.multiplex/` settings. Open to read, jump to your editor to change.
- **Roles**, **Skills**, **MCP servers** in use.

## Follow mode

- Toggle per session. While on, the Code view jumps to each file and line range the agent edits, highlighting new changes briefly.
- **Follow mode is view-only.** Clicking or scrolling in the main view never interrupts or sends anything to the agent. Interrupting is only possible from the agent panel.
- Scrolling away pauses following, with a visible "following paused, press F to resume" indicator.
- The edit stream comes from hook events and from watching the worktree for changes.

## Keys and commands

- **Prefix key** for Multiplex actions, like tmux. Keys without the prefix go to the focused pane, so the hosted agent works normally.
  - The prefix must not clash with keys the hosted agents use. opencode uses `Ctrl+X` as its own leader and Claude Code uses `Ctrl+B` to send work to the background, so neither is a safe default. Default to be chosen after CC-09. Fully configurable.
  - A **backslash prefix does not work** in a terminal host: we cannot tell whether the agent's input box is empty, so we would steal every `\` the user types (Windows paths, regex, escapes).
- **Slash commands** in inputs we own (native agent panel, comment boxes, command palette): `/` at the start of an input. In wrapper mode the agent's own `/` commands belong to the agent.
- **Command palette** on prefix + `p` (to confirm) for everything else.
- Both styles are configurable in one keymap file.

## Theme and motion

- Default colours from VS Code's **Dark+** theme (the theme files are MIT licensed in the VS Code repository). Port editor background, token colours and diff colours.
- Requires **truecolor**. Falls back to 256 colours when the terminal does not support it.
- Motion is short and purposeful: file-change banners and a quick fade (colour interpolation over about 150 ms) when the diff viewer moves to another file. A "reduce motion" setting turns all of it off.

## Mouse

- Clickable sidebar items, tabs, cards and diff lines.
- Mouse capture disables the terminal's native text selection. Holding Shift while dragging usually bypasses capture. Verify per terminal in CC-04, and provide a copy action for selected diff lines and cards.

## Terminal compatibility

Must work in: Windows Terminal, Ghostty, iTerm2, WezTerm, Kitty, Alacritty, and the integrated terminals of VS Code and Zed. Check macOS Terminal.app's colour support before claiming it. The legacy Windows console host (conhost) is not supported.
