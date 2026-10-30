package windowshello

import (
	"os"
	"path/filepath"
	"runtime"
)

// helperName is the helper's file name on this OS.
func helperName() string {
	if runtime.GOOS == "windows" {
		return "mpx-hello.exe"
	}
	return "mpx-hello"
}

// Find returns the path to the helper: MPX_HELLO_PATH if it names an existing
// file, else mpx-hello.exe next to the running executable. It returns "" when
// the helper cannot be found.
func Find() string {
	if p := os.Getenv("MPX_HELLO_PATH"); p != "" {
		if isFile(p) {
			return p
		}
		return ""
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	p := filepath.Join(filepath.Dir(exe), helperName())
	if isFile(p) {
		return p
	}
	return ""
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
