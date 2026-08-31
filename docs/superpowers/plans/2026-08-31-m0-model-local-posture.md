# M0 — Model and Local Posture — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `v0.1.0` — the core model types, the `Source` aggregator, and a
`source/kernel` implementation that answers "which networks can this host
reach" from the kernel's own routing table, addresses, and neighbour cache.

**Architecture:** A small root package (`netscope`) defines the vocabulary
(`Level`, `Confidence`, `Nexthop`, `Availability`, `SourceResult`, `Device`,
`Finding`, `Report`, `Source`) and a single `Run` aggregator that fans out to
`Source` implementations, filters by level, and reconciles duplicate
prefixes. `source/kernel` is the first (and for M0, only) `Source`: it wraps
`vishvananda/netlink` behind a narrow internal interface so its logic is
testable with a fake, never a live kernel.

**Tech Stack:** Go, `net/netip`, `github.com/vishvananda/netlink`. No cgo, no
external binaries. Everything Linux-only (`//go:build linux` on every file).

**Spec:** [PLAN.md](../../../PLAN.md) — the project's overall design doc and
milestone roadmap. This plan implements only the M0 milestone section.

## Global Constraints

- Module path: `github.com/paulstuart/netscope`. Go version: `go 1.23`.
- **Every `.go` file (including `_test.go` files) carries `//go:build linux`
  as its first line**, per PLAN.md's "Files carry `//go:build linux`"
  constraint. There is no non-Linux build of this module — that is
  intentional, not an oversight.
- **`net/netip` only** for addresses and prefixes. Never `net.IP` or
  `net.IPNet` in any exported type. `net.HardwareAddr` is fine for MAC
  addresses (it isn't the thing the constraint is about).
- **No cgo, no external binaries.** `vishvananda/netlink` is pure Go.
- **Read-only.** No source may write to any interface, route table, or
  neighbour cache.
- **Test verification runs inside a Linux container**, since this plan may
  be executed from a non-Linux host. Every task's verification step uses:

  ```bash
  docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "<command>"
  ```

  Do not attempt to run `go test` directly on a non-Linux host — the build
  constraints will exclude every file and produce a misleading "no Go files"
  result rather than a real pass/fail.

### Ruling: `Finding` gains `Hops` and `Table` fields

PLAN.md's Goals section requires: *"The returned networks should include the
number of hops from the existing network. Thus the subnet it exists on would
be returned with, 0, \<subnet\>."* The `Finding` struct sketch in PLAN.md's
"Core types" section does not include a field for this — an omission in the
sketch, not a deliberate exclusion. This plan adds:

```go
Hops  int // 0 = the host has an address on this network; 1 = reachable through a gateway
Table int // Linux routing table ID the route came from (e.g. 254 = main); 0 if not table-derived
```

`Table` additionally resolves PLAN.md's Open Decision #3 ("policy routing and
multiple tables") using the leaning it already states: *"all-with-attribution,
because VRF-style setups are exactly where this library earns its keep."*

### Ruling: M0 duplicate-prefix reconciliation

PLAN.md's Open Decision #5 (confidence arbitration across independent
sources) is deferred to M4. M0 only ships one source, but the aggregator
still needs a well-defined rule for two `Finding`s that name the same
prefix (e.g. two interfaces route the same subnet). This plan adopts the
minimum viable rule: **numerically lower `Confidence` wins** (`High` = 0 is
the strongest), ties keep the first-seen finding, and `Devices` slices are
unioned regardless of which finding wins. This is a placeholder-free, fully
specified rule for M0 — it does not attempt to answer M4's broader question
of promoting agreement across independent sources.

### Ruling: IPv6 parity from M0

PLAN.md's Open Decision #2 leans toward "full parity from the start" since
`netip` makes it nearly free. `source/kernel` queries both address families
in every netlink call (`netlink.FAMILY_ALL` or `AF_INET`+`AF_INET6`).

---

## File Structure

```
go.mod
doc.go                     // package doc comment
level.go                   // Level bitmask + String()
confidence.go              // Confidence enum + String()
nexthop.go                 // Nexthop enum + String()
sourcestatus.go            // SourceStatus enum + String()
availability.go            // Availability struct
device.go                  // Device struct
sourceresult.go            // SourceResult struct
finding.go                 // Finding struct
report.go                  // Report struct
source.go                  // Source interface
scannable.go               // Scannable(netip.Prefix) bool
netns.go                   // currentNetNS() (uint64, error)
aggregator.go              // Run(ctx, level, sources...) (*Report, error)
source/kernel/types.go     // rawLink, rawAddr, rawRoute, rawNeigh, netlinkClient interface
source/kernel/netlink.go   // real netlinkAdapter wrapping vishvananda/netlink
source/kernel/kernel.go    // Source struct: Name/Level/Available/Discover
README.md
```

