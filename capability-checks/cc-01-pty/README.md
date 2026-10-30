# CC-01 · Host a shell in a PTY or ConPTY from Go

## What it tests

Whether Multiplex can host an interactive program in a pseudo-terminal from Go on every target OS, through `internal/pty/xptyimpl` (which wraps `github.com/charmbracelet/x/xpty`: Unix PTY on Linux and macOS, ConPTY on Windows):

1. spawn the platform shell (`pwsh -NoProfile` on Windows, `/bin/sh` elsewhere) with no `sh -c`,
2. write a command and Enter, read the output back,
3. resize the terminal and have the shell report the new size,
4. exit with code 7 and have `Wait` return exactly 7 (on Windows through xpty's wait helper, since `exec.Cmd.Wait` does not work with ConPTY),
5. `Close` ends the reader goroutine (on ConPTY the output pipe only ends when the console is closed).

The same package is what the daemon uses for agent panes, so a pass here covers the PTY layer of CC-01. Hosting `claude` itself (the second half of the check in docs/13) is still to do by hand once the adapter is wired into the daemon.

## How

```
go run ./capability-checks/cc-01-pty
```

It prints one line per step with the time it took, then `CC-01 PASS` or `CC-01 FAIL` (exit status 1). No arguments.

The typed command is `echo hel""lo` (`echo ('hel'+'lo')` in PowerShell) so that the terminal's own echo of the keystrokes cannot be mistaken for the program's output. The size is read with `stty size` (Unix) or `[Console]::WindowHeight/WindowWidth` (Windows).

## Result

| Platform | Result | Spawn | Echo round trip | Resize round trip | Exit and Wait |
|---|---|---|---|---|---|
| Linux (6.18, research container, Go 1.26.10) | **PASS** | about 1 ms | about 6 ms | about 6 ms | about 1 ms |
| Windows 11 (author's machine, ConPTY, pwsh) | not run yet | | | | |
| macOS | not run yet | | | | |

Notes from implementing it on Linux, which the Windows run should confirm or refute:

- Closing the parent's copy of the PTY slave right after `Start` is required: without it, reads on the master never end after the child exits.
- A read on the master that is blocked cannot be woken by closing the master (Go keeps the descriptor in blocking mode once `Fd()` was called inside the PTY library). `xptyimpl` therefore makes `Close` kill the child first, like a real hangup, so the blocked `Read` returns.
- xpty's options are ignored in v0.1.4, so the size is set explicitly after creation.
- On Windows the shell must be started so that ConPTY can run it: a real `.exe`. `.cmd` shims (npm's `claude.cmd`) are wrapped in `cmd.exe /c` by `xptyimpl`; prefer the native `claude.exe`.
