package diff

// Summary returns the number of files and the total added and deleted lines.
func Summary(files []File) (n, added, deleted int) {
	for i := range files {
		added += files[i].Added
		deleted += files[i].Deleted
	}
	return len(files), added, deleted
}
