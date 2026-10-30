// Package config resolves where Multiplex keeps its files (data, config,
// runtime), the local socket address the daemon listens on, and user
// settings loaded from settings.json.
//
// Setting MPX_HOME puts every directory under one folder, which tests and a
// second instance use. Setting MPX_SOCKET overrides the socket address.
package config
