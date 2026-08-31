//go:build linux

package kernel

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/paulstuart/netscope"
)

type fakeNetlink struct {
	links  []rawLink
	addrs  []rawAddr
	routes []rawRoute
	neighs []rawNeigh
}

func (f fakeNetlink) Links() ([]rawLink, error)       { return f.links, nil }
func (f fakeNetlink) Addrs() ([]rawAddr, error)       { return f.addrs, nil }
func (f fakeNetlink) Routes() ([]rawRoute, error)     { return f.routes, nil }
func (f fakeNetlink) Neighbours() ([]rawNeigh, error) { return f.neighs, nil }

func mustPrefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	if err != nil {
		t.Fatalf("ParsePrefix(%q): %v", s, err)
	}
	return p
}

func mustAddr(t *testing.T, s string) netip.Addr {
	t.Helper()
	a, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatalf("ParseAddr(%q): %v", s, err)
	}
	return a
}

func TestSourceIdentity(t *testing.T) {
	s := &Source{client: fakeNetlink{}}
	if s.Name() != "kernel" {
		t.Errorf("Name() = %q, want %q", s.Name(), "kernel")
	}
	if s.Level() != netscope.Local {
		t.Errorf("Level() = %v, want Local", s.Level())
	}
	if avail := s.Available(context.Background()); !avail.Available {
		t.Errorf("Available() = %+v, want Available=true", avail)
	}
}

func TestDiscoverMultiHomedHost(t *testing.T) {
	// Two connected prefixes (eth0, eth1), a default route via eth0, and a
	// gateway route on eth1 reaching a subnet the host has no address on —
	// this is the M0 acceptance scenario from PLAN.md.
	fake := fakeNetlink{
		links: []rawLink{{Index: 1, Name: "eth0"}, {Index: 2, Name: "eth1"}},
		addrs: []rawAddr{
			{LinkIndex: 1, Prefix: mustPrefix(t, "192.168.1.10/24")},
			{LinkIndex: 2, Prefix: mustPrefix(t, "10.0.0.5/24")},
		},
		routes: []rawRoute{
			{LinkIndex: 1, Dst: netip.Prefix{}, Gateway: mustAddr(t, "192.168.1.1"), Table: 254},
			{LinkIndex: 2, Dst: mustPrefix(t, "10.0.1.0/24"), Gateway: mustAddr(t, "10.0.0.1"), Table: 254},
			// on-link route for the eth0 subnet: no gateway, already covered by
			// the connected prefix above, must not produce a duplicate finding.
			{LinkIndex: 1, Dst: mustPrefix(t, "192.168.1.0/24"), Gateway: netip.Addr{}, Table: 254},
		},
	}

	s := &Source{client: fake}
	findings, err := s.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	byNetwork := make(map[netip.Prefix]netscope.Finding)
	for _, f := range findings {
		byNetwork[f.Network] = f
	}

	connected1 := byNetwork[mustPrefix(t, "192.168.1.0/24")]
	if connected1.Via != netscope.Connected || connected1.Hops != 0 || !connected1.Scannable {
		t.Errorf("192.168.1.0/24 = %+v, want Via=Connected Hops=0 Scannable=true", connected1)
	}

	connected2 := byNetwork[mustPrefix(t, "10.0.0.0/24")]
	if connected2.Via != netscope.Connected || connected2.Hops != 0 {
		t.Errorf("10.0.0.0/24 = %+v, want Via=Connected Hops=0", connected2)
	}

	gateway := byNetwork[mustPrefix(t, "10.0.1.0/24")]
	if gateway.Via != netscope.Gateway || gateway.Hops != 1 || !gateway.Scannable || gateway.Table != 254 {
		t.Errorf("10.0.1.0/24 = %+v, want Via=Gateway Hops=1 Scannable=true Table=254", gateway)
	}

	def := byNetwork[mustPrefix(t, "0.0.0.0/0")]
	if def.Via != netscope.Gateway || def.Scannable {
		t.Errorf("0.0.0.0/0 = %+v, want Via=Gateway Scannable=false", def)
	}

	if len(findings) != 4 {
		t.Errorf("got %d findings, want 4 (two connected, one gateway-reachable, one default); findings=%+v", len(findings), findings)
	}
}

