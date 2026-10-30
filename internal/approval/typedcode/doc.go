// Package typedcode is the fallback approval method when the machine has no
// OS-level one (no Windows Hello, no polkit agent). The daemon issues a random
// six-character code and shows it ONLY in the TUI approval modal; the human
// types it back to approve.
//
// What it proves, honestly: that whoever answered could see the Multiplex UI
// and use its keyboard input. It is weaker than a biometric or an OS password
// prompt. A process running as the same user could in principle read the
// terminal (or the daemon's memory) and submit the code, which is the limit
// docs/04-safety.md calls out for a plain in-terminal "Accept". It does stop
// the common cases: an agent cannot approve by typing "y" into its own PTY,
// because the code is never written to any agent's pane, log or tool output,
// and the code is single-use, expires after five minutes, and allows three
// attempts.
//
// Never log or persist codes; callers must hand them only to the UI client.
package typedcode
