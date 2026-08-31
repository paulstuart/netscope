//go:build linux

package netscope

// Confidence ranks how sure a Finding is. Higher values are stronger, and
// Inferred is the zero value: a Source that forgets to set Confidence
// makes the weakest possible claim rather than silently outranking a
// Source that set it correctly.
type Confidence int

const (
	Inferred Confidence = iota
	Medium
	High
)

func (c Confidence) String() string {
	switch c {
	case High:
		return "High"
	case Medium:
		return "Medium"
	case Inferred:
		return "Inferred"
	default:
		return "Unknown"
	}
}
