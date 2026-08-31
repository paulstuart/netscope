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
    fmt.Printf("%-20s via=%-10s hops=%d scannable=%v table=%d\n", f.Network, f.Via, f.Hops, f.Scannable, f.Table)
}
```

## Constraints

- **Linux only.** No cgo, no external binaries — every dependency is pure Go.
- **Read-only.** Nothing this library does changes network or kernel state.
