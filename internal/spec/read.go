package spec

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Doc holds the three files of one spec. A missing file is an empty string.
type Doc struct {
	// Slug is the spec's folder name.
	Slug string
	// Requirements is the text of requirements.md.
	Requirements string
	// Design is the text of design.md.
	Design string
	// Tasks is the text of tasks.md.
	Tasks string
}

// Read loads one spec. It returns ErrInvalidSlug for a slug with a path
// separator, a colon or "..", and an error wrapping fs.ErrNotExist when the
// spec folder does not exist.
func Read(workspaceRoot, slug string) (Doc, error) {
	if !validSlug(slug) {
		return Doc{}, fmt.Errorf("%w: %q", ErrInvalidSlug, slug)
	}
	dir := specDir(workspaceRoot, slug)
	fi, err := os.Stat(dir)
	if err != nil {
		return Doc{}, fmt.Errorf("spec: read %s: %w", slug, err)
	}
	if !fi.IsDir() {
		return Doc{}, fmt.Errorf("spec: read %s: %w", slug, fs.ErrNotExist)
	}

	doc := Doc{Slug: slug}
	files := []struct {
		name string
		dst  *string
	}{
		{requirementsFile, &doc.Requirements},
		{designFile, &doc.Design},
		{tasksFile, &doc.Tasks},
	}
	for _, f := range files {
		text, err := readOptional(filepath.Join(dir, f.name))
		if err != nil {
			return Doc{}, err
		}
		*f.dst = text
	}
	return doc, nil
}
