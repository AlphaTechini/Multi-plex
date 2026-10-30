# mpx-hello (Windows Hello helper)

Small C++/WinRT console program the Multiplex daemon runs to ask the human to
approve a gated action with Windows Hello (fingerprint, face or PIN). See
`docs/04-safety.md` (Layer 5) and capability check CC-05.

```
mpx-hello.exe check
mpx-hello.exe verify --message "Allow git push --force?" [--timeout 60]
```

It prints one JSON line, `{"result":"...","detail":"..."}`, and exits with:

| Exit | Meaning | `result` values |
|---|---|---|
| 0 | verified / available | `verified`, `available` (check) |
| 1 | user canceled | `canceled` |
| 2 | not usable | `not_available`, `not_configured`, `disabled_by_policy`, `device_busy` |
| 3 | error | `error`, `retries_exhausted` |
| 4 | timeout | `error` with detail `timeout` |

It never reads stdin and only parses its own arguments. The dialog text is cut
to 256 characters; the full command is shown in the Multiplex UI.

## Prerequisites

- Windows 10 1809+ or Windows 11, with Windows Hello set up (Settings, Accounts, Sign-in options).
- Visual Studio 2022 Build Tools with "Desktop development with C++" and a Windows SDK 10.0.22621 or newer (C++/WinRT headers ship in the SDK).
- CMake 3.20+ (optional, only for the CMake build).

## Build

Developer PowerShell for VS:

```powershell
cd helpers\windows-hello
.\build.ps1                 # output: build\mpx-hello.exe
```

or with CMake:

```powershell
cmake -S . -B build -A x64
cmake --build build --config Release   # output: build\Release\mpx-hello.exe
```

The runtime is linked statically (`/MT`); no redistributable is needed.

## Where Multiplex looks for it

1. `MPX_HELLO_PATH` (full path to the exe), if set.
2. `mpx-hello.exe` in the same folder as `mpx.exe`.

If neither exists, Windows Hello is reported unavailable and Multiplex falls back
to the typed one-time code.

## How the prompt gets in front

A console process has no window, and `UserConsentVerifier.RequestVerificationAsync`
takes no owner, so the dialog can open behind the terminal. The helper uses the
`IUserConsentVerifierInterop::RequestVerificationForWindowAsync` overload with an
owner window: the console window if it is visible (classic conhost), otherwise a
tiny transparent topmost tool window it creates itself (Windows Terminal and VS
Code use ConPTY, whose console window is hidden). It then calls
`AllowSetForegroundWindow`/`SetForegroundWindow` best effort. CC-05 verifies this on
real hardware; see `capability-checks/cc-05-windows-hello/`.
