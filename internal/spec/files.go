package spec

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Names of the spec folder and its three files.
const (
	specsRel         = ".multiplex/specs"
	requirementsFile = "requirements.md"
	designFile       = "design.md"
	tasksFile        = "tasks.md"
)

// specDir returns the folder for one spec under the workspace root.
func specDir(workspaceRoot, slug string) string {
	return filepath.Join(workspaceRoot, filepath.FromSlash(specsRel), slug)
}

// readOptional returns the file's text, or "" when the file does not exist.
func readOptional(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("spec: read %s: %w", path, err)
	}
	return string(data), nil
}

// headingTitle returns the text of the first "# " heading in md, or "".
func headingTitle(md string) string {
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimRight(line, "\r")
		if rest, ok := strings.CutPrefix(line, "# "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}
