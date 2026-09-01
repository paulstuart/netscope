package netscope

// SourceStatus records what happened when a Source ran, distinguishing an
// empty result from a source that could not be attempted at all.
type SourceStatus int

const (
	Ran SourceStatus = iota
	Unavailable
	Empty
)

func (s SourceStatus) String() string {
	switch s {
	case Ran:
		return "Ran"
	case Unavailable:
		return "Unavailable"
	case Empty:
		return "Empty"
	default:
		return "Unknown"
	}
}
