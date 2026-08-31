//go:build linux

package netscope

import (
	"net"
	"net/netip"
)

// Device is an adjacent network device seen by some Source — a router,
// switch, or a host in the neighbour cache. Name, Platform, and
// Capabilities are only populated by sources that can see them (e.g.
// LLDP); the kernel neighbour cache alone only ever supplies Address and
// MAC.
type Device struct {
	Address      netip.Addr
	MAC          net.HardwareAddr
	Name         string
	Platform     string
	Capabilities []string
}
