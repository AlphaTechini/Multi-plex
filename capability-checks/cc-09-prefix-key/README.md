# CC-09 · Which prefix key is free?

## What it tests

Multiplex needs one default prefix key (like tmux's `Ctrl+B`) for its own actions. Keys without the prefix go to the hosted agent. The default must:

1. not be bound by any supported agent CLI (Claude Code, opencode, OpenAI Codex CLI), and
2. reach the application in every target terminal (Windows Terminal, Ghostty, iTerm2, WezTerm, Kitty, Alacritty, VS Code and Zed terminals), and
3. decode to one unambiguous key in Bubble Tea v2.

## How

Desk research, done on 2026-10-30:

- **Claude Code 2.1.314**: the official docs `https://code.claude.com/docs/en/keybindings.md` (default binding tables, reserved shortcuts, chords) and `interactive-mode.md`.
- **opencode**: `packages/web/src/content/docs/keybinds.mdx` on the `dev` branch of `sst/opencode` (the source of `opencode.ai/docs/keybinds`, which was not reachable from the research machine).
- **Codex CLI**: `codex-rs/tui/src/keymap.rs` (`built_in_defaults`) on the `main` branch of `openai/codex`. Codex's own docs site was not reachable, so the key map comes from the source.
- **Windows Terminal**: `src/cascadia/TerminalSettingsModel/defaults.json` and `userDefaults.json` on `main` of `microsoft/terminal`.
- **Decoding**: each candidate's legacy byte and its Kitty-protocol form were fed through `ultraviolet.EventDecoder`, the decoder Bubble Tea v2.1.1 uses. `0x1c` and `CSI 92;5u` both decode to `ctrl+\`. `0x00` and `CSI 32;5u` both decode to `ctrl+space`. `0x1d` decodes to `ctrl+]`.

Delivery in each terminal was **not** tested by hand. This README records the research only.

## Result

Bindings of the candidates (✗ = taken, ✓ = free):

| Candidate | Byte | Claude Code | opencode | Codex | Windows Terminal | Other known conflicts |
|---|---|---|---|---|---|---|
| `Ctrl+B` | 0x02 | ✗ background task | ✗ cursor left | ✗ cursor left | ✓ | tmux prefix |
| `Ctrl+X` | 0x18 | ✗ chord prefix (`Ctrl+X Ctrl+K/E/S/B`, `Ctrl+X Enter`, ...) | ✗ leader | ✓ | ✓ | |
| `Ctrl+A` | 0x01 | ✗ line start | ✗ line start / provider list | ✗ line start | ✓ | GNU screen prefix |
| `Ctrl+G` | 0x07 | ✗ external editor | ✗ first message | ✗ external editor | ✓ | |
| `Ctrl+]` | 0x1d | ✓ | ✓ | ✗ skip question | ✓ | telnet escape |
| `Alt+M` | ESC m | ✗ cycle mode (Windows without VT input) | ✓ | ✓ | ✓ | macOS needs Option-as-Meta, otherwise types `µ` |
| `Ctrl+Space` | 0x00 | ✓ | ✓ | ✓ | ✓ (`Ctrl+Shift+Space` and `Alt+Space` are bound, not this) | **macOS "Select the previous input source" shortcut** (on by default, takes the key when 2+ input sources exist); toggles some Chinese/Japanese IMEs on Windows; VS Code terminal may route it to its suggest widget (unverified) |
| `Ctrl+\` | 0x1c | ✓ | ✓ | ✓ | ✓ | SIGQUIT only in cooked mode (Multiplex runs raw, so no signal); hard to type on layouts where `\` needs AltGr (German, French, Spanish, Nordic); VS Code binds `Ctrl+\` to "split editor" outside the terminal (whether the integrated terminal passes it through is unverified) |

Keys Claude Code reserves and can never be rebound: `Ctrl+C`, `Ctrl+D`, `Ctrl+M`, `Ctrl+[`, `Ctrl+I`, `Ctrl+H`. Other Claude Code defaults: `Ctrl+B Ctrl+G Ctrl+J Ctrl+L Ctrl+O Ctrl+R Ctrl+S Ctrl+T Ctrl+V Ctrl+_`, `Meta+P/O/T`, `Shift+Tab`, `Esc`, `Ctrl+Enter`, `Ctrl+Up/Down`, `Ctrl+Home/End`.

Windows Terminal default bindings that never reach an application: `Ctrl+Shift+` + `Space , . F P T N W D C V A M K 1-9 Up Down PgUp PgDn Home End Tab`, `Ctrl+Tab`, `Ctrl+Alt+1-9`, `Ctrl+Alt+Left`, `Alt+Arrow` (move pane focus), `Alt+Shift+Arrow`, `Alt+Shift+-`, `Alt+Shift+Plus`, `Alt+Shift+D`, `Alt+Enter`, `Alt+F4`, `Alt+Space`, `F11`, `Ctrl+,`, `Ctrl+Alt+,`, `Ctrl+Plus/Minus/0`, `Ctrl+Insert`, `Shift+Insert`, and `Ctrl+C` / `Ctrl+V` (from the settings file a new install writes: copy only when text is selected, paste always).

## Decision

**Default prefix: `Ctrl+\`.** No supported agent, Windows Terminal, macOS or tmux binds it. It is one byte (`0x1c`) in every terminal's legacy encoding and decodes as `ctrl+\` in Bubble Tea with or without the Kitty keyboard protocol.

- Pressing the prefix twice sends a literal `Ctrl+\` to the focused pane.
- **Documented alternative: `Ctrl+Space`**, for keyboard layouts where `\` needs AltGr. It is not the default because macOS uses it to switch input sources.
- Both are only defaults in the keymap file ([06](../../docs/06-ui-layout.md)).

**Still to verify by hand (pass condition of CC-09):** press `Ctrl+\` and `Ctrl+Space` inside a Bubble Tea v2 test program in Windows Terminal (main development platform, test first), then Ghostty, iTerm2, WezTerm, Kitty, Alacritty, the VS Code terminal and the Zed terminal, and record what arrives. Also test a UK and a German layout on Windows.
