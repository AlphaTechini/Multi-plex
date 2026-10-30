# 05 · Sessions and worktrees

## Concepts

- **Workspace**: a repository you open Multiplex in. Its root is the base for every relative path.
- **Session**: one agent working on one task, in one worktree, on one branch.
- **Worktree**: a separate checkout of the same repository (`git worktree add`), sharing the object store. Agents in different worktrees cannot overwrite each other's files while working. Conflicts only appear when branches are combined.

## Session lifecycle

1. **Create.** Pick a task (free text, or a task from a spec), an agent (the installed adapters; the daemon lists them with `agent.list`), a role ([10](10-agent-roles-and-skills.md), listed with `role.list`) and a base branch. The repository needs at least one commit.
2. **Worktree.** The daemon creates `<data dir>/worktrees/<workspace-id>/<session-slug>` on branch `mpx/<session-slug>`. Worktrees live outside the repository so file watchers and search in the main checkout ignore them.
3. **Setup.** Run the workspace's setup recipe (see below) in the background.
4. **Run.** Launch the agent CLI in a PTY inside the worktree, with the scrubbed environment from [04](04-safety.md) and the hook and MCP configuration.
5. **Review.** Read the summary, diffs and notes. Comment and send feedback.
6. **Land.** Merge or rebase into the base branch after approval, or open a PR through `mpx push`.
7. **Clean up.** Remove the worktree and branch (checkpoint refs are kept for a configurable time).

Sessions survive closing the TUI. Reattach from any client.

## Setup recipe

A per-workspace file in `.multiplex/setup.toml` (format to confirm) describing:

- **Files to copy or link** into new worktrees (for example `.env.example` → `.env` with safe local values). Real secrets are not copied ([04](04-safety.md)).
- **Install commands** (for example `pnpm install`, `go mod download`). pnpm's shared store and Go's module cache keep these fast.
- **Port allocation.** Each session gets a port range exposed as environment variables (`MPX_PORT_BASE`, and `PORT` if the project uses it) so dev servers in different sessions do not fight over ports.
- **Services.** Notes on shared resources (a shared local database is a common source of conflicts between agents).

## Sidebar

- Sessions grouped by workspace, each with a state indicator (working, blocked, done, idle, error).
- A blocked session marks its workspace as blocked, so you can see what needs you from the top level.
- A done session stays highlighted until you view it.

## Conflicts

### Predict them early

Git can test a merge without touching any files: `git merge-tree --write-tree <a> <b>` (Git 2.38 or newer) reports conflicting files.

- After each turn ends, test the session's branch against the base branch and against every other active session's branch.
- Show warnings such as "will conflict with session *auth-refactor* in `src/auth/session.go`" in the sidebar and summary, before anyone tries to merge.

### Resolve them

A per-workspace setting, chosen during setup:

| Setting | Behaviour |
|---|---|
| `off` | Only show the warning |
| `ask` (default) | Offer to start a resolver session |
| `auto` | Start a resolver session automatically when a merge is attempted and conflicts |

A **resolver session** is a normal session with the `resolver` role. It gets its own worktree, both branches, the list of conflicting files and both sessions' summaries. Its result goes through the same diff review before it lands.

## Orchestration

An orchestrator session can create and coordinate other sessions through MCP tools (`session_spawn`, `session_status`, `session_wait`). This is how "planner → implementers → reviewer" flows work ([10](10-agent-roles-and-skills.md)).

## Windows notes

- Enable `core.longpaths`. Deep `node_modules` paths exceed the old limit.
- Files open in another process cannot be deleted. Removing a worktree must stop its processes first and retry, since antivirus scanners also hold files briefly.
- Symlinks need Developer Mode or admin rights. Prefer copies in setup recipes on Windows.
- Recommend a Dev Drive for the worktrees folder ([03](03-performance.md)).
