package diffview

import "time"

// Layout thresholds, in cells of the view's own (inner) width. docs/16 states
// them for the main view including its frame (112 for the file rail, 200 for
// side-by-side), which is two cells more than the view receives.
const (
	// railWidth is the width of the file rail.
	railWidth = 32
	// railMinWidth is the view width from which the file rail is shown.
	railMinWidth = 110
	// sbsMinWidth is the view width from which side-by-side is allowed.
	sbsMinWidth = 198
	// overlayWidth is the width of the file list overlay (rail collapsed).
	overlayWidth = 44
)

// Behaviour tunables.
const (
	// refreshDebounce is how long diff.changed events are coalesced.
	refreshDebounce = 300 * time.Millisecond
	// scrollStep is the lines moved by one mouse wheel notch.
	scrollStep = 3
	// scrolloff is how many lines the cursor keeps from the viewport edge.
	scrolloff = 3
	// hlMargin is how many units around the viewport are highlighted ahead.
	hlMargin = 70
	// hugeDiffLines is the changed-line count above which a file is not
	// opened until the user asks.
	hugeDiffLines = 40000
	// tabWidth is the cell width of a tab stop in code.
	tabWidth = 4
	// hscrollStep is the cells moved by one sideways scroll.
	hscrollStep = 8
	// maxCachedContents is how many file contents are kept.
	maxCachedContents = 8
	// bodyTop is the number of rows above the diff body (banner + hint row).
	bodyTop = 2
)
