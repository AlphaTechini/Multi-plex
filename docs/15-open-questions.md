# 15 · Open questions

Decisions not made yet. When one is decided, record the decision and the reason here and update the doc it affects.

| # | Question | Options | Decided by | Status |
|---|---|---|---|---|
| Q1 | Which VT emulator? | libghostty-vt via cgo, or a pure-Go emulator | CC-02, CC-03 | Open. v1 uses pure-Go `charmbracelet/x/vt` behind `internal/vt`. Linux findings (capability-checks/cc-02-vt): renders git, ls, top and vim correctly, but scrolling floods run at ~1.3 MB/s (a 10 MB `cat` takes seconds, missing the docs/03 budget) and wide characters at the right margin are mishandled. Redraw-in-place agent TUIs are unaffected. Fix upstream or switch to libghostty-vt via CC-03 |
| Q2 | Default prefix key? | Must not clash with Claude Code, opencode or Codex keys, and must reach the app in every target terminal | CC-09 | Proposed: `Ctrl+\` (free in Claude Code, opencode, Codex, Windows Terminal, tmux, macOS); `Ctrl+Space` as the alternative. Terminal delivery still to verify by hand |
| Q3 | Client/daemon message encoding? | JSON for commands; JSON or a binary format for cell streams | Bench of cell-stream size and CPU in Phase 1 | Open |
| Q4 | Which agent CLIs besides Claude Code in v1? | opencode, Codex, none | Author's daily use | Open |
| Q5 | Windows write restriction approach? | Agents in WSL2 with Claude Code's sandbox, low integrity level, AppContainer, or hook layer only | CC-11 | Open |
| Q6 | Which parts does the author hand-write instead of delegating to agents? | For example PTY hosting, the safety layer, the diff engine | Author | Open |
| Q7 | Final product name and binary name? | Working names: Multiplex, `mpx` | Author | Open |
| Q8 | Default agent panel width? | ~30% with a 60-column minimum, resizable | Phase 1 use | Proposed |
| Q9 | Default landing strategy? | Merge commit, squash, rebase, or PR through `mpx push` | Author | Open |
| Q10 | Setup recipe format? | TOML or YAML in `.multiplex/` | Phase 2 | Open |
| Q11 | Later GUI? | Stay TUI-only, or Tauri v2 + Svelte on the same daemon | After Phase 2, if the TUI diff viewer misses the visual bar | Open |
