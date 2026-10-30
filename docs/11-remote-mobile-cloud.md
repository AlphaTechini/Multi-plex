# 11 · Remote, mobile and cloud

The daemon design ([01](01-architecture.md)) is what makes all of this possible: sessions live in the daemon, and any number of clients attach to it.

## Stages

### Stage 1 · Remote attach over SSH

- `mpx attach --remote ssh://user@host` opens an SSH connection, starts or finds the remote daemon, and forwards its socket.
- The local TUI renders; only changed cells cross the network. This is lighter and less laggy than rendering a whole remote TUI over SSH.

### Stage 2 · Network API

- The same protocol over a WebSocket, for clients that are not a terminal.
- **Pairing**: the daemon shows a QR code; the phone scans it and registers a device key. Every request is signed with that key. No passwords.
- Reachability: over a private network such as Tailscale, or an SSH tunnel. No public port by default.

### Stage 3 · Mobile app

What the phone needs, in order:

1. Session list with states, so you see what is blocked or done.
2. Read summaries and insight cards.
3. **Approve gated actions** with a passkey protected by Face ID or fingerprint. The daemon sends a challenge; the phone signs it. This is also how approvals work when the daemon runs in the cloud, where no laptop sensor is present.
4. Send steer messages.
5. Read-only diffs.

Technology to decide later: Tauri v2 mobile with a Svelte UI (shared with a possible desktop GUI) or native apps.

### Stage 4 · Cloud sessions

- A cloud session is the daemon running on a server, with the workspace cloned there.
- **Moving a live session from laptop to cloud is not realistic**: the agent CLI's internal state lives in that machine's process and files. What moves is the **work**: the branch, the spec, the latest summary and open notes. A new agent session on the cloud daemon starts from that context.
- Credentials on the cloud machine follow the same rules as locally ([04](04-safety.md)). Approvals go to the phone.

## Security notes

- The network API is off by default.
- Every client action that would be gated locally is gated remotely too.
- Device keys can be listed and revoked from the TUI.
