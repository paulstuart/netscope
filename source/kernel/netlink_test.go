//go:build linux

package kernel

import (
	"net"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"
)

// These tests exercise the translation layer between vishvananda/netlink's
// structs and this package's raw* types. netlink.Route, netlink.Addr and
// netlink.Neigh are plain structs, so they can be built directly with no
// live kernel.

func TestIPNetToPrefix(t *testing.T) {
	cases := []struct {
		name  string
		in    *net.IPNet
		want  string // empty means "expect ok == false"
		wantK bool
	}{
		{
			name:  "well-formed IPv4",
			in:    &net.IPNet{IP: net.IPv4(192, 168, 1, 10), Mask: net.CIDRMask(24, 32)},
			want:  "192.168.1.10/24",
			wantK: true,
		},
		{
			name:  "well-formed IPv6",
			in:    &net.IPNet{IP: net.ParseIP("2001:db8::1"), Mask: net.CIDRMask(64, 128)},
			want:  "2001:db8::1/64",
			wantK: true,
		},
		{
			name:  "legitimate IPv4 default route mask",
			in:    &net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)},
			want:  "0.0.0.0/0",
			wantK: true,
		},
		{
			name:  "legitimate IPv6 default route mask",
			in:    &net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, 128)},
			want:  "::/0",
			wantK: true,
		},
		{
			name: "non-contiguous mask is rejected, not read as /0",
			in:   &net.IPNet{IP: net.IPv4(10, 0, 0, 1), Mask: net.IPMask{0xff, 0x00, 0xff, 0x00}},
		},
		{
			name: "mask wider than address family is rejected",
			in:   &net.IPNet{IP: net.IPv4(10, 0, 0, 1), Mask: net.CIDRMask(104, 128)},
		},
		{
			name: "nil IPNet",
			in:   nil,
		},
		{
			name: "unparseable IP length",
			in:   &net.IPNet{IP: net.IP{1, 2, 3}, Mask: net.CIDRMask(24, 32)},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ipNetToPrefix(tc.in)
			if ok != tc.wantK {
				t.Fatalf("ipNetToPrefix() ok = %v, want %v (got prefix %v)", ok, tc.wantK, got)
			}
			if !tc.wantK {
				if got != (netip.Prefix{}) {
					t.Errorf("ipNetToPrefix() = %v on failure, want zero Prefix", got)
				}
				return
			}
			if got.String() != tc.want {
				t.Errorf("ipNetToPrefix() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRouteGateway(t *testing.T) {
	cases := []struct {
		name  string
		route netlink.Route
		want  string // empty means "expect ok == false"
		wantK bool
	}{
		{
			name:  "single IPv4 gateway",
			route: netlink.Route{Gw: net.ParseIP("192.168.1.1")},
			want:  "192.168.1.1",
			wantK: true,
		},
		{
			name:  "single IPv6 gateway",
			route: netlink.Route{Gw: net.ParseIP("fe80::1")},
			want:  "fe80::1",
			wantK: true,
		},
		{
			name: "multipath route uses its first next hop",
			route: netlink.Route{
				Gw: nil,
				MultiPath: []*netlink.NexthopInfo{
					{LinkIndex: 1, Gw: net.ParseIP("10.0.0.1")},
					{LinkIndex: 2, Gw: net.ParseIP("10.0.0.2")},
				},
			},
			want:  "10.0.0.1",
			wantK: true,
		},
		{
			name: "multipath route skips on-link next hops without a gateway",
			route: netlink.Route{
				Gw: nil,
				MultiPath: []*netlink.NexthopInfo{
					{LinkIndex: 1, Gw: nil},
					{LinkIndex: 2, Gw: net.ParseIP("10.0.0.2")},
				},
			},
			want:  "10.0.0.2",
			wantK: true,
		},
		{
			name: "multipath route with no gateways at all is on-link",
			route: netlink.Route{
				Gw:        nil,
				MultiPath: []*netlink.NexthopInfo{{LinkIndex: 1}, {LinkIndex: 2}},
			},
		},
		{
			name:  "on-link route has no gateway",
			route: netlink.Route{Gw: nil},
		},
		{
			name:  "malformed gateway is rejected",
			route: netlink.Route{Gw: net.IP{1, 2, 3}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := routeGateway(tc.route)
			if ok != tc.wantK {
				t.Fatalf("routeGateway() ok = %v, want %v (got addr %v)", ok, tc.wantK, got)
			}
			if !tc.wantK {
				if got.IsValid() {
					t.Errorf("routeGateway() = %v on failure, want invalid Addr", got)
				}
				return
			}
			if got.String() != tc.want {
				t.Errorf("routeGateway() = %v, want %v", got, tc.want)
			}
			if got.Is4In6() {
				t.Errorf("routeGateway() = %v, want an unmapped address", got)
			}
		})
	}
}

// TestMultipathRouteYieldsAFinding is the end-to-end statement of the ECMP
// fix: a default route whose next hops live in MultiPath must still reach
// Discover as a gateway route rather than being mistaken for on-link and
// dropped.
func TestMultipathRouteYieldsAFinding(t *testing.T) {
	ecmp := netlink.Route{
		Table: 254,
		Gw:    nil,
		MultiPath: []*netlink.NexthopInfo{
			{LinkIndex: 1, Gw: net.ParseIP("192.168.1.1")},
			{LinkIndex: 2, Gw: net.ParseIP("192.168.2.1")},
		},
	}
	gw, ok := routeGateway(ecmp)
	if !ok {
		t.Fatal("routeGateway() on an ECMP route returned no gateway; the route would be dropped as on-link")
	}
	rr := rawRoute{LinkIndex: ecmp.MultiPath[0].LinkIndex, Gateway: gw, Table: ecmp.Table}
	if !rr.Gateway.IsValid() {
		t.Fatalf("rawRoute.Gateway = %v, want a valid address", rr.Gateway)
	}
}