---

### Task 1: Module scaffolding and README

**Files:**
- Create: `go.mod`
- Create: `doc.go`
- Modify: `README.md` (currently does not exist as tracked content beyond nothing — create it)

**Interfaces:**
- Produces: the module path `github.com/paulstuart/netscope` that every
  later task's imports depend on.

- [ ] **Step 1: Initialize the module**

```bash
go mod init github.com/paulstuart/netscope
```

Then edit `go.mod` so the `go` directive reads `go 1.23`.

- [ ] **Step 2: Write the package doc comment**

Create `doc.go`:

```go
//go:build linux

// Package netscope answers one question about the host it runs on: which
// networks can this machine actually reach, and how do we know?
//
// It is read-only — no route installation, no interface changes, no state
// changes of any kind — and Linux-only, using netlink instead of shelling
// out to external tools. Callers select which cost/risk tier of source to
// run (see Level) and get back a Report that distinguishes "found nothing"
// from "could not look" for every source it tried.
package netscope
```

- [ ] **Step 3: Write the README**

Create `README.md`:

```markdown
# netscope

A read-only, Linux-only Go library that answers one question about the host
it runs on: **which networks can this machine actually reach, and how do we
know?**

netscope never installs routes, never joins groups, never changes interface
state, and never shells out to external binaries (`ip`, `nmap`, `tcpdump`,
...). It reads the kernel's own routing table and neighbour cache, and can
optionally listen for or lightly probe adjacent network gear, always behind
an explicit cost tier the caller opts into.

## Status

`v0.1.0` — local posture only (`source/kernel`). See
[PLAN.md](PLAN.md) for the full roadmap.

## Install

```bash
go get github.com/paulstuart/netscope
```

## Usage

```go
report, err := netscope.Run(ctx, netscope.Local, kernel.New())
if err != nil {
    log.Fatal(err)
}
for _, f := range report.Networks {
    fmt.Printf("%-18s via=%-9s hops=%d scannable=%v\n", f.Network, f.Via, f.Hops, f.Scannable)
}
```

## Constraints

- **Linux only.** No cgo, no external binaries — every dependency is pure Go.
- **Read-only.** Nothing this library does changes network or kernel state.
```

- [ ] **Step 4: Verify it builds**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "go build ./... && go vet ./..."
```

Expected: no output, exit code 0 (an empty package with only a doc comment
builds cleanly).

- [ ] **Step 5: Commit**

```bash
git add go.mod doc.go README.md
git commit -m "chore: initialize module and package doc"
```

---

### Task 2: Core enums — Level, Confidence, Nexthop, SourceStatus

**Files:**
- Create: `level.go`
- Create: `confidence.go`
- Create: `nexthop.go`
- Create: `sourcestatus.go`
- Test: `level_test.go`
- Test: `confidence_test.go`
- Test: `nexthop_test.go`
- Test: `sourcestatus_test.go`

**Interfaces:**
- Produces: `Level` (bitmask type, values `Local`, `Listen`, `Ask`, method
  `Has(Level) bool`), `Confidence` (`High`, `Medium`, `Inferred`), `Nexthop`
  (`Connected`, `Gateway`, `Tunnel`, `Advertised`), `SourceStatus` (`Ran`,
  `Unavailable`, `Empty`). All four have a `String() string` method. Later
  tasks (3, 4, 7, 9) use all of these by name.

- [ ] **Step 1: Write the failing tests**

Create `level_test.go`:

```go
//go:build linux

package netscope

import "testing"

