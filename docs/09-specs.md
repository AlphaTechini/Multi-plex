# 09 · Specs (spec-driven development)

Inspired by Kiro's spec workflow. The value is in the process (written requirements, a design, and a task list the agent works through), not Kiro's UI.

## Files

```
.multiplex/specs/<feature-slug>/
  requirements.md   user stories with acceptance criteria
  design.md         architecture, data, interfaces, error handling, testing approach
  tasks.md          ordered, checkable implementation tasks
```

These live in the repository so they can be reviewed and versioned like code.

## Requirements format

User stories with acceptance criteria in EARS style ("Easy Approach to Requirements Syntax"), which Kiro also uses:

```markdown
### Requirement 1: Token refresh

**User story:** As an API client, I want expired tokens refreshed automatically, so that requests do not fail mid-session.

**Acceptance criteria**
1. WHEN a request is made with a token that expires within 60 seconds, THE SYSTEM SHALL refresh the token before sending the request.
2. IF the refresh fails, THEN THE SYSTEM SHALL return an authentication error and SHALL NOT retry more than once.
```

Each criterion gets a stable ID (`R1.1`, `R1.2`) so tasks, tests and review comments can point at it.

## Flow

1. **Start a spec** from a one-paragraph idea. A planner session drafts `requirements.md`.
2. **Review gate.** You edit or approve requirements before design starts.
3. The planner drafts `design.md`. **Review gate.**
4. The planner drafts `tasks.md`: small tasks, each listing the requirement IDs it satisfies.
5. **Implement.** Start sessions from tasks (one task or a group per session). Agents read the spec through `spec_get` and tick tasks through `task_update`.
6. **Verify.** A reviewer session (or you) checks each acceptance criterion against the code and tests, and records which criteria are met.

Gates can be skipped per spec for small features.

## In the UI

- **Specs** in the sidebar lists specs with progress (tasks done / total, criteria verified / total).
- The **Specs** main-view mode shows the three files with task checkboxes and links to the sessions working on each task.
- Diff review comments can reference criterion IDs.

## MCP tools

| Tool | Purpose |
|---|---|
| `spec_list` | List specs in the workspace |
| `spec_get` | Read a spec's files, or one requirement by ID |
| `task_update` | Mark a task in progress, done or blocked, with a note |
| `criterion_verify` | Record that a criterion is met, with evidence (test name, file and lines) |
