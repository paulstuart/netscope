package kernel

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/paulstuart/netscope"
)

// Source is the Local-tier Source that reads the kernel's routing table,
// interface addresses, and neighbour cache (via netlink on Linux, or
// routing sockets on BSD/macOS).
type Source struct {
	client netlinkClient
}

// New returns a kernel Source backed by the host's routing facilities.
func New() *Source {
	return &Source{client: newNetlinkAdapter()}
}

func (s *Source) Name() string { return "kernel" }

func (s *Source) Level() netscope.Level { return netscope.Local }

func (s *Source) Available(context.Context) netscope.Availability {
	return netscope.Availability{Available: true}
}

// Discover returns one Finding per connected interface prefix and one per
// gateway-reachable prefix (including the default route, which is always
// marked unscannable). Neighbour cache entries with a resolved hardware
// address are attached as Devices to the connected Finding that contains
// them.
func (s *Source) Discover(ctx context.Context) ([]netscope.Finding, error) {
	addrs, err := s.client.Addrs()
	if err != nil {
		return nil, err
	}
	routes, err := s.client.Routes()
	if err != nil {
		return nil, err
	}
	neighs, err := s.client.Neighbours()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	seen := make(map[netip.Prefix]bool)
	var findings []netscope.Finding

	for _, a := range addrs {
		masked := a.Prefix.Masked()
		if seen[masked] {
			continue
		}
		seen[masked] = true
		findings = append(findings, netscope.Finding{
			Network:    masked,
			Via:        netscope.Connected,
			Source:     "kernel",
			Confidence: netscope.High,
			Hops:       0,
			Table:      0,
			Scannable:  netscope.Scannable(masked),
			Observed:   now,
		})
	}

	for _, r := range routes {
		if !r.Gateway.IsValid() {
			continue // on-link route; already represented by a connected prefix
		}
		dst := r.Dst
		if !dst.IsValid() {
			if r.Gateway.Is4() {
				dst = netip.PrefixFrom(netip.IPv4Unspecified(), 0)
			} else {
				dst = netip.PrefixFrom(netip.IPv6Unspecified(), 0)
			}
		}
		masked := dst.Masked()
		if seen[masked] {
			continue // already connected; a gateway route doesn't downgrade it
		}
		seen[masked] = true
		findings = append(findings, netscope.Finding{
			Network:    masked,
			Via:        netscope.Gateway,
			Source:     "kernel",
			Confidence: netscope.High,
			Hops:       1,
			Table:      r.Table,
			Scannable:  netscope.Scannable(masked),
			Observed:   now,
		})
	}

	for _, n := range neighs {
		if len(n.HWAddr) == 0 {
			continue
		}
		for i := range findings {
			if findings[i].Via != netscope.Connected {
				continue
			}
			if findings[i].Network.Contains(n.Addr) {
				findings[i].Devices = append(findings[i].Devices, netscope.Device{
					Address: n.Addr,
					MAC:     n.HWAddr,
				})
				break
			}
		}
	}

	return findings, nil
}

func ipNetToPrefix(ipNet *net.IPNet) (netip.Prefix, bool) {
	if ipNet == nil {
		return netip.Prefix{}, false
	}
	addr, ok := netip.AddrFromSlice(ipNet.IP)
	if !ok {
		return netip.Prefix{}, false
	}
	addr = addr.Unmap()
	ones, bits := ipNet.Mask.Size()
	if bits == 0 {
		// net.IPMask.Size reports (0, 0) for a non-contiguous mask. A
		// legitimate all-zero mask still reports its width (32 or 128),
		// so bits == 0 means the mask was malformed — reporting it as a
		// /0 would fabricate a default route.
		return netip.Prefix{}, false
	}
	prefix := netip.PrefixFrom(addr, ones)
	if !prefix.IsValid() {
		// Mask width and address family disagree (e.g. a /104 on an
		// unmapped IPv4 address).
		return netip.Prefix{}, false
	}
	return prefix, true
}
