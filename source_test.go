package netscope

import (
	"context"
	"testing"
	"time"
)

// fakeSource is a minimal, compile-time-only check that Source is
// implementable with the fields this task defines.
type fakeSource struct {
	name    string
	level   Level
	avail   Availability
	finding []Finding
}

func (f fakeSource) Name() string { return f.name }
func (f fakeSource) Level() Level { return f.level }
func (f fakeSource) Available(context.Context) Availability { return f.avail }
func (f fakeSource) Discover(context.Context) ([]Finding, error) { return f.finding, nil }

var _ Source = fakeSource{}

func TestFindingFields(t *testing.T) {
	now := time.Now()
	f := Finding{
		Network:    mustPrefix(t, "10.0.0.0/24"),
		Via:        Connected,
		Source:     "kernel",
		Confidence: High,
		Hops:       0,
		Table:      0,
		Scannable:  true,
		Observed:   now,
	}
	if f.Hops != 0 || f.Via != Connected {
		t.Errorf("connected finding should have Hops=0, Via=Connected, got %+v", f)
	}
}

func TestReportZeroValue(t *testing.T) {
	var r Report
	if r.Networks != nil || r.Devices != nil || r.Sources != nil {
		t.Errorf("zero-value Report should have nil slices, got %+v", r)
	}
	if r.NetNS != 0 {
		t.Errorf("zero-value Report.NetNS = %d, want 0", r.NetNS)
	}
}
