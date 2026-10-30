// Package diffview is the Diff main-view mode (docs/07-diff-viewer.md): one
// file's changes at a time, a collapsible file rail, a banner and fade on
// every file switch, word-level highlights, syntax colours, inline review
// comments, hunk and file reviewed state, and an optional side-by-side mode.
//
// The view talks to the daemon only through backend.Backend, and only inside
// tea.Cmd functions. Parsing, word diffing and syntax highlighting also run
// in commands, so Update and Render never block. Render draws only the rows
// that are visible and caches their rendered strings.
//
// # Keys
//
//	j k ↑ ↓          move the line cursor          ctrl+d ctrl+u   half page
//	g G              top / bottom                   n N             next / previous change
//	] [              next / previous file           f               files (rail, or an overlay when narrow)
//	enter            expand a fold, else comment    c               comment the selection
//	V J K            extend the selection           y               copy path:Lx-Ly and the lines
//	v                file reviewed                  x               hunk reviewed
//	s                side-by-side (200+ columns)    S ctrl+s        send queued comments
//	e d              edit / delete the comment on the cursor line
//	h l ← →          scroll sideways                esc             clear the selection
//
// u (while the "Comment queued" toast shows) takes back the comment queued last.
//
// In the comment box: enter queues, ctrl+s queues and sends now, ctrl+d keeps
// a draft, esc cancels, shift+enter or alt+enter inserts a newline.
//
// The mouse clicks lines (shift extends, drag selects), folds and files, and
// scrolls with the wheel.
//
// # Sizes
//
// Render receives the inner size of the main view's frame. The file rail
// needs 110 columns of it (112 with the frame) and side-by-side 198 (200).
package diffview
