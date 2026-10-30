# CC-05 · Can a console app trigger Windows Hello reliably?

From `docs/13-capability-checks.md`:

> C++/WinRT helper calling `UserConsentVerifier`, launched from a terminal.
> Pass: prompt appears in front; success, cancel, timeout and "not available" are
> distinguishable; latency recorded.

The helper is `helpers/windows-hello/` (`mpx-hello.exe`). It could not be compiled
or run where it was written (Linux), so everything below is still to be proven.

## What it tests

1. The helper compiles with the Windows SDK C++/WinRT headers and the
   `IUserConsentVerifierInterop` interface.
2. The Hello dialog opens **in front** when launched from a console process with
   no window of its own (the helper uses the console window if visible, else its
   own transparent topmost owner window).
3. Every outcome is distinguishable through the JSON line and exit code.
4. How long a verification takes from process start to JSON line.

## How to run (Windows 10/11 with Windows Hello set up)

1. Build, in a Developer PowerShell for VS:
   ```powershell
   cd helpers\windows-hello
   .\build.ps1          # produces build\mpx-hello.exe
   ```
2. Availability:
   ```powershell
   .\build\mpx-hello.exe check; $LASTEXITCODE
   ```
   Expect `{"result":"available",...}` and exit 0.
3. Verify, once from each terminal below (open the terminal, start another window
   in front of it, then run the command so focus is not already on the terminal):
   ```powershell
   Measure-Command { .\build\mpx-hello.exe verify --message "CC-05 test: git push --force" --timeout 60 | Tee-Object -Variable out }
   $out; $LASTEXITCODE
   ```
   - Windows Terminal (PowerShell tab)
   - VS Code integrated terminal
   - classic console host (`conhost`, `cmd.exe` from Win+R)
   - launched with no console: `Start-Process .\build\mpx-hello.exe -ArgumentList 'verify','--message','detached' -WindowStyle Hidden -RedirectStandardOutput out.txt`
4. For each terminal, exercise every outcome:

   | Case | How | Expect |
   |---|---|---|
   | verified | complete fingerprint/face/PIN | `verified`, exit 0 |
   | canceled | press Cancel or Esc | `canceled`, exit 1 |
   | timeout | `--timeout 5`, do nothing | `error` with detail `timeout`, exit 4, dialog closes |
   | retries exhausted | fail the biometric repeatedly | `retries_exhausted`, exit 3 |
   | not available | run on a VM or machine without Hello, or `check` after disabling Hello | `not_available` / `not_configured` / `disabled_by_policy`, exit 2 |
   | bad args | `mpx-hello.exe verify --bogus x` | `error`, exit 3 |

5. Also try: a long message (300+ characters; the helper truncates to 256), a
   message with quotes and non-ASCII text, 10 runs in a row for latency, and
   running while another Hello prompt (lock screen, sign-in) is up (`device_busy`?).

## Record

| Question | Windows Terminal | VS Code terminal | conhost | detached |
|---|---|---|---|---|
| Prompt appeared in front without clicking? | | | | |
| Prompt took keyboard focus? | | | | |
| Latency to verified (median of 10) | | | | |
| Cancel / timeout / not-available distinguishable? | | | | |
| Owner window left behind after exit? | | | | |

Also record: Windows build number, Windows SDK version, Hello method used (PIN,
fingerprint, face), and whether the transparent owner window ever flashes visibly.

If the prompt still opens behind the terminal, things to try, in order:
`SetForegroundWindow` after a synthetic Alt key press (the classic focus-stealing
workaround), having the daemon call `AllowSetForegroundWindow(pid)` for the
helper before it starts, or parenting the dialog to `GetForegroundWindow()`.

## Result

Not run yet (needs Windows hardware).
