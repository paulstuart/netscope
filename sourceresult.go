package netscope

// SourceResult records what happened when the aggregator ran one Source,
// so a caller can tell an isolated host (Status == Empty) from a source it
// never managed to attempt (Status == Unavailable).
type SourceResult struct {
	Source string
	Status SourceStatus
	Reason string
}
