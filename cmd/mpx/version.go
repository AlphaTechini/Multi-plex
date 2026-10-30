package main

import (
	"fmt"
	"io"
	"runtime"
)

// version is set at build time: go build -ldflags "-X main.version=v0.1.0".
var version = "dev"

// runVersion prints the version line.
func runVersion(w io.Writer) {
	fmt.Fprintf(w, "mpx %s (%s/%s, %s)\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
}
