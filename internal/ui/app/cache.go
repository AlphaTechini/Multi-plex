package app

// regionCache remembers the last rendering of one region and the key it was
// made for. A region is drawn again only when its key changes.
type regionCache struct {
	key any
	out string
	ok  bool
}

// get returns the cached string when key matches. Keys are comparable
// structs holding everything the region's rendering depends on.
func (c *regionCache) get(key any, off bool) (string, bool) {
	if off || !c.ok || c.key != key {
		return "", false
	}
	return c.out, true
}

func (c *regionCache) put(key any, out string) string {
	c.key, c.out, c.ok = key, out, true
	return out
}

// joinExact places exactly sized blocks side by side. Unlike kit.JoinColumns
// it does not measure the lines, because every region panel is already
// exactly its width and height.
func joinExact(blocks ...[]string) string {
	if len(blocks) == 0 {
		return ""
	}
	h := len(blocks[0])
	n := 0
	for _, b := range blocks {
		for _, l := range b {
			n += len(l)
		}
	}
	buf := make([]byte, 0, n+h)
	for y := range h {
		if y > 0 {
			buf = append(buf, '\n')
		}
		for _, b := range blocks {
			buf = append(buf, b[y]...)
		}
	}
	return string(buf)
}
