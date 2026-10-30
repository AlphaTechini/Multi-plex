# 04 · Safety

## Goal

Agents can read broadly and work freely, but **nothing irreversible happens without a human-present approval**. This is not a sandbox that cuts the agent off from the machine. It is a set of controls on what the agent can change permanently.

## The rule

> If an action is reversible, allow it. If it is not reversible, stop it until a human approves it.

The design work is making as much as possible **reversible**, so approvals are rare. If the gate fires often, people approve by reflex and it protects nothing.

## Threat model

What we defend against:

- **Agent mistakes**: deleting the wrong folder, force-pushing, dropping a database, running `terraform destroy` against the wrong workspace.
- **Prompt injection**: instructions hidden in repository files, web pages, issue text, dependencies or installed skills that steer the agent into harmful actions.
- **Self-approval**: an agent approving its own request, for example by typing "y" into a confirmation prompt in its own terminal.

What we do not try to defend against (out of scope):

- A malicious local user, malware already on the machine, or anyone with admin/root.
- A compromised agent CLI binary.

## Why command matching alone fails

Blocking `rm -rf` by matching the command text is easy to get around, accidentally or not: `python -c "import shutil; shutil.rmtree(...)"`, `find . -delete`, `Remove-Item -Recurse`, a script the agent writes and then runs, an alias. Command classification is useful as a **fast first filter**, never as the only line of defence.

## Layers

### Layer 1 · Reversible by default (local files)

- Every session works in its **own git worktree** ([05](05-sessions-and-worktrees.md)).
- Before each shell command and at each turn end, the daemon records a **checkpoint**: a commit object of the worktree's tracked and untracked (non-ignored) files, stored under `refs/multiplex/checkpoints/<session>/<n>` using a temporary index so the agent's staging area is untouched.
- Any change inside the worktree can be rolled back to a checkpoint. These actions need **no prompt**.
- **Known gap**: files ignored by `.gitignore` (for example `.env`, local databases, build output) are not captured. Secrets should not live in worktrees at all (Layer 3). Other important ignored files can be added to a per-workspace "protected paths" list, which the hook layer gates.

### Layer 2 · Writes outside the worktree are gated

- The agent can **read** anywhere it normally could.
- **Writes** outside its worktree (and a small allow-list such as temp and tool caches) need approval.
- Enforcement:
  - **Hook layer** (all platforms, v1): file-editing tool calls with paths outside the worktree are sent to the gate.
  - **OS layer**: on macOS and Linux, the agent CLI's own sandbox where it has one (Claude Code's, see "Using the agent CLI's own sandbox" below). Windows has no equivalent in Claude Code. Candidates there are WSL2, a **low integrity level** process with the worktree labelled writable, or an **AppContainer**. The last two may break common tools (npm caches, `%APPDATA%`). Research item, CC-11.

### Layer 3 · Credentials are withheld, so remote actions cannot happen silently

Remote side effects (`git push --force`, `aws s3 rm`, `kubectl delete`, `terraform destroy`, `DROP TABLE` against a real database) cannot be undone with a local snapshot. The strongest control is that the agent **does not hold the keys** until a human approves.

- Agents start with a **scrubbed environment**: known credential variables removed (`AWS_*`, `GOOGLE_APPLICATION_CREDENTIALS`, `GH_TOKEN`, `GITHUB_TOKEN`, `KUBECONFIG`, database URLs, and a user-editable list).
- **Credential stores are on a deny-read list** for agent file tools: `~/.aws`, `~/.config/gcloud`, `~/.ssh`, `~/.kube`, `~/.docker/config.json`, browser profile folders, password manager data. Enforced by the hook layer, and by the OS layer where available.
- **Git pushes go through a broker.** Each session's worktree gets a per-worktree `pushurl` that does not work directly, set in the worktree's config file inside the main repository's `.git` folder (outside the worktree, so the agent cannot edit it under Layer 2). Pushing happens through `mpx push`, which asks for approval and then pushes with the user's own credentials. Verify in CC-14.
- **Approved credential use is scoped**: approving one `terraform apply` injects credentials for that one command, not for the rest of the session.
- The agent's git configuration is launched with credential helpers disabled, and any `git push` or `git config` change to remotes or credential settings is gated by Layer 4. Layer 4 gates every `git push`, not only force pushes: a push publishes commits, which no local checkpoint can undo, so pushes go through `mpx push`.

### Limits of Layers 2 and 3 without OS isolation

Be honest about what holds without OS-level isolation (especially on Windows until CC-11 is solved): an agent process runs as the same user, so a determined or injected command can still reach credentials the user's account can reach (for example Windows Credential Manager, which Git Credential Manager uses), or pass an explicit URL to `git push`. Without OS isolation, Layers 2 and 3 reliably stop **mistakes and casual injection**, not a deliberate bypass. Full protection needs the OS layer, or keeping long-lived credentials out of reach of the user account that runs agents.

### Layer 4 · Command classification (fast path)

Every shell tool call is classified before it runs:

