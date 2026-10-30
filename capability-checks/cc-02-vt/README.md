# CC-02 · Which VT emulator renders agent TUIs correctly?

## What it tests

Whether the pure-Go emulator (`github.com/charmbracelet/x/vt`, wrapped by `internal/vt/charmvt`) draws the programs Multiplex hosts without visible glitches in colours, cursor, wide characters and scrolling, and whether it is fast enough (docs/03: keep up with `cat` of a 10 MB log).

The full check compares recorded sessions of Claude Code, opencode, vim and btop with Windows Terminal, and must be done on the author's machine. This directory holds the tool that makes the comparison repeatable, and the Linux results that could be obtained here.

## How

```
go run ./capability-checks/cc-02-vt record -o out.bin [-size 100x30] [-send '"\x1b"' -send-after 1s] [-timeout 30s] -- <cmd> [args...]
go run ./capability-checks/cc-02-vt replay [-size 100x30] [-ansi] out.bin
```

- `record` runs the command in a PTY (`internal/pty/xptyimpl`, `TERM=xterm-256color`) and writes the raw bytes it prints to `out.bin`. A throwaway emulator answers the program's terminal queries (device attributes, cursor position), so full-screen programs start normally. `-send` types a Go-quoted string once after `-send-after` (to quit vim, for instance); the program is killed after `-timeout`.
- `replay` feeds the file through `charmvt` using the same frame builder the daemon uses (`term.Replay`), prints the final screen as text (or with colours using `-ansi`), the cursor, alt-screen state and title, and the cost of building frames. It fails if any row is not exactly the screen width.

Replay at the size the recording was made at. To compare with Windows Terminal, record on Windows (same tool), then replay the file here and look at both side by side, or replay with `-ansi` in the terminal under test.

## Result

### Linux (research container, Go 1.26.10, x/vt v0.1.0)

| Recording | Result |
|---|---|
| `git --no-pager log --graph --color=always -n 20 --stat` | Correct: graph glyphs, 16-colour and bold SGR, long lines wrapped at the margin, scroll to the last 30 rows. |
| `ls --color=always -la /` | Correct: bold blue and cyan names, the sticky-directory background colour on `tmp`, alignment. |
| `top -n 1` (`TERM=xterm-256color`) | Correct: bold fields, reverse-video header row, 100 columns of cursor-addressed output. |
| `vim -n -u NONE -N -c 'set number' -c 'syntax on'` (`LANG=C.UTF-8`), typing `hello wide 世界 😀 end` | Correct: alternate screen entered, line numbers coloured, wide CJK and emoji at the right columns, cursor position right. `:q!` returns to the (blank) main screen. |
| `seq 1 400000` (3.1 MB of short lines) | Final screen correct, **but replay takes about 12 s** (see below). |
| `htop`, `btop` | Not installed in the research container. Pending. |

Rendering of every row stayed exactly the screen width (`ansi.StringWidth(Line(y)) == cols`), including after random input and resizes (fuzzed in `internal/vt/charmvt`).

Not recorded here (pending on the author's machine): Claude Code, opencode, btop, and the side-by-side comparison with Windows Terminal.

### Findings about x/vt v0.1.0

1. **Scrolling is slow.** A line feed at the bottom of the screen copies the whole screen cell by cell (`uv.Buffer.DeleteLineArea`; a cell is about 130 bytes). Measured on this machine, feeding the emulator alone:

   | Screen | Throughput on scrolling text |
   |---|---|
   | 80x24 | about 2.4 MB/s |
   | 120x40 | about 1.3 MB/s (10 MB synthetic log: 7.6 s) |
   | 100x30, `seq` | about 30 µs per scrolled line |

   Frame building is not the problem (p50 about 0.1 ms, p99 under 0.5 ms per frame). The pane host keeps the UI and keystrokes responsive during a flood (reader, frame loop and input are separate; frames are dropped, never input), but the program writing to the PTY is held back by backpressure, so `cat bigfile` in a pane takes seconds instead of milliseconds. Cursor-addressed TUIs such as Claude Code, which redraw in place, do not scroll the screen and are not affected. A fix would be upstream in ultraviolet: rotate the line slices instead of copying cells (O(rows) instead of O(rows x cols)). Until then this is the main performance argument for libghostty-vt (CC-03).
2. **Wide characters at the right margin are mishandled.** A wide character that ends exactly in the last column does not set the pending-wrap state, so the next character overwrites it; one that does not fit is dropped instead of wrapped to the next row (for example `abc世界` in 5 columns shows `abc  `). `charmvt.Line` keeps the row exactly `cols` wide whatever the library leaves, but cannot restore the lost character.
3. **A few sequences panic the library** (for example a scroll region taller than the screen followed by a scroll). `charmvt.Write` recovers, resets the scroll region (otherwise every later scroll would panic too) and returns an error, so the daemon survives; the rest of that chunk is lost.
4. x/vt does not answer the Kitty keyboard query (`CSI ? u`); `charmvt` answers "no flags" itself. It also has no Kitty keyboard protocol towards the hosted program, so Shift+Enter is forwarded as `ESC CR` (`internal/keys`).

## Recommendation

**Pure-Go x/vt for v1** (keeps the build free of cgo and the toolchain simple), because everything recorded here renders correctly, the Windows path needs no extra toolchain, and the neutral `internal/vt` interface lets libghostty-vt replace it later without touching callers.

Revisit if the author's side-by-side comparison finds glitches in Claude Code, opencode or btop, or if flood speed (finding 1) matters in practice: the options are a patch to ultraviolet's line deletion, or CC-03 (libghostty-vt). The measured numbers above are the baseline to beat.
