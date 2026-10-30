-- Migration 1: initial schema. Times are unix nanoseconds (0 = zero time),
-- string slices are JSON text, booleans are 0/1.

CREATE TABLE workspaces (
    id         TEXT PRIMARY KEY,
    root       TEXT NOT NULL,
    name       TEXT NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE INDEX workspaces_root ON workspaces (root, created_at);

CREATE TABLE sessions (
    id            TEXT PRIMARY KEY,
    workspace_id  TEXT NOT NULL,
    slug          TEXT NOT NULL,
    task          TEXT NOT NULL,
    role          TEXT NOT NULL,
    agent         TEXT NOT NULL,
    base_branch   TEXT NOT NULL,
    branch        TEXT NOT NULL,
    worktree_path TEXT NOT NULL,
    state         TEXT NOT NULL,
    state_detail  TEXT NOT NULL,
    agent_pane    TEXT NOT NULL,
    unseen        INTEGER NOT NULL,
    follow        INTEGER NOT NULL,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);
CREATE INDEX sessions_workspace ON sessions (workspace_id, created_at);
CREATE INDEX sessions_created ON sessions (created_at);

CREATE TABLE summaries (
    seq            INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id     TEXT NOT NULL,
    done           TEXT NOT NULL,
    files_changed  TEXT NOT NULL,
    decisions      TEXT NOT NULL,
    open_questions TEXT NOT NULL,
    next           TEXT NOT NULL,
    at             INTEGER NOT NULL
);
CREATE INDEX summaries_session ON summaries (session_id, at DESC, seq DESC);

CREATE TABLE notes (
    id         TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    number     INTEGER NOT NULL,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL,
    severity   TEXT NOT NULL,
    refs       TEXT NOT NULL,
    dismissed  INTEGER NOT NULL,
    at         INTEGER NOT NULL,
    UNIQUE (session_id, number)
);

CREATE TABLE comments (
    id          TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL,
    path        TEXT NOT NULL,
    new_start   INTEGER NOT NULL,
    new_end     INTEGER NOT NULL,
    old_start   INTEGER NOT NULL,
    old_end     INTEGER NOT NULL,
    base_commit TEXT NOT NULL,
    head_commit TEXT NOT NULL,
    snippet     TEXT NOT NULL,
    context     TEXT NOT NULL,
    language    TEXT NOT NULL,
    body        TEXT NOT NULL,
    status      TEXT NOT NULL,
    changed     INTEGER NOT NULL,
    at          INTEGER NOT NULL
);
CREATE INDEX comments_session ON comments (session_id, path, new_start);

CREATE TABLE checkpoints (
    session_id TEXT NOT NULL,
    n          INTEGER NOT NULL,
    ref        TEXT NOT NULL,
    commit_id  TEXT NOT NULL,
    reason     TEXT NOT NULL,
    at         INTEGER NOT NULL,
    PRIMARY KEY (session_id, n)
);

-- Append-only audit log: the triggers make that true even for raw SQL.
CREATE TABLE approvals (
    seq                 INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id          TEXT NOT NULL,
    session_id          TEXT NOT NULL,
    kind                TEXT NOT NULL,
    tool                TEXT NOT NULL,
    command             TEXT NOT NULL,
    cwd                 TEXT NOT NULL,
    paths               TEXT NOT NULL,
    rule                TEXT NOT NULL,
    layer               TEXT NOT NULL,
    requested_at        INTEGER NOT NULL,
    decision_request_id TEXT NOT NULL,
    outcome             TEXT NOT NULL,
    message             TEXT NOT NULL,
    method              TEXT NOT NULL,
    decided_at          INTEGER NOT NULL
);
CREATE INDEX approvals_session ON approvals (session_id, decided_at DESC, seq DESC);
CREATE TRIGGER approvals_no_update BEFORE UPDATE ON approvals
BEGIN SELECT RAISE(ABORT, 'approvals is append-only'); END;
CREATE TRIGGER approvals_no_delete BEFORE DELETE ON approvals
BEGIN SELECT RAISE(ABORT, 'approvals is append-only'); END;
