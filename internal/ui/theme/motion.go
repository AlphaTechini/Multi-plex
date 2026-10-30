package theme

import "time"

// Motion holds animation timings (docs/16, "Motion"). All motion is short,
// purposeful and switchable off with Reduce.
type Motion struct {
	// Reduce turns all animation off: no fades, no highlight decay, a static
	// glyph instead of the spinner, and the file banner shown without a fade.
	Reduce bool
	// FadeDuration is the content fade-in after a diff file switch.
	FadeDuration time.Duration
	// FadeSteps is how many frames the fade uses (one per FadeDuration/FadeSteps).
	FadeSteps int
	// BannerHold is how long the file banner keeps its accent tint before it
	// settles to its resting colours.
	BannerHold time.Duration
	// HunkDecay is how long a freshly arrived hunk stays highlighted.
	HunkDecay time.Duration
	// HunkDecaySteps is how many colour steps the hunk highlight decay uses.
	HunkDecaySteps int
	// SpinnerInterval is the time between spinner frames.
	SpinnerInterval time.Duration
	// ToastDuration is how long an info toast stays before it is dismissed.
	ToastDuration time.Duration
}

// DefaultMotion returns the standard timings: a 150 ms fade in 9 frames
// (60 fps), a 1.2 s hunk highlight decay in 8 steps and an 80 ms spinner.
func DefaultMotion() Motion {
	return Motion{
		FadeDuration:    150 * time.Millisecond,
		FadeSteps:       9,
		BannerHold:      600 * time.Millisecond,
		HunkDecay:       1200 * time.Millisecond,
		HunkDecaySteps:  8,
		SpinnerInterval: 80 * time.Millisecond,
		ToastDuration:   4 * time.Second,
	}
}
