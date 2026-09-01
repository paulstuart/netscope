//go:build darwin || freebsd || openbsd || netbsd

package kernel

import (
	"net"
	"net/netip"
	"syscall"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

type bsdAdapter struct{}

func newNetlinkAdapter() netlinkClient { return bsdAdapter{} }

func (bsdAdapter) Links() ([]rawLink, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	out := make([]rawLink, 0, len(ifaces))
	for _, iface := range ifaces {
		out = append(out, rawLink{Index: iface.Index, Name: iface.Name})
	}
	return out, nil
}

func (bsdAdapter) Addrs() ([]rawAddr, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var out []rawAddr
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipNet, ok := a.(*net.IPNet); ok {
				if prefix, ok := ipNetToPrefix(ipNet); ok {
					out = append(out, rawAddr{LinkIndex: iface.Index, Prefix: prefix})
				}
			}
		}
	}
	return out, nil
}

func (bsdAdapter) Routes() ([]rawRoute, error) {
	rib, err := route.FetchRIB(syscall.AF_UNSPEC, route.RIBTypeRoute, 0)
	if err != nil {
		return nil, err
	}
	msgs, err := route.ParseRIB(route.RIBTypeRoute, rib)
	if err != nil {
		return nil, err
	}
	return parseBSDRoutes(msgs)
}

func (bsdAdapter) Neighbours() ([]rawNeigh, error) {
	rib, err := route.FetchRIB(syscall.AF_UNSPEC, route.RIBTypeRoute, 0)
	if err != nil {
		return nil, err
	}
	msgs, err := route.ParseRIB(route.RIBTypeRoute, rib)
	if err != nil {
		return nil, err
	}
	return parseBSDNeighbours(msgs)
}

func parseBSDRoutes(msgs []route.Message) ([]rawRoute, error) {
	var routes []rawRoute
	for _, m := range msgs {
		rm, ok := m.(*route.RouteMessage)
		if !ok || rm.Flags&unix.RTF_UP == 0 || rm.Flags&unix.RTF_LLINFO != 0 || rm.Flags&unix.RTF_WASCLONED != 0 {
			continue
		}
		var dstAddr, gwAddr, maskAddr route.Addr
		if len(rm.Addrs) > 0 {
			dstAddr = rm.Addrs[0]
		}
		if len(rm.Addrs) > 1 {
			gwAddr = rm.Addrs[1]
		}
		if len(rm.Addrs) > 2 {
			maskAddr = rm.Addrs[2]
		}
		prefix, ok := parseBSDPrefix(dstAddr, maskAddr, rm.Flags)
		if !ok {
			continue
		}
		var gw netip.Addr
		if rm.Flags&unix.RTF_GATEWAY != 0 {
			gw, _ = parseBSDAddr(gwAddr)
		}
		routes = append(routes, rawRoute{
			LinkIndex: rm.Index,
			Dst:       prefix,
			Gateway:   gw,
			Table:     0,
		})
	}
	return routes, nil
}

func parseBSDNeighbours(msgs []route.Message) ([]rawNeigh, error) {
	var neighs []rawNeigh
	for _, m := range msgs {
		rm, ok := m.(*route.RouteMessage)
		if !ok || rm.Flags&unix.RTF_LLINFO == 0 {
			continue
		}
		var ip netip.Addr
		var hwAddr net.HardwareAddr
		if len(rm.Addrs) > 0 {
			ip, _ = parseBSDAddr(rm.Addrs[0])
		}
		if len(rm.Addrs) > 1 {
			if la, ok := rm.Addrs[1].(*route.LinkAddr); ok && len(la.Addr) > 0 {
				hwAddr = net.HardwareAddr(la.Addr)
			}
		}
		if ip.IsValid() && len(hwAddr) > 0 {
			neighs = append(neighs, rawNeigh{
				LinkIndex: rm.Index,
				Addr:      ip,
				HWAddr:    hwAddr,
			})
		}
	}
	return neighs, nil
}

func parseBSDAddr(a route.Addr) (netip.Addr, bool) {
	if a == nil {
		return netip.Addr{}, false
	}
	switch v := a.(type) {
	case *route.Inet4Addr:
		return netip.AddrFrom4(v.IP).Unmap(), true
	case *route.Inet6Addr:
		ip := v.IP
		// Clear embedded BSD IPv6 link-local interface index in bytes 2-3
		if ip[0] == 0xfe && (ip[1]&0xc0) == 0x80 {
			ip[2] = 0
			ip[3] = 0
		}
		return netip.AddrFrom16(ip).Unmap(), true
	}
	return netip.Addr{}, false
}

func parseBSDPrefix(dstAddr, maskAddr route.Addr, flags int) (netip.Prefix, bool) {
	dst, ok := parseBSDAddr(dstAddr)
	if !ok {
		return netip.Prefix{}, false
	}
	if flags&unix.RTF_HOST != 0 {
		return netip.PrefixFrom(dst, dst.BitLen()), true
	}
	mask, ok := parseBSDAddr(maskAddr)
	if !ok {
		if dst.IsUnspecified() {
			return netip.PrefixFrom(dst, 0), true
		}
		return netip.PrefixFrom(dst, dst.BitLen()), true
	}
	ones, bits := net.IPMask(mask.AsSlice()).Size()
	if bits == 0 {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(dst, ones), true
}
