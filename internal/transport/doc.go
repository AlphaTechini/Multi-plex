// Package transport is the neutral local IPC layer between Multiplex clients
// and the daemon. Callers use Listen and Dial. A Unix domain socket is used on
// macOS and Linux, and a named pipe (github.com/Microsoft/go-winio) on Windows.
// See docs/01-architecture.md "Client to daemon protocol".
package transport
