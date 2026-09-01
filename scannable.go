package netscope

import "net/netip"

// Scannable reports whether a prefix is a bounded, unicast network worth
// handing to a scanner — false for default routes, loopback, link-local,
// and multicast ranges.
func Scannable(p netip.Prefix) bool {
	if !p.IsValid() {
		return false
	}
	if p.Bits() == 0 {
		return false
	}
	addr := p.Addr()
	switch {
	case addr.IsLoopback(),
		addr.IsLinkLocalUnicast(),
		addr.IsLinkLocalMulticast(),
		addr.IsMulticast(),
		addr.IsUnspecified():
		return false
	}
	return true
}
