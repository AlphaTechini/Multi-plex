package theme

import "image/color"

// Surfaces are background colours, from darkest to lightest.
type Surfaces struct {
	// Sunken is the status bar and the scrim colour behind modals.
	Sunken color.Color
	// Base is the default background of every region (Dark+ editor background).
	Base color.Color
	// Raised is for cards, inputs and comment boxes sitting on Base.
	Raised color.Color
	// Overlay is for modals, the command palette and toasts.
	Overlay color.Color
	// Control is the background of secondary buttons and key caps.
	Control color.Color
	// Hover is the background of a row under the mouse or keyboard cursor in
	// an unfocused list.
	Hover color.Color
	// Selected is the background of the selected row in a focused list.
	Selected color.Color
	// TextSelection is the background of selected text or diff lines.
	TextSelection color.Color
}

// Borders are line colours.
type Borders struct {
	// Subtle is for dividers inside a region (rules, rail separators).
	Subtle color.Color
	// Default is the frame of an unfocused region.
	Default color.Color
	// Focus is the frame of the focused region. Only one region uses it.
	Focus color.Color
}

// Texts are foreground tiers. Use the lowest tier that is still readable for
// the job; hierarchy comes from these tiers more than from bold.
type Texts struct {
	// Strong is for titles, the active tab and the selected row.
	Strong color.Color
	// Primary is for body text and code.
	Primary color.Color
	// Secondary is for supporting text (summary details, descriptions).
	Secondary color.Color
	// Muted is for metadata, labels and line numbers.
	Muted color.Color
	// Faint is for placeholders, separators and disabled items.
	Faint color.Color
	// OnAccent is text on an Accent.Fill background.
	OnAccent color.Color
	// OnState is dark text on a bright state or severity background (pills).
	OnState color.Color
}

// Accents are the interactive accent colours.
type Accents struct {
	// Base is the focus ring, links, the active tab marker and the cursor.
	Base color.Color
	// Fill is the background of the primary button.
	Fill color.Color
	// Subtle is a tinted background (active tab pill, file banner).
	Subtle color.Color
}

// States are session state colours, used by state dots and badges.
type States struct {
	// Starting is a session creating its worktree or launching the agent.
	Starting color.Color
	// Working is an agent running a turn.
	Working color.Color
	// Blocked is an agent waiting for the user (approval or question).
	Blocked color.Color
	// Done is a finished session not yet viewed.
	Done color.Color
	// Idle is a session waiting for a new task.
	Idle color.Color
	// Error is a crashed agent or failed setup.
	Error color.Color
}

// Severities are insight card severity colours (docs/08).
type Severities struct {
	// Info is for notes that need no answer.
	Info color.Color
	// Warning is for notes about risk.
	Warning color.Color
	// Decision is for notes that need an answer from the user.
	Decision color.Color
}

// DiffColors are the diff viewer colours (docs/07).
type DiffColors struct {
	// AddLineBg is the background of an added line.
	AddLineBg color.Color
	// AddWordBg is the background of the changed words inside an added line.
	AddWordBg color.Color
	// AddGutterBg is the line-number gutter background of an added line.
	AddGutterBg color.Color
	// AddSign is the "+" sign and added counts.
	AddSign color.Color
	// DelLineBg is the background of a removed line.
	DelLineBg color.Color
	// DelWordBg is the background of the changed words inside a removed line.
	DelWordBg color.Color
	// DelGutterBg is the line-number gutter background of a removed line.
	DelGutterBg color.Color
	// DelSign is the "−" sign and removed counts.
	DelSign color.Color
	// HunkBg is the background of a hunk header row.
	HunkBg color.Color
	// HunkFg is the "@@ -a,b +c,d @@" range text of a hunk header.
	HunkFg color.Color
	// LineNumber is the gutter line-number colour.
	LineNumber color.Color
	// FoldFg is the "··· 42 unchanged lines ···" row text.
	FoldFg color.Color
	// NewHunkBg is where a freshly arrived hunk's background starts before it
	// decays to its normal line background (live updates).
	NewHunkBg color.Color
	// Renamed is the file status colour for renames.
	Renamed color.Color
	// Modified is the file status colour for modified files.
	Modified color.Color
}

// SyntaxColors are code token colours (Dark+ token colours).
type SyntaxColors struct {
	// Plain is default code text, operators and punctuation.
	Plain color.Color
	// Keyword is declaration and storage keywords (func, var, type).
	Keyword color.Color
	// Control is control-flow keywords (if, return, for).
	Control color.Color
	// String is string and rune literals.
	String color.Color
	// Number is numeric literals.
	Number color.Color
	// Comment is comments.
	Comment color.Color
	// Function is function and method names.
	Function color.Color
	// Type is type names.
	Type color.Color
	// Variable is variables, parameters and fields.
	Variable color.Color
	// Constant is constants and enum members.
	Constant color.Color
	// Regexp is regular expressions and escape sequences.
	Regexp color.Color
}
