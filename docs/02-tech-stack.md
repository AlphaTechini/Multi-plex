# 02 · Tech stack

Each decision lists the reason and what was rejected. Priorities from the README apply: speed, safety, review experience, multi-agent management, comfort. The author's fluency matters too: a stack he ships fast in beats a stack that is faster on paper.

## Decisions

| Layer | Choice | Why | Status |
|---|---|---|---|
| Daemon, TUI, hooks, MCP server | **Go 1.26** | Author is fluent. Starts in milliseconds, compiles in seconds, ships as one static binary, cross-compiles. Goroutines suit many PTYs and streams. | Decided |
| TUI framework | **Bubble Tea v2**, Lip Gloss v2, Bubbles v2 | Stable v2 released Feb 2026 with a renderer that only redraws what changed, keyboard enhancements (disambiguated keys), and production use in Charm's own coding agent, Crush. | Decided |
| PTYs | **`github.com/charmbracelet/x/xpty`** | One interface: Unix PTY on Unix, ConPTY on Windows. Note: on Windows the Go runtime cannot `Wait` on ConPTY processes, so use xpty's own wait helper (CC-01). | Decided, verify CC-01 |
| VT emulation | **libghostty-vt** (via cgo) or a pure-Go emulator | See below. | Open, CC-02, CC-03 |
| Storage | **SQLite via `modernc.org/sqlite`** | Pure Go, no cgo, so it does not complicate cross-compiling. | Decided |
| Socket on Windows | **`github.com/Microsoft/go-winio`** named pipes | Standard way to do local IPC on Windows in Go. | Decided |
| Syntax highlighting | **chroma v2** (`github.com/alecthomas/chroma/v2`) | Pure Go, many languages, theme support. | Decided for v1 |
| Diff hunks | **git** (`git diff --histogram`) plus our own word-level diff | git is always present. We only add the intra-line highlighting and presentation. | Decided |
| Structural diffs | **tree-sitter** (cgo) | Needed for "function moved/renamed" views. Adds cgo, so it waits until after v1. | Later |
| MCP server | Official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk`) | Official SDK, stable v1 line (v1.6.x as of Oct 2026), maintained with the MCP spec. | Decided |
| Windows Hello | **C++/WinRT helper executable** (`helpers/windows-hello/`) | Windows Hello's `UserConsentVerifier` is a native WinRT API. A ~100-line native helper is the simplest reliable path, keeps cgo out of the main binary, and is the one place where C++ is the natural language. It never parses untrusted input. | Decided, verify CC-05 |
| Touch ID (later) | Swift helper using LocalAuthentication | Native API on macOS. | Later |
| Linux approvals (later) | polkit / fprintd where present, password fallback | Biometric support on Linux is inconsistent. | Later |
| GUI (optional, later) | Tauri v2 + Svelte, with the Go daemon bundled as a sidecar | Only if the TUI cannot meet the visual bar. Tauri can bundle external binaries in any language, and v2 targets Android and iOS too, so the Svelte UI could be reused on mobile. | Not planned for v1 |
| Browser extension (later) | TypeScript, pnpm | Chrome extension plus native messaging ([12](12-browser.md)). | Later |

## VT emulator: the open decision

To show an agent's real terminal inside a pane, we must emulate a terminal. Bugs here appear as broken rendering of the agent's own UI, which is the most visible failure the tool can have.

| Option | For | Against |
|---|---|---|
| **libghostty-vt** | Ghostty's own parser and terminal state (cursor, styles, reflow, scrollback), proven in a widely used terminal. Zero dependencies, C API usable from Go. Supports the Kitty keyboard protocol. | Needs cgo and Zig 0.15.x in the build. API not fully stable yet. Debug builds are very slow (benchmark release builds only). Cross-compiling Go gets harder with cgo. |
| **Pure-Go emulator** | Keeps the build pure Go and trivially cross-compiled. | Less proven. Every missing escape sequence is a rendering bug we own. |

Decide after CC-02 (render fidelity with Claude Code, opencode, vim, btop) and CC-03 (libghostty-vt builds through cgo on Windows, macOS and Linux).

## Rejected, and why

### C++ for the core
- Performance is not the bottleneck for a wrapper. The work is moving bytes between PTYs and redrawing cells, which Go does in microseconds. What matters (startup, added latency) Go already meets.
- Compile times are not better than Rust's, often worse, and CMake plus package management on Windows adds friction.
- The core parses untrusted terminal output. Doing that in a memory-unsafe language the author is still learning, in a tool whose main promise is safety, is the riskiest combination available.
- Learning C++ for game development is a separate goal. The Windows Hello helper is where C++ fits naturally in this project.

### Rust for the core
- A valid choice (herdr is Rust). The author has Rust exposure from Substrate, ink! and Solana.
- Rejected because shipping speed for this author is higher in Go, and Rust's longer clean builds slow the iteration loop.

### Tauri now
- Tauri builds desktop GUI apps with a system web view. It does not run inside a terminal, so it cannot meet the "works in any terminal" requirement.
- It stays an option for a later GUI on top of the same daemon.

### Electron
- Too heavy for the "light like Zed" requirement.
