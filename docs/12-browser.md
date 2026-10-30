# 12 · Browser access

## Decision

**No built-in browser.** Browser access goes through an extension in the user's real browser.

Why:

- A built-in browser either blocks things a developer needs to test (cookies, tracking-dependent flows) or becomes a full browser to maintain.
- Built-in browsers cannot run the user's extensions (wallets, auth helpers, dev tools), which some apps need to work.
- The browser should get the full window, not a pane inside a terminal.
- Using the user's real profile means real sign-ins and extensions.

## v1: reuse what exists

Claude Code already has a Chrome integration through the Claude in Chrome extension. In wrapper mode, sessions use that. Multiplex only adds an **"open in browser"** action that opens the app's URL (for example the session's dev server port from [05](05-sessions-and-worktrees.md)) in the user's browser.

## Later: our own extension (for the native agent)

- A Chrome extension plus a **native messaging host** registered by `mpx`, so the extension talks to the daemon without opening a network port.
- Capabilities exposed to agents as MCP tools: open URL, read console logs, read network errors, take a screenshot, read the page's DOM text, click and type (gated for anything that submits forms on sites outside a local dev allow-list).
- Each session gets its own browser tab group, labelled with the session name.

## Not using remote debugging on the main profile

Driving Chrome through its remote debugging port is an alternative, but since Chrome 136 the `--remote-debugging-port` and `--remote-debugging-pipe` switches are ignored for the default data directory. They only work with a separate `--user-data-dir`, which has none of the user's sign-ins or extensions. (A manual toggle in `chrome://inspect` can still enable debugging on a real profile, but that is a per-session manual step.) The extension route avoids all of this.
