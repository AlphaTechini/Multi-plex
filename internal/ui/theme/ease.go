package theme

// EaseOutCubic maps linear progress p in [0,1] to eased progress: fast start,
// gentle landing. Values outside [0,1] are clamped. Use it to turn a frame
// index into the t passed to Blend.
func EaseOutCubic(p float64) float64 {
	switch {
	case p <= 0:
		return 0
	case p >= 1:
		return 1
	}
	q := 1 - p
	return 1 - q*q*q
}
