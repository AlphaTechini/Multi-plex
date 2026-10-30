# CC-10 · Conflict prediction with `git merge-tree`

## What it tests

Whether `git merge-tree --write-tree` on the author's Git:

1. reports a clean merge as clean (exit 0, no conflicted paths, a valid tree object),
2. lists exactly the conflicting files for a conflicting pair (content, add/add, modify/delete, and a path with a space), and
3. leaves the repository untouched: HEAD, all refs, the index, `git status` output and every working-tree file, even with uncommitted and untracked changes present.

Conflict prediction between sessions ([05](../../docs/05-sessions-and-worktrees.md)) is built on this.

## How

```
go run ./capability-checks/cc-10-merge-tree
```

Standard library only, no shell, works on Windows, macOS and Linux. Needs `git` 2.38 or later on `PATH`.

The program creates a throwaway repository in the system temp directory and deletes it afterwards. It isolates git config (`GIT_CONFIG_NOSYSTEM=1`, `GIT_CONFIG_GLOBAL` set to the null device) so commit signing or hooks in the user's config cannot interfere. It runs:

```
git merge-tree --write-tree --name-only --no-messages -z <ours> <theirs>
```

- `-z` makes the output `<tree-oid> NUL <path> NUL <path> NUL ...` so paths with spaces or non-ASCII characters are not quoted.
- `--name-only` lists each conflicted path once.
- `--no-messages` drops the informational section.
- Exit status: 0 is clean, 1 is conflicts, anything else is an error.

Before and after, it hashes HEAD, `for-each-ref`, `status --porcelain=v1 -z`, `.git/index` and every working-tree file, and compares the hashes.

## Result

| Platform | Git | Result | Time per merge-tree call |
|---|---|---|---|
| Linux (6.18, research container) | 2.43.0 | **PASS** | 4–5 ms (tiny repo) |
| Windows (author's machine) | not run yet | — | — |
| macOS | not run yet | — | — |

Linux output:

```
git version 2.43.0
ok   a+c: clean=true conflicts=[] tree=5dd49a5c0657 (4ms)
ok   a+b: clean=false conflicts=["added-both.txt" "deleted-on-b.txt" "dir with space/notes.md" "shared.txt"] want [...] (5ms)
ok   working tree, index, HEAD and refs unchanged
PASS
```

## Decision

Use `git merge-tree --write-tree --name-only --no-messages -z` for conflict prediction. Treat exit 1 as "conflicts" and any other non-zero exit as an error. Require Git 2.38 or later and check the version at startup.

Notes:

- merge-tree writes new blob and tree objects into the object database. They are unreferenced and `git gc` removes them later. Refs, the index and the worktree are not touched.
- Without `-z`, paths with special characters are C-quoted, so always pass `-z`.

**Still open:** run on Windows with the author's Git for Windows and record the version and result in the table. That is the pass condition for this check.
