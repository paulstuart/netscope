package netscope

// Availability reports whether a Source can run in the current
// environment. Reason is populated when Available is false, so a caller
// can distinguish "no capability" from "no network".
type Availability struct {
	Available bool
	Reason    string
}
