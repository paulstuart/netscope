//go:build linux

package netscope

import (
	"context"
	"net/netip"
)

// Run executes every source whose Level is included in level, merges their
// findings, and returns a Report. Sources are run in the order given;
// Run never runs a Source outside the requested level.
func Run(ctx context.Context, level Level, sources ...Source) (*Report, error) {
	report := &Report{}
	// A missing or unreadable /proc/self/ns/net (a scratch container, say)
	// disables namespace attribution, not the whole run: NetNS stays zero
	// and every source still executes.
	if netns, err := currentNetNS(); err == nil {
		report.NetNS = netns
	}

	byPrefix := make(map[netip.Prefix]int)
	deviceSeen := make(map[netip.Addr]bool)

	for _, src := range sources {
		if !level.Has(src.Level()) {
			continue
		}

		avail := src.Available(ctx)
		if !avail.Available {
			report.Sources = append(report.Sources, SourceResult{
				Source: src.Name(), Status: Unavailable, Reason: avail.Reason,
			})
			continue
		}

		findings, err := src.Discover(ctx)
		if err != nil {
			report.Sources = append(report.Sources, SourceResult{
				Source: src.Name(), Status: Unavailable, Reason: err.Error(),
			})
			continue
		}
		if len(findings) == 0 {
			report.Sources = append(report.Sources, SourceResult{
				Source: src.Name(), Status: Empty,
			})
			continue
		}
		report.Sources = append(report.Sources, SourceResult{
			Source: src.Name(), Status: Ran,
		})

		for _, f := range findings {
			// Merge on the canonical (masked) prefix so a source that
			// hands back 192.168.1.10/24 still reconciles against another
			// source's 192.168.1.0/24. Masking is not a contract Sources
			// are held to, so the aggregator enforces it here.
			f.Network = f.Network.Masked()
			if idx, ok := byPrefix[f.Network]; ok {
				report.Networks[idx] = mergeFindings(report.Networks[idx], f)
			} else {
				byPrefix[f.Network] = len(report.Networks)
				report.Networks = append(report.Networks, f)
			}
		}
	}

	for _, f := range report.Networks {
		for _, d := range f.Devices {
			if deviceSeen[d.Address] {
				continue
			}
			deviceSeen[d.Address] = true
			report.Devices = append(report.Devices, d)
		}
	}

	return report, nil
}

// mergeFindings reconciles two Findings that name the same prefix. The
// stronger (numerically higher) Confidence wins; Devices are unioned
// regardless of which finding wins. This is a minimum-viable rule for
// M0's single-source reality — full multi-source arbitration is an M4
// open decision (see PLAN.md).
func mergeFindings(existing, incoming Finding) Finding {
	merged := existing
	if incoming.Confidence > existing.Confidence {
		merged.Confidence = incoming.Confidence
		merged.Via = incoming.Via
		merged.Source = incoming.Source
		merged.Hops = incoming.Hops
		merged.Table = incoming.Table
	}
	merged.Devices = append(append([]Device{}, existing.Devices...), incoming.Devices...)
	return merged
}
