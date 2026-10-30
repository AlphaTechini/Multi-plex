package spec

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Info summarises one spec for the sidebar.
type Info struct {
	// Slug is the folder name, used to refer to the spec.
	Slug string
	// Title is the first "# " heading of requirements.md, or the slug if there is none.
	Title string
	// TasksDone counts tasks marked done in tasks.md, subtasks included.
	TasksDone int
	// TasksTotal counts all tasks in tasks.md, subtasks included.
	TasksTotal int
	// CriteriaTotal counts the acceptance criteria in requirements.md.
	CriteriaTotal int
}

// List returns every spec under workspaceRoot/.multiplex/specs, sorted by slug.
// A workspace with no specs folder returns an empty list.
func List(workspaceRoot string) ([]Info, error) {
	root := filepath.Join(workspaceRoot, filepath.FromSlash(specsRel))
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("spec: list %s: %w", root, err)
	}

	var out []Info
	for _, e := range entries {
		if !e.IsDir() || !validSlug(e.Name()) {
			continue
		}
		info, err := summarise(workspaceRoot, e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, info)
	}
	return out, nil
}

// summarise reads one spec's requirements and tasks and counts its progress.
func summarise(workspaceRoot, slug string) (Info, error) {
	dir := specDir(workspaceRoot, slug)
	reqs, err := readOptional(filepath.Join(dir, requirementsFile))
	if err != nil {
		return Info{}, err
	}
	tasksMD, err := readOptional(filepath.Join(dir, tasksFile))
	if err != nil {
		return Info{}, err
	}

	info := Info{Slug: slug, Title: headingTitle(reqs), CriteriaTotal: len(ParseCriteria(reqs))}
	if info.Title == "" {
		info.Title = slug
	}
	for _, t := range ParseTasks(tasksMD) {
		info.TasksTotal++
		if t.Done {
			info.TasksDone++
		}
	}
	return info, nil
}