func TestLevelHas(t *testing.T) {
	cases := []struct {
		name     string
		selected Level
		check    Level
		want     bool
	}{
		{"local in local|listen", Local | Listen, Local, true},
		{"listen in local|listen", Local | Listen, Listen, true},
		{"ask not in local|listen", Local | Listen, Ask, false},
		{"ask in ask", Ask, Ask, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.selected.Has(tc.check); got != tc.want {
				t.Errorf("Has() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLevelString(t *testing.T) {
	cases := []struct {
		level Level
		want  string
	}{
		{Local, "Local"},
		{Listen, "Listen"},
		{Ask, "Ask"},
		{Local | Listen, "Local|Listen"},
		{Local | Listen | Ask, "Local|Listen|Ask"},
		{0, "none"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.level.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
```

Create `confidence_test.go`:

```go
//go:build linux

package netscope

import "testing"

func TestConfidenceString(t *testing.T) {
	cases := []struct {
		c    Confidence
		want string
	}{
		{High, "High"},
		{Medium, "Medium"},
		{Inferred, "Inferred"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.c.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
```

Create `nexthop_test.go`:

```go
//go:build linux

package netscope

import "testing"

func TestNexthopString(t *testing.T) {
	cases := []struct {
		n    Nexthop
		want string
	}{
		{Connected, "Connected"},
		{Gateway, "Gateway"},
		{Tunnel, "Tunnel"},
		{Advertised, "Advertised"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.n.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
```

Create `sourcestatus_test.go`:

```go
//go:build linux

package netscope

import "testing"

func TestSourceStatusString(t *testing.T) {
	cases := []struct {
		s    SourceStatus
		want string
	}{
		{Ran, "Ran"},
		{Unavailable, "Unavailable"},
		{Empty, "Empty"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.s.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "go test ./... -run 'TestLevel|TestConfidence|TestNexthop|TestSourceStatus' -v"
```

Expected: FAIL — `Level`, `Confidence`, `Nexthop`, `SourceStatus` undefined.

- [ ] **Step 3: Implement**

Create `level.go`:

```go
//go:build linux

package netscope

import "strings"

// Level is the cost/risk tier a Source belongs to, and also the bitmask a
// caller passes to Run to select which tiers to execute.
type Level uint8

const (
	// Local sources cost zero packets: kernel routes, addresses, IMDS.
	Local Level = 1 << iota
	// Listen sources only receive: LLDP, CDP, router advertisements.
	Listen
	// Ask sources send a few packets: DHCPINFORM, ARP sweep, ICMP.
	Ask
)

// Has reports whether other is included in the level bitmask l.
func (l Level) Has(other Level) bool {
	return l&other != 0
}

func (l Level) String() string {
	if l == 0 {
		return "none"
	}
	var parts []string
	if l.Has(Local) {
		parts = append(parts, "Local")
	}
	if l.Has(Listen) {
		parts = append(parts, "Listen")
	}
	if l.Has(Ask) {
		parts = append(parts, "Ask")
	}
	return strings.Join(parts, "|")
}
```

Create `confidence.go`:

```go
//go:build linux

package netscope

// Confidence ranks how sure a Finding is. Lower values are stronger; High
// is the zero value so an unset Confidence reads as the strongest claim
// only when a Source deliberately sets it — Sources must always set this
// field explicitly.
type Confidence int

const (
	High Confidence = iota
	Medium
	Inferred
)

func (c Confidence) String() string {
	switch c {
	case High:
		return "High"
	case Medium:
		return "Medium"
	case Inferred:
		return "Inferred"
	default:
		return "Unknown"
	}
}
```

Create `nexthop.go`:

```go
//go:build linux

package netscope

// Nexthop describes how a Finding's network is reached.
type Nexthop int

const (
	Connected Nexthop = iota
	Gateway
	Tunnel
	Advertised
)

func (n Nexthop) String() string {
	switch n {
	case Connected:
		return "Connected"
	case Gateway:
		return "Gateway"
	case Tunnel:
		return "Tunnel"
	case Advertised:
		return "Advertised"
	default:
		return "Unknown"
	}
}
```

Create `sourcestatus.go`:

```go
//go:build linux

package netscope

// SourceStatus records what happened when a Source ran, distinguishing an
// empty result from a source that could not be attempted at all.
type SourceStatus int

const (
	Ran SourceStatus = iota
	Unavailable
	Empty
)

func (s SourceStatus) String() string {
	switch s {
	case Ran:
		return "Ran"
	case Unavailable:
		return "Unavailable"
	case Empty:
		return "Empty"
	default:
		return "Unknown"
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "go test ./... -run 'TestLevel|TestConfidence|TestNexthop|TestSourceStatus' -v"
```

Expected: PASS, all subtests.

- [ ] **Step 5: Commit**

```bash
git add level.go confidence.go nexthop.go sourcestatus.go \
        level_test.go confidence_test.go nexthop_test.go sourcestatus_test.go
git commit -m "feat: add Level, Confidence, Nexthop, SourceStatus enums"
```

---

### Task 3: Availability, Device, SourceResult

**Files:**
- Create: `availability.go`
- Create: `device.go`
- Create: `sourceresult.go`
- Test: `sourceresult_test.go`

**Interfaces:**
- Consumes: `SourceStatus` (Task 2).
- Produces: `Availability{Available bool; Reason string}`,
  `Device{Address netip.Addr; MAC net.HardwareAddr; Name, Platform string;
  Capabilities []string}`, `SourceResult{Source string; Status SourceStatus;
  Reason string}`. Tasks 4, 7, 9 construct these directly.

- [ ] **Step 1: Write the failing test**

Create `sourceresult_test.go`:

```go
//go:build linux

package netscope

import "testing"

func TestSourceResultZeroValue(t *testing.T) {
	var r SourceResult
	if r.Status != Ran {
		t.Errorf("zero-value Status = %v, want Ran", r.Status)
	}
	if r.Source != "" || r.Reason != "" {
		t.Errorf("zero-value SourceResult has non-empty strings: %+v", r)
	}
}

func TestSourceResultUnavailableCarriesReason(t *testing.T) {
	r := SourceResult{Source: "kernel", Status: Unavailable, Reason: "no CAP_NET_RAW"}
	if r.Status != Unavailable {
		t.Fatalf("Status = %v, want Unavailable", r.Status)
	}
	if r.Reason == "" {
		t.Fatal("Reason must be populated when Status is Unavailable")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "go test ./... -run TestSourceResult -v"
```

Expected: FAIL — `SourceResult` undefined.

- [ ] **Step 3: Implement**

Create `availability.go`:

```go
//go:build linux

package netscope

// Availability reports whether a Source can run in the current
// environment. Reason is populated when Available is false, so a caller
// can distinguish "no capability" from "no network".
type Availability struct {
	Available bool
	Reason    string
}
```

Create `device.go`:

```go
//go:build linux

package netscope

import (
	"net"
	"net/netip"
)

// Device is an adjacent network device seen by some Source — a router,
// switch, or a host in the neighbour cache. Name, Platform, and
// Capabilities are only populated by sources that can see them (e.g.
// LLDP); the kernel neighbour cache alone only ever supplies Address and
// MAC.
type Device struct {
	Address      netip.Addr
	MAC          net.HardwareAddr
	Name         string
	Platform     string
	Capabilities []string
}
```

Create `sourceresult.go`:

```go
//go:build linux

package netscope

// SourceResult records what happened when the aggregator ran one Source,
// so a caller can tell an isolated host (Status == Empty) from a source it
// never managed to attempt (Status == Unavailable).
type SourceResult struct {
	Source string
	Status SourceStatus
	Reason string
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "go test ./... -run TestSourceResult -v"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add availability.go device.go sourceresult.go sourceresult_test.go
git commit -m "feat: add Availability, Device, SourceResult types"
```

---

### Task 4: Finding, Report, Source interface

**Files:**
- Create: `finding.go`
- Create: `report.go`
- Create: `source.go`
- Test: `source_test.go`

**Interfaces:**
- Consumes: `Nexthop`, `Confidence` (Task 2), `Device` (Task 3).
- Produces: `Finding` (with `Hops`, `Table` per the ruling above), `Report`,
  and the `Source` interface (`Name() string`, `Level() Level`,
  `Available(context.Context) Availability`,
  `Discover(context.Context) ([]Finding, error)`). Tasks 7 (aggregator) and
  9 (kernel source) implement/consume `Source` directly.

- [ ] **Step 1: Write the failing test**

Create `source_test.go`:

```go
//go:build linux

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
```

Add a small test helper — create `helpers_test.go`:

```go
//go:build linux

package netscope

import (
	"net/netip"
	"testing"
)

func mustPrefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	if err != nil {
		t.Fatalf("ParsePrefix(%q): %v", s, err)
	}
	return p
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "go test ./... -run 'TestFinding|TestReport' -v"
```

Expected: FAIL — `Finding`, `Report`, `Source` undefined.

- [ ] **Step 3: Implement**

Create `finding.go`:

```go
//go:build linux

package netscope

import (
	"net/netip"
	"time"
)

// Finding is one network a Source believes this host can reach.
type Finding struct {
	Network    netip.Prefix
	Via        Nexthop
	Source     string
	Confidence Confidence

	// Hops counts distance from the host's own directly-connected network:
	// 0 for a prefix the host has an address on, 1 for a prefix reachable
	// through a single gateway. Sources beyond the kernel tier may report
	// higher values once they can see further than one hop (e.g. an LLDP
	// neighbour's own advertised routes).
	Hops int

	// Table is the Linux routing table ID the route came from (e.g. 254
	// for main, 255 for local). Zero for findings with no associated
	// table, such as a directly connected prefix derived from an
	// interface address rather than a route.
	Table int

	// Scannable is false for default routes and other unbounded or
	// non-unicast prefixes — see Scannable().
	Scannable bool
	Observed  time.Time
	Devices   []Device
}
```

Create `report.go`:

```go
//go:build linux

package netscope

import "net/netip"

// Report is the result of one Run call.
type Report struct {
	Networks []Finding
	Devices  []Device
	Sources  []SourceResult

	// NetNS is the inode of /proc/self/ns/net at the time of the run.
	// Netlink answers for whichever namespace the process occupies, so
	// this lets a caller weight or reject a Report made inside an
	// unexpected namespace (e.g. a NAT'd container).
	NetNS uint64

	// SuggestedScope is populated starting at M4; nil in v0.1.0.
	SuggestedScope []netip.Prefix
}
```

Create `source.go`:

```go
//go:build linux

package netscope

import "context"

// Source is one mechanism for discovering reachable networks.
type Source interface {
	Name() string
	Level() Level
	Available(context.Context) Availability
	Discover(context.Context) ([]Finding, error)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "go test ./... -run 'TestFinding|TestReport' -v"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add finding.go report.go source.go source_test.go helpers_test.go
git commit -m "feat: add Finding, Report, Source interface"
```

---

### Task 5: Scannable classification

**Files:**
- Create: `scannable.go`
- Test: `scannable_test.go`

**Interfaces:**
- Produces: `Scannable(netip.Prefix) bool`. Task 9 (kernel Discover) calls
  this for every Finding it builds.

- [ ] **Step 1: Write the failing test**

Create `scannable_test.go`:

```go
//go:build linux

package netscope

import "testing"

func TestScannable(t *testing.T) {
	cases := []struct {
		prefix string
		want   bool
	}{
		{"10.0.0.0/24", true},
		{"192.168.1.0/24", true},
		{"2001:db8::/32", true},
		{"0.0.0.0/0", false},
		{"::/0", false},
		{"127.0.0.0/8", false},
		{"::1/128", false},
		{"169.254.0.0/16", false},
		{"fe80::/10", false},
		{"224.0.0.0/4", false},
		{"ff00::/8", false},
	}
	for _, tc := range cases {
		t.Run(tc.prefix, func(t *testing.T) {
			p := mustPrefix(t, tc.prefix)
			if got := Scannable(p); got != tc.want {
				t.Errorf("Scannable(%s) = %v, want %v", tc.prefix, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "go test ./... -run TestScannable -v"
```

Expected: FAIL — `Scannable` undefined.

- [ ] **Step 3: Implement**

Create `scannable.go`:

```go
//go:build linux

package netscope

import "net/netip"

// Scannable reports whether a prefix is a bounded, unicast network worth
// handing to a scanner — false for default routes, loopback, link-local,
// and multicast ranges.
func Scannable(p netip.Prefix) bool {
	if !p.IsValid() {
		return false
	}
	if p.Bits() == 0 {
		return false
	}
	addr := p.Addr()
	switch {
	case addr.IsLoopback(),
		addr.IsLinkLocalUnicast(),
		addr.IsLinkLocalMulticast(),
		addr.IsMulticast(),
		addr.IsUnspecified():
		return false
	}
	return true
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "go test ./... -run TestScannable -v"
```

Expected: PASS, all subtests.

- [ ] **Step 5: Commit**

```bash
git add scannable.go scannable_test.go
git commit -m "feat: add Scannable prefix classification"
```

---

### Task 6: Network namespace identity

**Files:**
- Create: `netns.go`
- Test: `netns_test.go`

**Interfaces:**
- Produces: `currentNetNS() (uint64, error)` (unexported). Task 7
  (aggregator) calls this to populate `Report.NetNS`.

- [ ] **Step 1: Write the failing test**

Create `netns_test.go`:

```go
//go:build linux

package netscope

import "testing"

func TestCurrentNetNS(t *testing.T) {
	inode, err := currentNetNS()
	if err != nil {
		t.Fatalf("currentNetNS() error: %v", err)
	}
	if inode == 0 {
		t.Error("currentNetNS() = 0, want a non-zero inode")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "go test ./... -run TestCurrentNetNS -v"
```

Expected: FAIL — `currentNetNS` undefined.

- [ ] **Step 3: Implement**

Create `netns.go`:

```go
//go:build linux

package netscope

import (
	"fmt"
	"os"
)

// currentNetNS returns the inode of /proc/self/ns/net, which identifies
// which network namespace this process occupies. Netlink answers for
// whichever namespace the process is in, so this lets a caller tell the
// host's real posture apart from a container's NAT'd bridge.
func currentNetNS() (uint64, error) {
	link, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		return 0, fmt.Errorf("read /proc/self/ns/net: %w", err)
	}
	var inode uint64
	if _, err := fmt.Sscanf(link, "net:[%d]", &inode); err != nil {
		return 0, fmt.Errorf("unexpected netns link format %q: %w", link, err)
	}
	return inode, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "go test ./... -run TestCurrentNetNS -v"
```

Expected: PASS. (This reads real process state rather than a fake — that is
correct here: it is deterministic OS introspection, not live network
traffic, so it does not violate "no live network in tests.")

- [ ] **Step 5: Commit**

```bash
git add netns.go netns_test.go
git commit -m "feat: add network namespace identity via /proc/self/ns/net"
```

---

### Task 7: Aggregator

**Files:**
- Create: `aggregator.go`
- Test: `aggregator_test.go`

**Interfaces:**
- Consumes: `Source`, `Finding`, `Report`, `Level`, `SourceResult`,
  `SourceStatus`, `Confidence`, `currentNetNS()` (Tasks 2–6).
- Produces: `Run(ctx context.Context, level Level, sources ...Source)
  (*Report, error)`. This is the package's public entry point; Task 9's
  kernel `Source` is passed to it by callers (and by this task's own
  tests, via fakes).

- [ ] **Step 1: Write the failing tests**

Create `aggregator_test.go`:

```go
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
```

Add `mustAddr` to `helpers_test.go` (append to the file created in Task 4):

```go
func mustAddr(t *testing.T, s string) netip.Addr {
	t.Helper()
	a, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatalf("ParseAddr(%q): %v", s, err)
	}
	return a
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "go test ./... -run TestRun -v"
```

Expected: FAIL — `Run` undefined.

- [ ] **Step 3: Implement**

Create `aggregator.go`:

```go
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
	netns, err := currentNetNS()
	if err != nil {
		return nil, err
	}
	report := &Report{NetNS: netns}

	byPrefix := make(map[netip.Prefix]int)
	deviceSeen := make(map[string]bool)

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
			key := d.Address.String()
			if deviceSeen[key] {
				continue
			}
			deviceSeen[key] = true
			report.Devices = append(report.Devices, d)
		}
	}

	return report, nil
}

// mergeFindings reconciles two Findings that name the same prefix. The
// numerically lower Confidence wins (High == 0 is strongest); Devices are
// unioned regardless of which finding wins. This is a minimum-viable rule
// for M0's single-source reality — full multi-source arbitration is an M4
// open decision (see PLAN.md).
func mergeFindings(existing, incoming Finding) Finding {
	merged := existing
	if incoming.Confidence < existing.Confidence {
		merged.Confidence = incoming.Confidence
		merged.Via = incoming.Via
		merged.Source = incoming.Source
		merged.Hops = incoming.Hops
		merged.Table = incoming.Table
	}
	merged.Devices = append(append([]Device{}, existing.Devices...), incoming.Devices...)
	return merged
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "go test ./... -v"
```

Expected: PASS, every test in the package so far.

- [ ] **Step 5: Commit**

```bash
git add aggregator.go aggregator_test.go helpers_test.go
git commit -m "feat: add Run aggregator with level filtering and prefix reconciliation"
```

---

### Task 8: source/kernel — netlink adapter

**Files:**
- Create: `source/kernel/types.go`
- Create: `source/kernel/netlink.go`

**Interfaces:**
- Produces: `rawLink{Index int; Name string}`,
  `rawAddr{LinkIndex int; Prefix netip.Prefix}`,
  `rawRoute{LinkIndex int; Dst netip.Prefix; Gateway netip.Addr; Table int}`
  (zero-value `Dst` means default route; invalid `Gateway` means on-link),
  `rawNeigh{LinkIndex int; Addr netip.Addr; HWAddr net.HardwareAddr}`, and
  the `netlinkClient` interface (`Links`, `Addrs`, `Routes`, `Neighbours`,
  each returning `([]rawX, error)`). Task 9 consumes this interface with a
  hand-written fake and never touches `netlinkAdapter` directly.

This task has no unit tests of its own: `netlinkAdapter` is a thin,
mechanical translation from the real `vishvananda/netlink` library, which
cannot be exercised without a live kernel. Task 9's tests cover all of the
package's actual logic against a fake `netlinkClient`. Verification here is
a build check only.

- [ ] **Step 1: Add the dependency**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "go get github.com/vishvananda/netlink@latest && go mod tidy"
```

- [ ] **Step 2: Define the narrow internal types**

Create `source/kernel/types.go`:

```go
//go:build linux

package kernel

import (
	"net"
	"net/netip"
)

// rawLink, rawAddr, rawRoute, and rawNeigh are minimal representations
// decoded from netlink, kept narrow so tests can fake them without
// constructing real vishvananda/netlink structs.

type rawLink struct {
	Index int
	Name  string
}

type rawAddr struct {
	LinkIndex int
	Prefix    netip.Prefix
}

// rawRoute represents one kernel route. A zero-value (invalid) Dst means
// the default route. An invalid Gateway means the route is on-link (no
// next hop) and is therefore represented by a connected prefix instead.
type rawRoute struct {
	LinkIndex int
	Dst       netip.Prefix
	Gateway   netip.Addr
	Table     int
}

type rawNeigh struct {
	LinkIndex int
	Addr      netip.Addr
	HWAddr    net.HardwareAddr
}

// netlinkClient is the seam between kernel.Source and the real netlink
// library, so Discover's logic can be tested against a fake instead of a
// live kernel.
type netlinkClient interface {
	Links() ([]rawLink, error)
	Addrs() ([]rawAddr, error)
	Routes() ([]rawRoute, error)
	Neighbours() ([]rawNeigh, error)
}
```

- [ ] **Step 3: Implement the real adapter**

Create `source/kernel/netlink.go`:

```go
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
```

If `go build` fails because your resolved `vishvananda/netlink` version
uses different field names or does not accept `nil` for "all links" in
`AddrList`/`RouteList`/`NeighList`, run
`docker run --rm -v "$PWD":/src -w /src golang:1.23 go doc github.com/vishvananda/netlink Addr`
(and `Route`, `Neigh`) to check the installed version's exact shape, and
adjust the field access accordingly — the translation logic above (build a
`rawX` from each library struct) stays the same either way.

- [ ] **Step 4: Verify it builds**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "go build ./... && go vet ./..."
```

Expected: no output, exit code 0.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum source/kernel/types.go source/kernel/netlink.go
git commit -m "feat: add source/kernel netlink adapter"
```

---

### Task 9: source/kernel — Discover logic

**Files:**
- Create: `source/kernel/kernel.go`
- Test: `source/kernel/kernel_test.go`

**Interfaces:**
- Consumes: `netscope.Source`, `netscope.Finding`, `netscope.Device`,
  `netscope.Local`, `netscope.Connected`, `netscope.Gateway`,
  `netscope.High`, `netscope.Availability`, `netscope.Scannable`
  (root package, Tasks 2–5); `netlinkClient`, `rawLink`, `rawAddr`,
  `rawRoute`, `rawNeigh` (Task 8, same package).
- Produces: `kernel.New() *Source` and `(*Source).Discover`, satisfying
  `netscope.Source`. This is the concrete source a caller passes to
  `netscope.Run`.

- [ ] **Step 1: Write the failing tests**

Create `source/kernel/kernel_test.go`:

```go
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

func (f fakeNetlink) Links() ([]rawLink, error)         { return f.links, nil }
func (f fakeNetlink) Addrs() ([]rawAddr, error)         { return f.addrs, nil }
func (f fakeNetlink) Routes() ([]rawRoute, error)       { return f.routes, nil }
func (f fakeNetlink) Neighbours() ([]rawNeigh, error)   { return f.neighs, nil }

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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "cd source/kernel && go test ./... -v"
```

Expected: FAIL — `Source` undefined.

- [ ] **Step 3: Implement**

Create `source/kernel/kernel.go`:

```go
//go:build linux

package kernel

import (
	"context"
	"net/netip"
	"time"

	"github.com/paulstuart/netscope"
)

// Source is the Local-tier Source that reads the kernel's routing table,
// interface addresses, and neighbour cache via netlink.
type Source struct {
	client netlinkClient
}

// New returns a kernel Source backed by the real netlink library.
func New() *Source {
	return &Source{client: newNetlinkAdapter()}
}

func (s *Source) Name() string { return "kernel" }

func (s *Source) Level() netscope.Level { return netscope.Local }

func (s *Source) Available(context.Context) netscope.Availability {
	return netscope.Availability{Available: true}
}

// Discover returns one Finding per connected interface prefix and one per
// gateway-reachable prefix (including the default route, which is always
// marked unscannable). Neighbour cache entries with a resolved hardware
// address are attached as Devices to the connected Finding that contains
// them.
func (s *Source) Discover(ctx context.Context) ([]netscope.Finding, error) {
	addrs, err := s.client.Addrs()
	if err != nil {
		return nil, err
	}
	routes, err := s.client.Routes()
	if err != nil {
		return nil, err
	}
	neighs, err := s.client.Neighbours()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	seen := make(map[netip.Prefix]bool)
	var findings []netscope.Finding

	for _, a := range addrs {
		masked := a.Prefix.Masked()
		if seen[masked] {
			continue
		}
		seen[masked] = true
		findings = append(findings, netscope.Finding{
			Network:    masked,
			Via:        netscope.Connected,
			Source:     "kernel",
			Confidence: netscope.High,
			Hops:       0,
			Table:      0,
			Scannable:  netscope.Scannable(masked),
			Observed:   now,
		})
	}

	for _, r := range routes {
		if !r.Gateway.IsValid() {
			continue // on-link route; already represented by a connected prefix
		}
		dst := r.Dst
		if !dst.IsValid() {
			if r.Gateway.Is4() {
				dst = netip.PrefixFrom(netip.IPv4Unspecified(), 0)
			} else {
				dst = netip.PrefixFrom(netip.IPv6Unspecified(), 0)
			}
		}
		masked := dst.Masked()
		if seen[masked] {
			continue // already connected; a gateway route doesn't downgrade it
		}
		seen[masked] = true
		findings = append(findings, netscope.Finding{
			Network:    masked,
			Via:        netscope.Gateway,
			Source:     "kernel",
			Confidence: netscope.High,
			Hops:       1,
			Table:      r.Table,
			Scannable:  netscope.Scannable(masked),
			Observed:   now,
		})
	}

	for _, n := range neighs {
		if len(n.HWAddr) == 0 {
			continue
		}
		for i := range findings {
			if findings[i].Via != netscope.Connected {
				continue
			}
			if findings[i].Network.Contains(n.Addr) {
				findings[i].Devices = append(findings[i].Devices, netscope.Device{
					Address: n.Addr,
					MAC:     n.HWAddr,
				})
				break
			}
		}
	}

	return findings, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c "go build ./... && go vet ./... && go test ./... -race -v"
```

Expected: PASS, every test in the module (root package and `source/kernel`).

- [ ] **Step 5: Commit**

```bash
git add source/kernel/kernel.go source/kernel/kernel_test.go
git commit -m "feat: implement source/kernel Discover"
```

---

### Task 10: M0 close-out verification

**Files:**
- Modify: `README.md` (confirm the usage sample matches the real API)

**Interfaces:**
- Consumes: everything from Tasks 1–9. No new production code.

- [ ] **Step 1: Full verification build**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 sh -c \
  "CGO_ENABLED=0 go build ./... && go vet ./... && go test ./... -race -v"
```

Expected: PASS, zero test failures, clean vet output, and the
`CGO_ENABLED=0` build succeeds (per PLAN.md's constraint that this must be
an explicit gate).

- [ ] **Step 2: Confirm the README example matches the real signatures**

Open `README.md` and verify the usage sample's call —
`netscope.Run(ctx, netscope.Local, kernel.New())` — matches
`Run`'s and `kernel.New`'s actual signatures from Tasks 7 and 9. Fix the
sample if either signature drifted during implementation.

- [ ] **Step 3: Manually confirm the acceptance criterion**

If you have access to a real multi-homed Linux host (not required if you
only have the Docker container — note this in your report either way),
build and run:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/paulstuart/netscope"
	"github.com/paulstuart/netscope/source/kernel"
)

func main() {
	report, err := netscope.Run(context.Background(), netscope.Local, kernel.New())
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range report.Networks {
		fmt.Printf("%-20s via=%-10s hops=%d scannable=%v table=%d\n", f.Network, f.Via, f.Hops, f.Scannable, f.Table)
	}
}
```

Confirm every connected prefix and every gateway-reachable prefix appears,
and the default route (`0.0.0.0/0` or `::/0`) is marked `scannable=false`.
Record the actual output (or its absence, with the reason) in your task
report — this is the plan's acceptance criterion, and whoever reads the
final report needs to know whether it was verified against a real host or
only against the fakes in Task 9.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: confirm M0 usage sample against final API" --allow-empty
```

(Use `--allow-empty` only if Step 2 found nothing to fix; otherwise commit
the real README diff with a normal commit.)
