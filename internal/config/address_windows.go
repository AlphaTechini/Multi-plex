package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// defaultSocketAddress is the named pipe for the current user. When MPX_HOME
// is set, a short hash of it is added so a second instance does not collide.
func defaultSocketAddress(Paths) string {
	name := "multiplex-" + sanitizeUser(currentUsername())
	if home := os.Getenv(EnvHome); home != "" {
		sum := sha256.Sum256([]byte(filepath.Clean(home)))
		name += "-" + hex.EncodeToString(sum[:4])
	}
	return `\\.\pipe\` + name
}

// currentUsername returns the account name, falling back to the environment.
func currentUsername() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		// Windows reports DOMAIN\name; only the name is used.
		if i := strings.LastIndexByte(u.Username, '\\'); i >= 0 {
			return u.Username[i+1:]
		}
		return u.Username
	}
	return os.Getenv("USERNAME")
}
