# 07 · Diff viewer

The diff viewer is where most time in this tool is spent. It has to be the best-looking and most comfortable part of the app, and the author is the judge.

## Problems with existing viewers

- All changed files are stacked into one long scroll, so it is easy to scroll from one file into the next without noticing.
- It gets messy fast after a few files.
- No good way to point at a specific change and say something about it to the agent.

## Design

### One file at a time

- The main view shows **one file's changes**. Never a stacked list.
- A **file rail** (collapsible, left of the diff) lists changed files with status (added, modified, deleted, renamed), change counts, and a reviewed checkmark.
- Moving between files is always explicit: click in the rail, or the **next file / previous file** keys.
- On every file switch, a short **banner** shows the new path (`src/auth/session.go · 3 of 7`) and the content fades in. You always know you have moved to a different file.
- Reaching the end of a file does **not** scroll into the next one. You see an end marker with "next file: `path` (key)".

### Inside a file

- **Unified view by default**, with word-level highlights inside changed lines so you see exactly what changed in a line, not just that it changed.
- **Side-by-side** available when the terminal is wide enough (about 200 columns or more), toggled per user preference.
- Unchanged regions collapse to "··· 42 unchanged lines ···", expandable.
- Syntax highlighting with chroma, Dark+ colours.
- Hunk navigation keys (next/previous change within the file).
- Per-hunk and per-file **reviewed** state, so you can come back to unfinished reviews.

### Live updates

- While an agent is working, the diff refreshes on file changes (debounced). New hunks are briefly highlighted.
- The diff source is `git diff` of the session's worktree against its base, including untracked files, so it works with any agent CLI.

### Structural mode (later)

With tree-sitter, show changes in terms of code structure: "function `Refresh` moved from `a.go` to `b.go`", "parameter renamed", formatting-only changes hidden. difftastic is the reference for this approach. This is the part of the viewer most likely to be new compared with existing tools.

## Comments

### Making a comment

- Select one line or a range in the diff (keyboard or mouse) and press the comment key.
- Type the comment. It shows inline under the selected lines.
- Comments can be draft (only for you) or queued (to send to the agent).

### Anchoring

Each comment stores:

- the path relative to the workspace root;
- the line range on the new side (and old side for deletions);
- the base and head commits at the time;
- the exact text of the selected lines and a few lines around them.

When the file changes later, the comment re-finds its place by searching for the stored text near the old position. If it cannot, it is marked **outdated** and still shows the original snippet.

### Sending to the agent

Queued comments are sent as one message:

````
Review comments (session: auth-refactor):

1. src/auth/session.go:L12-L30
   ```go
   <the selected lines>
   ```
   Comment: this refresh logic runs on every request, cache the token instead.

2. src/auth/session_test.go:L88
   ...
````

- Wrapper mode: the message is typed into the agent's input. Whether it arrives as a queued message or interrupts depends on the CLI (CC-08).
- Native mode: delivered as a user message with the comments attached as structured data.
- After the agent's next turn, each comment shows whether the lines it points to changed.

## Performance

- Large files are rendered virtually (only visible lines).
- Syntax highlighting runs lazily per visible range and is cached.
- Switching files must stay within the one-frame budget ([03](03-performance.md)), excluding the fade.
