//go:build darwin || freebsd || openbsd || netbsd

package kernel

import (
	"net/netip"
	"testing"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

func TestParseBSDPrefix(t *testing.T) {
	tests := []struct {
		name    string
		dst     route.Addr
		mask    route.Addr
		flags   int
		want    netip.Prefix
		wantOk  bool
	}{
		{
			name:   "IPv4 subnet",
			dst:    &route.Inet4Addr{IP: [4]byte{192, 168, 1, 0}},
			mask:   &route.Inet4Addr{IP: [4]byte{255, 255, 255, 0}},
			flags:  unix.RTF_UP,
			want:   mustPrefix(t, "192.168.1.0/24"),
			wantOk: true,
		},
		{
			name:   "IPv4 default route",
			dst:    &route.Inet4Addr{IP: [4]byte{0, 0, 0, 0}},
			mask:   &route.Inet4Addr{IP: [4]byte{0, 0, 0, 0}},
			flags:  unix.RTF_UP | unix.RTF_GATEWAY,
			want:   mustPrefix(t, "0.0.0.0/0"),
			wantOk: true,
		},
		{
			name:   "IPv4 host route",
			dst:    &route.Inet4Addr{IP: [4]byte{192, 168, 1, 50}},
			mask:   nil,
			flags:  unix.RTF_UP | unix.RTF_HOST,
			want:   mustPrefix(t, "192.168.1.50/32"),
			wantOk: true,
		},
		{
			name:   "IPv6 default route",
			dst:    &route.Inet6Addr{IP: [16]byte{}},
			mask:   &route.Inet6Addr{IP: [16]byte{}},
			flags:  unix.RTF_UP | unix.RTF_GATEWAY,
			want:   mustPrefix(t, "::/0"),
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseBSDPrefix(tt.dst, tt.mask, tt.flags)
			if ok != tt.wantOk {
				t.Fatalf("parseBSDPrefix() ok = %v, want %v", ok, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("parseBSDPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseBSDRoutesAndNeighbours(t *testing.T) {
	// Synthetic RouteMessages representing a default route, a connected route, and an ARP neighbor
	msgs := []route.Message{
		&route.RouteMessage{
			Index: 1,
			Flags: unix.RTF_UP | unix.RTF_GATEWAY,
			Addrs: []route.Addr{
				&route.Inet4Addr{IP: [4]byte{0, 0, 0, 0}},
				&route.Inet4Addr{IP: [4]byte{192, 168, 1, 1}},
				&route.Inet4Addr{IP: [4]byte{0, 0, 0, 0}},
			},
		},
		&route.RouteMessage{
			Index: 1,
			Flags: unix.RTF_UP,
			Addrs: []route.Addr{
				&route.Inet4Addr{IP: [4]byte{192, 168, 1, 0}},
				&route.LinkAddr{Index: 1},
				&route.Inet4Addr{IP: [4]byte{255, 255, 255, 0}},
			},
		},
		&route.RouteMessage{
			Index: 1,
			Flags: unix.RTF_UP | unix.RTF_LLINFO | unix.RTF_HOST,
			Addrs: []route.Addr{
				&route.Inet4Addr{IP: [4]byte{192, 168, 1, 1}},
				&route.LinkAddr{Index: 1, Addr: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}},
			},
		},
	}

	routes, err := parseBSDRoutes(msgs)
	if err != nil {
		t.Fatalf("parseBSDRoutes() err = %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("got %d routes, want 2 (default + connected; LLINFO should be filtered)", len(routes))
	}
	if routes[0].Dst != mustPrefix(t, "0.0.0.0/0") || routes[0].Gateway != mustAddr(t, "192.168.1.1") {
		t.Errorf("routes[0] = %+v, want 0.0.0.0/0 via 192.168.1.1", routes[0])
	}

	neighs, err := parseBSDNeighbours(msgs)
	if err != nil {
		t.Fatalf("parseBSDNeighbours() err = %v", err)
	}
	if len(neighs) != 1 {
		t.Fatalf("got %d neighs, want 1", len(neighs))
	}
	if neighs[0].Addr != mustAddr(t, "192.168.1.1") || neighs[0].HWAddr.String() != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("neighs[0] = %+v, want 192.168.1.1 (aa:bb:cc:dd:ee:ff)", neighs[0])
	}
}
