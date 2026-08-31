//go:build linux

package netscope

import "strings"

// Level is the cost/risk tier a Source belongs to, and also the bitmask a
// caller passes to Run to select which tiers to execute.
type Level uint8

const (
	// Local sources cost zero packets: kernel routes, addresses, IMDS.
	Local Level = 1 << iota
	// Listen sources only receive: LLDP, CDP, router advertisements.
	Listen
	// Ask sources send a few packets: DHCPINFORM, ARP sweep, ICMP.
	Ask
)

// Has reports whether other is included in the level bitmask l.
func (l Level) Has(other Level) bool {
	return l&other != 0
}

func (l Level) String() string {
	if l == 0 {
		return "none"
	}
	var parts []string
	if l.Has(Local) {
		parts = append(parts, "Local")
	}
	if l.Has(Listen) {
		parts = append(parts, "Listen")
	}
	if l.Has(Ask) {
		parts = append(parts, "Ask")
	}
	return strings.Join(parts, "|")
}
