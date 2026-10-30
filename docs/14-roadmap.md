# 14 · Roadmap

Each phase ends with evidence you can see or measure, not a claim. Windows is the first platform in every phase; macOS and Linux must build and pass tests in CI throughout.

## Phase 0 · Capability checks

Run the checks that block Phase 1: CC-01, CC-02, CC-03 (if libghostty-vt is a candidate), CC-04, CC-09, CC-12.

**Exit:** results written in each check's README; VT emulator and prefix key decided ([15](15-open-questions.md)).

## Phase 1 · A multiplexer you can use daily

- Daemon + TUI client, auto-start, detach and reattach.
- Workspaces and sessions; each session creates a worktree and runs Claude Code in it.
- Layout: sidebar, main view (Overview placeholder, Term), agent side panel.
- Session state from Claude Code hooks (working, blocked, done, idle).
- Prefix key, command palette, Dark+ theme.
- `mpx bench startup` and `mpx bench render`.

**Exit:** the author runs three Claude Code sessions in parallel for a full working day on Windows; startup and switching meet the budgets in [03](03-performance.md); recorded bench results in `evidence/bench/`.

## Phase 2 · The diff viewer

- One-file-at-a-time diff with file rail, explicit navigation, banners and fade.
- Word-level highlights, collapsed context, side-by-side on wide terminals.
- Live refresh while the agent works.
- Comments with anchoring, sent to the agent as one message.
- Worktree setup recipes (copy files, install, ports).

**Exit:** a full review of a real multi-file agent change done entirely in Multiplex, comments sent and acted on by the agent; the author prefers it to Zed's diff view.

## Phase 3 · Safety

- `mpx hook` gate with the classifier rules.
- Checkpoints before shell commands and at turn ends, with rollback.
- Scrubbed environment, deny-read list for credential stores, git push broker.
- Windows Hello helper; password fallback.
- Approval prompt UI, audit log, approvals-per-hour metric.
- Claude Code's own sandbox switched on for every session on macOS and Linux, with credential stores on `denyRead`.
- OS write restriction on Windows through the route CC-11 picks (WSL2, low integrity level or AppContainer).

**Exit:** a scripted test suite of dangerous actions (including the bypass examples in [04](04-safety.md)) where every irreversible action is either blocked or rolled back, with results committed; `mpx bench hook` within budget.

## Phase 4 · Insights, summaries and steering

- `mpx mcp` with `report_progress` and `note_to_user`.
- Overview mode: summary and numbered cards.
- Ask (via `/btw`), Steer and Interrupt actions.
- Follow mode in the Code view.

**Exit:** CC-07 and CC-08 pass; a day of use where cards were answered without interrupting the agent.

## Phase 5 · Specs, roles and coordination

- Specs (requirements, design, tasks) with review gates and the spec MCP tools.
- Roles with built-in defaults; agents can create roles.
- Orchestrator tools (`session_spawn`, `session_status`, `session_wait`).
- Conflict prediction and resolver sessions.
- Skills catalog with pinned installs.

**Exit:** one feature taken from idea to merged code through a spec, with a planner, two implementers in parallel and a reviewer, including one predicted and resolved conflict.

## Phase 6 · Remote and mobile

- Remote attach over SSH.
- Network API with device pairing.
- Mobile client: session states, summaries, passkey approvals.

## Phase 7 · Native agent (v2)

- Agent loop inside the daemon with API-key providers (Anthropic API, OpenAI-compatible, DeepSeek, local models).
- Persistent shell per session, exit-status completion, output truncation, parallel tool calls.
- Native rendering of the conversation in the agent panel.
- Own browser extension.

**Exit:** measured per-command overhead and end-to-end task time compared against the same tasks in wrapper mode.
