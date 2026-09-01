package netscope_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/paulstuart/netscope"
	"github.com/paulstuart/netscope/source/kernel"
)

func TestLiveKernelDiscovery(t *testing.T) {
	report, err := netscope.Run(context.Background(), netscope.Local, kernel.New())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	t.Logf("Discovered %d networks, %d devices, NetNS=%d:", len(report.Networks), len(report.Devices), report.NetNS)
	for _, f := range report.Networks {
		t.Logf("  %-20s via=%-10s hops=%d scannable=%-5v table=%d devices=%d",
			f.Network, f.Via, f.Hops, f.Scannable, f.Table, len(f.Devices))
	}
}

func ExampleRun() {
	report, err := netscope.Run(context.Background(), netscope.Local, kernel.New())
	if err != nil {
		fmt.Printf("Run error: %v\n", err)
		return
	}
	for _, f := range report.Networks {
		fmt.Printf("%-20s via=%-10s hops=%d scannable=%v table=%d\n",
			f.Network, f.Via, f.Hops, f.Scannable, f.Table)
	}
}
