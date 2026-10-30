package config

// checkOwner is a no-op on Windows: the daemon uses a named pipe whose
// security descriptor already restricts it to the owner.
func checkOwner(dir string) error { return nil }
