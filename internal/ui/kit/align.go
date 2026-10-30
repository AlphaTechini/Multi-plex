package kit

// Align is horizontal alignment for labels in dividers and rows.
type Align int

// Alignments.
const (
	// AlignLeft puts the label at the start.
	AlignLeft Align = iota
	// AlignCenter centres the label.
	AlignCenter
	// AlignRight puts the label at the end.
	AlignRight
)