| Class | Examples | Action |
|---|---|---|
| Known safe | `ls`, `git status`, `go test`, `pnpm install` inside the worktree | Allow, no prompt |
| Known dangerous | `git push --force`, `git reset --hard` on shared branches, recursive deletes outside the worktree, `terraform destroy`, cloud CLI delete verbs, `DROP`/`TRUNCATE` | Gate |
| Unknown | anything else | Allow, relying on Layers 1 to 3 |

The rules are data (a file in `.multiplex/` plus user defaults), so users can extend them.

### Layer 5 · Human-present approval

When an action is gated, the user approves with:

| Platform | Method |
|---|---|
| Windows | **Windows Hello** (fingerprint, face or PIN) through `helpers/windows-hello` (CC-05) |
| macOS | **Touch ID** through a LocalAuthentication helper (later) |
| Linux | polkit or fprintd where available, otherwise the user's password (later) |
| Remote / cloud session | Approval on the **phone**, signed with a passkey protected by Face ID or fingerprint (later, [11](11-remote-mobile-cloud.md)) |
| No biometric hardware | OS password or PIN |

A biometric check is something an agent cannot fake. A plain "Accept" key in the same terminal the agent controls is weaker than it looks.

The approval prompt shows:

- the exact command or file operation, with the working directory;
- why it was gated (which rule or layer);
- for file operations, which files are affected;
- options: **approve once**, **deny**, **deny with a message to the agent**. Session-wide approvals for a pattern are possible but off by default and shown clearly while active.

## Wrapper mode enforcement (v1, Claude Code)

1. Multiplex writes the hook configuration that sends Claude Code's `PreToolUse` events to `mpx hook pre-tool`.
2. `mpx hook` forwards the tool name and input to the daemon.
3. The daemon runs Layers 2 to 4. If gated, it runs Layer 5 and waits.
4. `mpx hook` returns Claude Code's decision in `hookSpecificOutput` (`permissionDecision`: `allow`, `deny` or `ask`; `defer` only applies in non-interactive `-p` mode, plus `permissionDecisionReason`). Exit code 2 also blocks.

What Claude Code's hook docs say (checked Oct 2026), and what it means for us:

- **Hooks fail open by default.** The default timeout for command hooks is 600 seconds, and a timed-out `PreToolUse` hook does **not** block the tool call unless the hook sets `"onFailure": "block"`. Our hook configuration must always set `onFailure: block`, and `mpx hook` must deny if it cannot reach the daemon. Fail closed.
- **Hooks can rewrite tool input** (`updatedInput`). We can use this to route an approved command through `mpx exec`, which injects scoped credentials for that one command (Layer 3).
- **Hook settings are live.** Edits to hook settings files are picked up automatically by a file watcher. An agent that edits a settings file could remove our hook mid-session. File tools touching any Claude Code settings file are gated, and Layer 4 gates shell commands that write to them.
- **Project settings can disable user-level hooks.** Claude Code's docs (checked Oct 2026) say `"disableAllHooks": true` in a repository's `.claude/settings.json` outranks user settings. So our hooks do **not** live in the user's `~/.claude/settings.json`. Each session is launched with `claude --settings <file>`, where the file sits in the daemon's data directory (outside every worktree) and holds our hooks plus `"disableAllHooks": false`. Command-line settings outrank project and local settings and merge with the user's own hooks. CC-06 must confirm that a project-level `disableAllHooks: true`, set before or during a session, cannot turn our hooks off.
- **Hook commands use exec form** (`command` plus `args`), so no shell parses them on any OS, and on Windows the shell matcher is `Bash|PowerShell`.
- **`PermissionRequest`** fires the moment Claude Code is about to show its own permission prompt; it is the "blocked" signal (the `Notification` hook fires only after a delay). A user interrupt produces no `Stop` event.

## Using the agent CLI's own sandbox (macOS, Linux)

Claude Code has a built-in sandbox on **macOS, Linux and WSL2** (off by default). It limits writes to the working directory, temp and added directories, keeps reads broad, blocks network except allowed domains, supports `denyRead` lists, and protects its own settings and hook files from writes even inside allowed folders. Deny entries can only narrow access, never be removed by another scope.

On macOS and Linux, Multiplex turns it on for every session, with the worktree as the working directory and the credential stores from Layer 3 on `denyRead`. That gives us OS-level enforcement of Layers 2 and 3 in v1 without building our own.

**It does not run on native Windows**: there Claude Code runs commands unsandboxed. Options for Windows (CC-11, Q5):

1. Run agents inside **WSL2**, launched from the Windows daemon (`wsl claude ...` in a ConPTY), with worktrees on the WSL filesystem. Gets the sandbox, costs some Windows-native workflow.
2. Our own Windows restriction (low integrity level or AppContainer).
3. Hook layer only, accepting the limits described above.

## Approval fatigue

Track approvals per hour per session. If a workspace regularly exceeds a few per hour, the rules are wrong. Show it in the UI and suggest making the frequent action reversible or allow-listed.

## Audit log

Every gated action, decision, method (biometric, password) and timestamp goes into the SQLite log. Agents cannot read or modify it.
