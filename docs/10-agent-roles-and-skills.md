# 10 · Agent roles and skills

## Roles

A role is a reusable definition of how an agent session behaves. Creating one should be as easy as writing a short file, and an agent can create one for you when asked.

### Format

`.multiplex/roles/<name>.md` (workspace) or the user config folder (global):

```markdown
---
name: researcher
description: Investigates a question and reports findings without changing code.
agent: claude-code          # claude-code | opencode | codex | native
model: default              # or a specific model the CLI accepts
permissions: read-only      # read-only | standard | custom
tools:
  deny: [Edit, Write]
report: summary-only
---

You are a researcher. Find the answer, cite files and lines, and do not modify code.
Call report_progress with your findings and note_to_user for anything surprising.
```

### Built-in roles (shipped, editable)

| Role | Purpose |
|---|---|
| `orchestrator` | Breaks work down and starts other sessions through `session_spawn`, then waits on and combines their results |
| `planner` | Writes and updates specs ([09](09-specs.md)). Does not change code. It keeps file-writing tools because spec files are files; its prompt limits writes to `.multiplex/specs/`, and the worktree keeps every change reversible |
| `implementer` | Default coding role |
| `researcher` | Read-only investigation |
| `reviewer` | Reviews another session's diff against the spec's acceptance criteria |
| `resolver` | Resolves merge conflicts between sessions ([05](05-sessions-and-worktrees.md)) |

### How roles map to agent CLIs (wrapper mode)

- **Claude Code**: launch flags (for example an appended system prompt and allowed or denied tools) plus the hook and MCP configuration. Where a role fits better as a Claude Code subagent, write it to the subagent folder as well. Exact flags verified against the current Claude Code CLI before use.
- **Other CLIs**: each adapter declares how it applies a role, or which parts it cannot apply.
- **Native agent (v2)**: roles are applied directly.

### Creating roles with an agent

Ask any session "create a role that ...". It writes the role file in `.multiplex/roles/`. Since that folder is inside the repository, the change is reversible and shows up in the diff like any other change.

## Skills

Skills are folders of instructions (and sometimes scripts) that teach an agent a task. We use the existing `SKILL.md` format so skills written for other tools work unchanged.

### Skills directory

Multiplex does **not** bundle third-party skills. It ships a **catalog**: a list of known skills with name, description, source repository URL, path inside the repo, and a pinned commit.

- Browse the catalog in the sidebar.
- **Install** fetches the skill at the pinned commit into the agent's skills folder (project or user level).
- **Before installing**, show the full `SKILL.md` and list any scripts. A skill is a prompt, so it can carry a prompt injection.
- **Updating** shows the diff between the pinned commit and the new one before applying.
- Users can add their own catalog entries or point at another catalog file.

## Orchestration tools

| MCP tool | Purpose |
|---|---|
| `session_spawn` | Start a session with a role, task and base branch. Returns its ID |
| `session_status` | State, latest summary and notes for one or more sessions |
| `session_wait` | Wait until sessions reach `done` or `blocked` |
| `session_message` | Send a steer message to another session |

Sessions spawned by an agent appear in the sidebar like any other, and follow the same safety rules. An agent can never approve a gated action for another session.
