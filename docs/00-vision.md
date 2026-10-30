# 00 · Vision

## The problem

Working with several coding agents today means juggling windows: two editor windows, a Claude Code session in each, a terminal panel squeezed under the code, and diffs that stack every changed file into one long scroll. It works, but:

- You cannot see at a glance which agent is working, waiting for you, or finished.
- Agents in the same folder step on each other.
- Diff viewers make it easy to scroll from one file into the next without noticing.
- Approving a dangerous command is a single keypress, which an agent (or your own tired hand) can trigger without real attention.
- The things an agent thinks you should know scroll past while it keeps working, and asking about them means interrupting it or copying text by hand.
- Editors load far more than an agent-driven workflow needs.

## What we are building

A developer productivity tool for agent-driven development. A light terminal application where:

- Each agent session runs in its own git worktree, so agents do not conflict while working.
- The screen is split into a narrow **agent panel** (the agent's own terminal, full height, on the side) and a large **main view** for reading: summaries, diffs, code and specs.
- Reviewing agent work is the main activity, so the diff viewer gets the most design attention.
- Irreversible actions need a human-present approval (biometric where available).
- Everything is fast enough that the tool never becomes the thing you wait on.

The first user is the author. It is built for daily use first, and as a showcase second.

## Principles

1. **Fast by default.** Startup, switching and rendering are measured against budgets, not felt out.
2. **Reversible means allowed; irreversible means ask.** Make as much as possible reversible so prompts stay rare and meaningful.
3. **Opinionated defaults, replaceable parts.** Useful out of the box (roles, specs, review flow) but every part can be configured or swapped. Not an empty "bring your own everything" core.
4. **Agent-agnostic core.** Sessions, worktrees, review and safety do not depend on which agent CLI is running.
5. **Read more code, not less.** The UI should pull you back into reading the code the agent writes (follow mode, focused diffs), not hide it.
6. **Paths relative to the workspace root.** Everything shown to the user or sent to an agent uses repository-relative paths.

## What we take from existing tools

| Tool | What we take | What we leave |
|---|---|---|
| herdr | Agent-aware multiplexer, client-server design, detach and reattach, agents that can drive the multiplexer | Its UI, its rendering issues, state detection that depends only on scraping agent output |
| Kiro | Spec-driven development (requirements, design, tasks), the sidebar giving quick access to steering and global instruction files, following the agent as it edits | The rest of the IDE |
| Claude Code | `/btw` side questions, slash commands, hooks as integration points | Insight notes you cannot act on without interrupting |
| opencode | Leader-key shortcuts as an alternative to slash commands | |
| Zed | Lightness and startup speed as the bar to meet | Its default look |
| pi | Small core that loads extensions | Shipping almost nothing by default |
| Cursor / Antigravity | Ideas for presenting multiple agents (to review later) | |

## Non-goals (for now)

- **Not a full code editor.** You can read code, diffs and specs, and jump to your editor of choice. Editing stays in your editor.
- **No built-in browser.** Browser access goes through an extension in your real browser ([12-browser.md](12-browser.md)).
- **Not a sandbox that cuts the agent off from your machine.** Agents can read broadly. The safety layer controls what they can change irreversibly.
- **No telemetry.**
