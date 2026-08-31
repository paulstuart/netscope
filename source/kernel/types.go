//go:build linux

package kernel

import (
	"net"
	"net/netip"
)

// rawLink, rawAddr, rawRoute, and rawNeigh are minimal representations
// decoded from netlink, kept narrow so tests can fake them without
// constructing real vishvananda/netlink structs.

type rawLink struct {
	Index int
	Name  string
}

type rawAddr struct {
	LinkIndex int
	Prefix    netip.Prefix
}

// rawRoute represents one kernel route. A zero-value (invalid) Dst means
// the default route. An invalid Gateway means the route is on-link (no
// next hop) and is therefore represented by a connected prefix instead.
//
// For a multipath (ECMP) route the kernel reports no single gateway;
// Gateway then carries the first next hop that has one, so the route's
// destination prefix is still reported rather than being mistaken for an
// on-link route and dropped.
type rawRoute struct {
	LinkIndex int
	Dst       netip.Prefix
	Gateway   netip.Addr
	Table     int
}

type rawNeigh struct {
	LinkIndex int
	Addr      netip.Addr
	HWAddr    net.HardwareAddr
}

// netlinkClient is the seam between kernel.Source and the real netlink
// library, so Discover's logic can be tested against a fake instead of a
// live kernel.
type netlinkClient interface {
	Links() ([]rawLink, error)
	Addrs() ([]rawAddr, error)
	Routes() ([]rawRoute, error)
	Neighbours() ([]rawNeigh, error)
}
