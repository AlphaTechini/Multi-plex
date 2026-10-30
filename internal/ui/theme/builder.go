package theme

import "github.com/charmbracelet/colorprofile"

// Builder constructs a theme resolved for a colour profile. Theme files
// register one with Register.
type Builder func(p colorprofile.Profile) *Theme
