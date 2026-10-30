package app

import (
	"github.com/alphatechini/multi-plex/internal/ui/view"
	"github.com/alphatechini/multi-plex/internal/ui/views/codeview"
	"github.com/alphatechini/multi-plex/internal/ui/views/diffview"
	"github.com/alphatechini/multi-plex/internal/ui/views/overview"
	"github.com/alphatechini/multi-plex/internal/ui/views/specview"
	"github.com/alphatechini/multi-plex/internal/ui/views/term"
)

// modeConstructors lists the constructor of each main-view mode: one line per
// view, each with the signature func(view.Env) view.View. A mode whose view is
// not built yet uses placeholder(mode) here.
var modeConstructors = map[view.Mode]func(view.Env) view.View{
	view.ModeOverview: overview.New,
	view.ModeDiff:     diffview.New,
	view.ModeCode:     codeview.New,
	view.ModeSpecs:    specview.New,
	view.ModeTerm:     term.New,
}

// modeOrder is the tab order.
var modeOrder = []view.Mode{view.ModeOverview, view.ModeDiff, view.ModeCode, view.ModeSpecs, view.ModeTerm}

// modeName is the tab label before a view has been built, and the name used
// in the palette and help.
func modeName(m view.Mode) string {
	switch m {
	case view.ModeOverview:
		return "Overview"
	case view.ModeDiff:
		return "Diff"
	case view.ModeCode:
		return "Code"
	case view.ModeSpecs:
		return "Specs"
	}
	return "Term"
}

// modeKeys are the prefix-key letters of each mode (besides 1-5).
var modeKeys = map[view.Mode]string{
	view.ModeOverview: "o", view.ModeDiff: "d", view.ModeCode: "c", view.ModeSpecs: "s", view.ModeTerm: "t",
}
