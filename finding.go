//go:build linux

package netscope

import (
	"net/netip"
	"time"
)

// Finding is one network a Source believes this host can reach.
type Finding struct {
	Network    netip.Prefix
	Via        Nexthop
	Source     string
	Confidence Confidence

	// Hops counts distance from the host's own directly-connected network:
	// 0 for a prefix the host has an address on, 1 for a prefix reachable
	// through a single gateway. Sources beyond the kernel tier may report
	// higher values once they can see further than one hop (e.g. an LLDP
	// neighbour's own advertised routes).
	Hops int

	// Table is the Linux routing table ID the route came from (e.g. 254
	// for main, 255 for local). Zero for findings with no associated
	// table, such as a directly connected prefix derived from an
	// interface address rather than a route.
	Table int

	// Scannable is false for default routes and other unbounded or
	// non-unicast prefixes — see Scannable().
	Scannable bool
	Observed  time.Time
	Devices   []Device
}
