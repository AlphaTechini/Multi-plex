package codeview

import "strconv"

func itoa(n int) string { return strconv.Itoa(n) }

// lines returns the number of lines in the open file.
func (m *model) lines() int {
	if m.src == nil {
		return 0
	}
	return m.src.NumLines()
}

// clampScroll keeps the cursor inside the file and the viewport inside both
// the file and around the cursor.
func (m *model) clampScroll() {
	n := m.lines()
	m.cursor = min(max(m.cursor, 0), max(n-1, 0))
	rows := max(m.rows, 1)
	m.top = min(max(m.top, 0), max(n-rows, 0))
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+rows {
		m.top = m.cursor - rows + 1
	}
}

// moveCursor moves the cursor by d lines and scrolls to keep it visible.
func (m *model) moveCursor(d int) {
	m.cursor += d
	m.clampScroll()
}

// scrollBy scrolls the viewport by d lines and drags the cursor along when it
// would leave the screen.
func (m *model) scrollBy(d int) {
	n := m.lines()
	rows := max(m.rows, 1)
	m.top = min(max(m.top+d, 0), max(n-rows, 0))
	m.cursor = min(max(m.cursor, m.top), m.top+rows-1)
	m.cursor = min(m.cursor, max(n-1, 0))
}

// showRange scrolls so the lines first..last (0-based) are visible, starting
// a little above the first when they fit, and moves nothing else.
func (m *model) showRange(first, last int) {
	n := m.lines()
	rows := max(m.rows, 1)
	if first >= m.top && last < m.top+rows && last-first < rows {
		return
	}
	span := last - first + 1
	top := first - 3
	if span > rows-3 {
		top = first
	}
	m.top = min(max(top, 0), max(n-rows, 0))
	m.clampScroll()
}

// pageBy moves the cursor and the viewport together by d lines (ctrl+d,
// ctrl+u, page keys), like a pager.
func (m *model) pageBy(d int) {
	m.top += d
	m.cursor += d
	m.clampScroll()
}
