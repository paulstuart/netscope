//go:build linux

package netscope

import "net/netip"

// Report is the result of one Run call.
type Report struct {
	Networks []Finding
	Devices  []Device
	Sources  []SourceResult

	// NetNS is the inode of /proc/self/ns/net at the time of the run.
	// Netlink answers for whichever namespace the process occupies, so
	// this lets a caller weight or reject a Report made inside an
	// unexpected namespace (e.g. a NAT'd container).
	NetNS uint64

	// SuggestedScope is populated starting at M4; nil in v0.1.0.
	SuggestedScope []netip.Prefix
}
