package agent

// Detector is implemented by adapters that can tell whether their program is
// installed. Adapters that do not implement it are always available.
type Detector interface {
	// Detect reports whether the agent's program was found, and a short
	// detail: where it was found, or how to install it.
	Detect() (found bool, detail string)
}

// Detect asks the adapter whether its program is installed. An adapter
// without a Detector is reported as found.
func Detect(a Adapter) (found bool, detail string) {
	if d, ok := a.(Detector); ok {
		return d.Detect()
	}
	return true, ""
}