// TestSourceThroughRun exercises the seam between this package and the
// aggregator: a kernel.Source backed by a fake client, driven by
// netscope.Run rather than by calling Discover directly.
func TestSourceThroughRun(t *testing.T) {
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	fake := fakeNetlink{
		links: []rawLink{{Index: 1, Name: "eth0"}, {Index: 2, Name: "eth1"}},
		addrs: []rawAddr{
			{LinkIndex: 1, Prefix: mustPrefix(t, "192.168.1.10/24")},
			{LinkIndex: 2, Prefix: mustPrefix(t, "10.0.0.5/24")},
		},
		routes: []rawRoute{
			{LinkIndex: 1, Dst: netip.Prefix{}, Gateway: mustAddr(t, "192.168.1.1"), Table: 254},
			{LinkIndex: 2, Dst: mustPrefix(t, "10.0.1.0/24"), Gateway: mustAddr(t, "10.0.0.1"), Table: 254},
			{LinkIndex: 1, Dst: mustPrefix(t, "192.168.1.0/24"), Gateway: netip.Addr{}, Table: 254},
		},
		neighs: []rawNeigh{{LinkIndex: 1, Addr: mustAddr(t, "192.168.1.1"), HWAddr: mac}},
	}

	report, err := netscope.Run(context.Background(), netscope.Local, &Source{client: fake})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(report.Networks) != 4 {
		t.Errorf("Networks = %d, want 4 (two connected, one gateway-reachable, one default); got %+v",
			len(report.Networks), report.Networks)
	}
	if len(report.Sources) != 1 {
		t.Fatalf("Sources = %+v, want exactly one result", report.Sources)
	}
	if report.Sources[0].Source != "kernel" || report.Sources[0].Status != netscope.Ran {
		t.Errorf("Sources[0] = %+v, want Source=kernel Status=Ran", report.Sources[0])
	}
	if report.NetNS == 0 {
		t.Error("Report.NetNS = 0, want a non-zero inode (tests run with a real /proc)")
	}
	if len(report.Devices) != 1 || report.Devices[0].Address != mustAddr(t, "192.168.1.1") {
		t.Errorf("Devices = %+v, want the one neighbour hoisted to the report", report.Devices)
	}
	for _, f := range report.Networks {
		if f.Network != f.Network.Masked() {
			t.Errorf("Network %v is not in canonical masked form", f.Network)
		}
		if f.Confidence != netscope.High {
			t.Errorf("Network %v Confidence = %v, want High", f.Network, f.Confidence)
		}
	}
}

func TestDiscoverAttachesNeighboursToConnectedPrefix(t *testing.T) {
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	fake := fakeNetlink{
		links: []rawLink{{Index: 1, Name: "eth0"}},
		addrs: []rawAddr{{LinkIndex: 1, Prefix: mustPrefix(t, "192.168.1.10/24")}},
		neighs: []rawNeigh{
			{LinkIndex: 1, Addr: mustAddr(t, "192.168.1.1"), HWAddr: mac},
			{LinkIndex: 1, Addr: mustAddr(t, "192.168.1.2"), HWAddr: nil}, // unresolved, must be skipped
		},
	}

	s := &Source{client: fake}
	findings, err := s.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	devices := findings[0].Devices
	if len(devices) != 1 {
		t.Fatalf("got %d devices, want 1 (the unresolved neighbour must be skipped)", len(devices))
	}
	if devices[0].Address != mustAddr(t, "192.168.1.1") || devices[0].MAC.String() != mac.String() {
		t.Errorf("device = %+v, want Address=192.168.1.1 MAC=%s", devices[0], mac)
	}
}
