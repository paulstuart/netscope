//go:build linux

package kernel

import (
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
)

type netlinkAdapter struct{}

func newNetlinkAdapter() netlinkClient { return netlinkAdapter{} }

func (netlinkAdapter) Links() ([]rawLink, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}
	out := make([]rawLink, 0, len(links))
	for _, l := range links {
		attrs := l.Attrs()
		out = append(out, rawLink{Index: attrs.Index, Name: attrs.Name})
	}
	return out, nil
}

func (netlinkAdapter) Addrs() ([]rawAddr, error) {
	addrs, err := netlink.AddrList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return nil, err
	}
	out := make([]rawAddr, 0, len(addrs))
	for _, a := range addrs {
		prefix, ok := ipNetToPrefix(a.IPNet)
		if !ok {
			continue
		}
		out = append(out, rawAddr{LinkIndex: a.LinkIndex, Prefix: prefix})
	}
	return out, nil
}

func (netlinkAdapter) Routes() ([]rawRoute, error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return nil, err
	}
	out := make([]rawRoute, 0, len(routes))
	for _, r := range routes {
		rr := rawRoute{LinkIndex: r.LinkIndex, Table: r.Table}
		if r.Dst != nil {
			if p, ok := ipNetToPrefix(r.Dst); ok {
				rr.Dst = p
			}
		}
		if r.Gw != nil {
			if addr, ok := netip.AddrFromSlice(r.Gw); ok {
				rr.Gateway = addr.Unmap()
			}
		}
		out = append(out, rr)
	}
	return out, nil
}

func (netlinkAdapter) Neighbours() ([]rawNeigh, error) {
	neighs, err := netlink.NeighList(0, netlink.FAMILY_ALL)
	if err != nil {
		return nil, err
	}
	out := make([]rawNeigh, 0, len(neighs))
	for _, n := range neighs {
		addr, ok := netip.AddrFromSlice(n.IP)
		if !ok {
			continue
		}
		out = append(out, rawNeigh{
			LinkIndex: n.LinkIndex,
			Addr:      addr.Unmap(),
			HWAddr:    n.HardwareAddr,
		})
	}
	return out, nil
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
	ones, _ := ipNet.Mask.Size()
	return netip.PrefixFrom(addr, ones), true
}
