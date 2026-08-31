//go:build linux

package netscope

import (
	"context"
	"errors"
	"testing"
)

type stubSource struct {
	name     string
	level    Level
	avail    Availability
	findings []Finding
	err      error
}

func (s stubSource) Name() string { return s.name }
func (s stubSource) Level() Level { return s.level }
func (s stubSource) Available(context.Context) Availability { return s.avail }
func (s stubSource) Discover(context.Context) ([]Finding, error) { return s.findings, s.err }

func TestRunFiltersByLevel(t *testing.T) {
	local := stubSource{name: "local", level: Local, avail: Availability{Available: true},
		findings: []Finding{{Network: mustPrefix(t, "10.0.0.0/24"), Confidence: High}}}
	ask := stubSource{name: "ask", level: Ask, avail: Availability{Available: true},
		findings: []Finding{{Network: mustPrefix(t, "10.0.1.0/24"), Confidence: High}}}

	report, err := Run(context.Background(), Local, local, ask)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if len(report.Networks) != 1 {
		t.Fatalf("Networks = %v, want exactly the local source's one finding", report.Networks)
	}
	if len(report.Sources) != 1 || report.Sources[0].Source != "local" {
		t.Errorf("Sources = %v, want only the selected 'local' source recorded", report.Sources)
	}
}

func TestRunRecordsUnavailable(t *testing.T) {
	src := stubSource{name: "kernel", level: Local, avail: Availability{Available: false, Reason: "no permission"}}
	report, err := Run(context.Background(), Local, src)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if len(report.Sources) != 1 || report.Sources[0].Status != Unavailable {
		t.Fatalf("Sources = %+v, want one Unavailable result", report.Sources)
	}
	if report.Sources[0].Reason != "no permission" {
		t.Errorf("Reason = %q, want %q", report.Sources[0].Reason, "no permission")
	}
}

func TestRunRecordsErrorAsUnavailable(t *testing.T) {
	src := stubSource{name: "kernel", level: Local, avail: Availability{Available: true}, err: errors.New("netlink: boom")}
	report, err := Run(context.Background(), Local, src)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if len(report.Sources) != 1 || report.Sources[0].Status != Unavailable {
		t.Fatalf("Sources = %+v, want one Unavailable result", report.Sources)
	}
}

func TestRunRecordsEmpty(t *testing.T) {
	src := stubSource{name: "kernel", level: Local, avail: Availability{Available: true}, findings: nil}
	report, err := Run(context.Background(), Local, src)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if len(report.Sources) != 1 || report.Sources[0].Status != Empty {
		t.Fatalf("Sources = %+v, want one Empty result", report.Sources)
	}
}

func TestRunMergesDuplicatePrefixesPreferringHigherConfidence(t *testing.T) {
	prefix := mustPrefix(t, "10.0.0.0/24")
	weak := stubSource{name: "weak", level: Local, avail: Availability{Available: true},
		findings: []Finding{{Network: prefix, Confidence: Inferred, Devices: []Device{{Address: mustAddr(t, "10.0.0.5")}}}}}
	strong := stubSource{name: "strong", level: Local, avail: Availability{Available: true},
		findings: []Finding{{Network: prefix, Confidence: High, Devices: []Device{{Address: mustAddr(t, "10.0.0.6")}}}}}

	report, err := Run(context.Background(), Local, weak, strong)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if len(report.Networks) != 1 {
		t.Fatalf("Networks = %v, want exactly one merged finding", report.Networks)
	}
	got := report.Networks[0]
	if got.Confidence != High {
		t.Errorf("Confidence = %v, want High (the stronger of the two sources)", got.Confidence)
	}
	if len(got.Devices) != 2 {
		t.Errorf("Devices = %v, want both sources' devices unioned", got.Devices)
	}
}

func TestRunPopulatesNetNS(t *testing.T) {
	report, err := Run(context.Background(), Local)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if report.NetNS == 0 {
		t.Error("Report.NetNS = 0, want a non-zero inode")
	}
}
