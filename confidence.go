//go:build linux

package netscope

// Confidence ranks how sure a Finding is. Lower values are stronger; High
// is the zero value so an unset Confidence reads as the strongest claim
// only when a Source deliberately sets it — Sources must always set this
// field explicitly.
type Confidence int

const (
	High Confidence = iota
	Medium
	Inferred
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
