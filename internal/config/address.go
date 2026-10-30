package config

import (
	"os"
	"strings"
)

// EnvSocket overrides the daemon's socket address (a path on Unix, a pipe
// name on Windows).
const EnvSocket = "MPX_SOCKET"

// SocketAddress returns the address the daemon listens on and clients dial.
// It is a Unix socket path on macOS and Linux, and a named pipe on Windows.
// MPX_SOCKET takes precedence over the default.
func SocketAddress(p Paths) string {
	if addr := os.Getenv(EnvSocket); addr != "" {
		return addr
	}
	return defaultSocketAddress(p)
}

// sanitizeUser keeps letters, digits, dot, dash and underscore, and replaces
// anything else with an underscore, so the name is safe in a pipe path.
func sanitizeUser(name string) string {
	if name == "" {
		return "user"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
