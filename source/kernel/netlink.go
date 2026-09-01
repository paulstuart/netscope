//go:build linux

package kernel

import (
	"net/netip"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
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
	// netlink.RouteList silently restricts its answer to the main table
	// (254). Filtering on RT_TABLE_UNSPEC instead returns every table, so
	// VRF and policy-routing setups can be attributed to the table their
	// route actually came from. This also surfaces table 255 (local)
	// routes, which carry no gateway and are dropped downstream by
	// Discover's on-link check.
	routes, err := netlink.RouteListFiltered(
		netlink.FAMILY_ALL,
		&netlink.Route{Table: unix.RT_TABLE_UNSPEC},
		netlink.RT_FILTER_TABLE,
	)
	if err != nil {
		return nil, err
	}
	out := make([]rawRoute, 0, len(routes))
	for _, r := range routes {
		rr := rawRoute{LinkIndex: r.LinkIndex, Table: r.Table}
		if r.Dst != nil {
			p, ok := ipNetToPrefix(r.Dst)
			if !ok {
				// A present but unparseable destination must not be
				// confused with an absent one, which is the sentinel for
				// the default route. Drop the route instead.
				continue
			}
			rr.Dst = p
		}
		if gw, ok := routeGateway(r); ok {
			rr.Gateway = gw
		}
		out = append(out, rr)
	}
	return out, nil
}

// routeGateway returns a route's next-hop gateway address. A multipath
// (ECMP) route carries no Route.Gw at all — the kernel reports its next
// hops in Route.MultiPath instead — so without this the route would look
// on-link and be dropped, losing the destination prefix entirely. M0
// records the first usable next hop, which is enough to emit one Finding
// per destination prefix; enumerating every ECMP next hop is deferred.
func routeGateway(r netlink.Route) (netip.Addr, bool) {
	if r.Gw != nil {
		addr, ok := netip.AddrFromSlice(r.Gw)
		if !ok {
			return netip.Addr{}, false
		}
		return addr.Unmap(), true
	}
	for _, nh := range r.MultiPath {
		if nh == nil || nh.Gw == nil {
			continue // on-link next hop; carries no gateway address
		}
		if addr, ok := netip.AddrFromSlice(nh.Gw); ok {
			return addr.Unmap(), true
		}
	}
	return netip.Addr{}, false
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
