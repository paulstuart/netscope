# PLAN

## What this is

A Go library that answers one question about the host it runs on:

> **Which networks can this machine actually reach, and how do we know?**

It answers it without SNMP, without device credentials, and without shelling out to
external binaries. Sources range from free (read the kernel's routing table) through
passive (listen for what routers and switches already broadcast) to small, well-formed
probes (ask the DHCP server for its static routes).

The intended consumer is any agent or tool that needs to bound its own scope before doing
work — a discovery process that would otherwise be handed every possible target and asked
to guess which ones it can see.

### Why it needs to exist

If an application needs to be able to understand where it lives and what it can talk to 
without using external tools.

The protocol pieces are all solved and mature. Nothing here requires writing a packet
decoder. What does not exist is the layer above them: a normalized result model, a
cost-and-risk ladder so callers opt in to what they are comfortable running, and honest
reporting of what could not be attempted. That coordination layer is the library.

---

## Goals

- Enumerate reachable networks with provenance and a confidence level per finding.
- The returned networks should include the number of hops from the existing network.
  Thus the subnet it exists on would be returned with, 0, <subnet>
- Distinguish "found nothing" from "could not look". This is the property that makes the
  output trustworthy, and it is easy to get wrong.
- Identify adjacent network devices (routers, switches) and their management addresses,
  so a caller has a seed for whatever deeper inspection it does next.
- Degrade cleanly. A missing capability disables one source, never the run.
- Be safe enough that a security review is short: read-only, no state changes, no writes.

## Non-goals

- Port scanning, service detection, OS fingerprinting. Out of scope, permanently.
- Device inventory or configuration collection. This library finds the door; it does not
  walk through it.
- Topology graphing or visualization.
- Anything that alters network state — no route installation, no group joins, no
  interface changes.
- Cross-platform support. See *Constraints*.

---

## Constraints

These are decisions, not preferences. Treat a proposal that breaks one as disqualified
until the constraint itself is revisited.

| Constraint | Rationale |
|---|---|
| **Linux only** | The deployment target is Linux. Committing to it replaces the messiest part of this problem (cross-platform route enumeration) with a single well-trodden netlink call. Files carry `//go:build linux`. |
| **No cgo** | Must cross-compile with `CGO_ENABLED=0` and run in a scratch or distroless image. Every dependency below is pure Go, capture included. |
| **No external binaries** | No shelling out to `nmap`, `ip`, `tcpdump`, `lldpcli` or anything else at runtime. Those are fine for the pre-work spike; they are not dependencies. |
| **Read-only** | Enforce in review. State it in the README's first paragraph. |
| **`net/netip`, not `net.IP`** | Comparable, allocation-free, correct about zones. Retrofitting prefix types through a public API later is a breaking change. |

### On being Linux-only, honestly

Do not ship stub implementations for other platforms, and do not build an abstraction
layer awaiting a second OS. A library that silently returns empty results on macOS is
worse than one that refuses to compile there. If Darwin ever matters, a build-tagged
sibling file per source is the seam, and the `Source` interface already provides it.

---

## Pre-work: validate the yield before writing code

The open empirical question is whether anything on real segments actually volunteers
topology. Answer it with tools already present on a host. This is throwaway work and
produces no code.

```sh
# Local posture — what the kernel already knows.
ip -d route show table all
ip neigh show
ip -br addr

# LLDP frames (ethertype 0x88cc). Two minutes is enough; TTLs are typically 120s.
tcpdump -nn -v -i eth0 -s0 'ether[12:2] == 0x88cc'

# CDP frames (Cisco SNAP multicast).
tcpdump -nn -v -i eth0 -s0 'ether dst 01:00:0c:cc:cc:cc'

# IPv6 router advertisements (ICMPv6 type 134).
tcpdump -nn -v -i eth0 'icmp6 and ip6[40] == 134'
```

**What to look for:** a *management address* in an LLDP or CDP frame. That single field is
what turns "I have a default gateway" into "here is a device I can go ask properly", and
it is the main justification for the Listen tier. If three representative segments yield
nothing, that is worth knowing before building M2.

**Also capture the frames.** Save anything interesting with `tcpdump -w` — those become the
golden-file fixtures in `testdata/`, and collecting them is otherwise the most annoying
part of M2.

- [ ] Run the spike on at least three representative segments
- [ ] Record LLDP/CDP/RA yield per segment
- [ ] Save captured frames as `.pcap` for use as test fixtures
- [ ] Decide from the results whether M2 is worth its position in the order

---

## Architecture

### Tiers

Callers opt in by level. The default is `Local | Listen`.

| Tier | Cost | Mechanisms | Privilege |
|---|---|---|---|
| `Local` | zero packets | routes, interface prefixes, neighbour cache, netns identity, cloud IMDS | none |
| `Listen` | receive only | LLDP, CDP, IPv6 router advertisements | `CAP_NET_RAW` |
| `Ask` | a few packets | DHCPINFORM (opt 121/249), router solicitation, ARP sweep, ICMP | `CAP_NET_RAW` (see note) |

There is deliberately **no fourth tier**. See *Deliberately excluded*.

> ICMP echo can run unprivileged on Linux via `icmp.ListenPacket("udp4", ...)` when
> `net.ipv4.ping_group_range` permits it. Prefer that path and fall back to raw sockets.

### Core types

```go
type Source interface {
    Name() string
    Level() Level                // Local | Listen | Ask
    Available(context.Context) Availability
    Discover(context.Context) ([]Finding, error)
}

type Finding struct {
    Network    netip.Prefix
    Via        Nexthop      // Connected | Gateway | Tunnel | Advertised
    Source     string       // provenance; always populated
    Confidence Confidence   // High | Medium | Inferred
    Scannable  bool         // false for default routes and other unbounded prefixes
    Observed   time.Time
    Devices    []Device     // addr, name, platform, capabilities
}

type Report struct {
    Networks       []Finding
    Devices        []Device
    Sources        []SourceResult // Ran | Unavailable | Empty — with a reason
    NetNS          uint64         // inode of /proc/self/ns/net
    SuggestedScope []netip.Prefix
}
```

Three fields carry most of the design's weight:

- **`Scannable`** stops a default route from ever reaching a consumer as a scan target,
  while still recording that it exists and where it points.
- **`SourceResult`** separates an empty answer from an unavailable source. Without it a
  caller cannot tell an isolated host from a missing capability.
- **`NetNS`** exists because netlink answers for whichever namespace the process occupies.
  The same code returns the host's real posture under host networking and a meaningless
  bridge CIDR inside a NAT'd container, with nothing to distinguish them. Report the
  inode and let the caller weight or reject the answer.

### Package layout

```
/                     public types, Source interface, aggregator
/source/kernel        netlink: routes, neighbours, addresses, links
/source/imds          cloud metadata (AWS, Azure, GCP)
/source/dhcp          DHCPINFORM, options 121 and 249
/source/lldp          LLDP + CDP listener
/source/ra            IPv6 router advertisements
/source/arp           active ARP sweep
/source/icmp          echo and address-mask
/cmd/netscope         CLI — text and JSON output
/testdata             captured frames as golden fixtures
```

No `capture` package. With one `AF_PACKET` implementation there is nothing to abstract.

---

## Dependencies

| Module | Provides |
|---|---|
| `github.com/vishvananda/netlink` | `RouteList`, `NeighList`, `AddrList`, `LinkList` — the whole `Local` tier from one import |
| `github.com/mdlayher/packet` | pure-Go `AF_PACKET` capture |
| `github.com/gopacket/gopacket` | `layers.LinkLayerDiscoveryInfo` (SysName, **MgmtAddress**, PortDescription, SysCapabilities), `layers.CiscoDiscoveryInfo` (DeviceID, Addresses, Platform) |
| `github.com/mdlayher/ndp` | RFC 4861 router advertisements and solicitations |
| `github.com/mdlayher/arp` | RFC 826 ARP requests |
| `github.com/insomniacslk/dhcp` | `dhcpv4.ClasslessStaticRoute` per RFC 3442 |
| `golang.org/x/net/icmp` | echo and address-mask (type 17) |

Use the maintained **`github.com/gopacket/gopacket`** fork; `google/gopacket` is archived.
Only its `layers` decoders are needed — they are pure Go. The cgo-bound `pcap` package is
not used.

**Alternative to consider at M0:** `github.com/jsimonetti/rtnetlink` is cleaner,
lower-level and more actively maintained than `vishvananda/netlink`, at the cost of more
assembly for the same result. `vishvananda` is recommended first because it is ubiquitous
(Docker, most CNI plugins) and collapses four needs into one import. Both sit behind
`source/kernel`, so swapping later is contained.

### Protocol constants worth having to hand

| | |
|---|---|
| LLDP | ethertype `0x88CC`, dst `01:80:C2:00:00:0E`, TTL typically 120s |
| CDP | SNAP, dst `01:00:0C:CC:CC:CC` |
| ICMPv6 RA / RS | type 134 / type 133, all-routers `ff02::2` |
| DHCP | UDP 67/68, option 121 (RFC 3442), option 249 (Microsoft) |
| ICMP address mask | type 17 |

---

## Milestones

Each ships something usable on its own. Do not start the next until the current one is
tagged.

### M0 — model and local posture · `v0.1.0`

The foundation, and already useful: on most hosts the kernel alone answers the question.

- [ ] Repo init, module path, licence, README stating Linux-only and read-only
- [ ] `Level`, `Confidence`, `Nexthop`, `Availability`, `SourceResult` types
- [ ] `Source` interface, `Finding`, `Report`
- [ ] Aggregator: run selected sources, merge findings, reconcile duplicate prefixes
- [ ] `source/kernel` — routes, interface prefixes, neighbour cache
- [ ] Netns inode from `/proc/self/ns/net`
- [ ] `Scannable` classification (default routes, link-local, loopback, multicast)
- [ ] Unit tests with a faked netlink layer

**Acceptance:** on a multi-homed host, returns every connected prefix plus every
gateway-reachable prefix, marks the default route unscannable, and runs as an
unprivileged user with no capabilities.

### M1 — cheap authoritative sources · `v0.2.0`

Small, high-confidence, no capture stack.

- [ ] `source/imds` — AWS (`subnet-ipv4-cidr-block`, `vpc-ipv4-cidr-blocks`), Azure, GCP
- [ ] IMDS detection that fails fast off-cloud (short timeout, no retries)
- [ ] `source/dhcp` — DHCPINFORM, parse options 121 and 249
- [ ] Reconciliation: DHCP-advertised routes vs kernel routes

**Acceptance:** in a cloud VM, VPC CIDRs appear with `High` confidence. On a DHCP network
serving option 121, its routes appear. Off-cloud, `imds` reports `Unavailable` within
one second and the run is unaffected.

### M2 — listeners · `v0.3.0`

The highest-value release. This is where adjacent device management addresses appear.

- [ ] `AF_PACKET` socket setup via `mdlayher/packet`, with BPF filtering
- [ ] `source/lldp` — LLDP and CDP decode, extract management addresses
- [ ] `source/ra` — RA listening, extract prefix information options
- [ ] Bounded listen windows with context cancellation
- [ ] `CAP_NET_RAW` detection reported as `Unavailable`, never a hard failure
- [ ] Golden-file decode tests from the fixtures captured during pre-work

**Acceptance:** on a segment with LLDP-speaking kit, returns the neighbour's system name
and management address. Without `CAP_NET_RAW`, reports the source unavailable and the
other tiers still produce a full report.

### M3 — active probes · `v0.4.0`

- [ ] `source/arp` — sweep connected prefixes only, rate-limited, concurrency-capped
- [ ] `source/icmp` — echo, preferring the unprivileged `udp4` path; address-mask
- [ ] Guard against sweeping anything larger than a configurable prefix size

**Acceptance:** ARP sweep of a `/24` completes in a few seconds and finds hosts the
neighbour cache had not seen. Never probes a prefix the `Local` tier did not establish.

### M4 — derived views and CLI · `v1.0.0`

- [ ] Confidence reconciliation when sources disagree about a prefix
- [ ] `SuggestedScope` — the ranked, scannable, deduplicated answer
- [ ] `cmd/netscope` — text and JSON output, `--level`, `--iface`, `--timeout`
- [ ] Exit codes that distinguish "nothing found" from "could not look"
- [ ] README with real output samples
- [ ] Godoc on every exported symbol

**Acceptance:** `netscope --level=local,listen` prints a readable report on a bare host,
and `--json` emits something a caller can consume without parsing prose.

---

## Testing and CI

- **No live network in tests.** Every protocol decode is tested against golden files in
  `testdata/`. Fake the netlink and capture layers behind narrow internal interfaces.
- **Fixtures come from the pre-work spike.** Real frames from real kit beat synthesized
  ones, particularly for CDP and vendor LLDP TLVs.
- **Table-driven decode tests**, one case per fixture, asserting the full `Finding`.
- **`go vet`, `staticcheck`, `golangci-lint`** in CI. Race detector on.
- **Build check with `CGO_ENABLED=0`** as an explicit CI step — this is a stated
  constraint and deserves a gate, not a convention.
- **A privileged integration job** (network namespaces via `ip netns`, or a container with
  `CAP_NET_RAW`) exercising the Listen and Ask tiers against a synthetic peer. Nice to
  have; not a blocker for `v1.0.0`.

---

## Open decisions

Resolve at the milestone noted; do not let them block earlier work.

1. **`vishvananda/netlink` vs `jsimonetti/rtnetlink`** — M0. Recommendation above; revisit
   only if dependency weight becomes a real problem.
2. **IPv6 depth** — M0/M2. Full parity from the start, or IPv4 first with IPv6 arriving
   with the RA source? Leaning parity, since `netip` makes it mostly free in the model.
3. **Policy routing and multiple tables** — M0. `ip route show table all` can return a lot.
   Report all tables with the table ID on each finding, or only the main table? Leaning
   all-with-attribution, because VRF-style setups are exactly where this library earns
   its keep.
4. **Caching and staleness** — M4. Should a `Report` be cacheable with a TTL, or is every
   call a fresh observation? Affects whether `Observed` is per-finding or per-report.
5. **Confidence when sources disagree** — M4. Highest wins, or does agreement across two
   independent sources promote a finding?

---

## Deliberately excluded

Recorded so the reasoning is not re-litigated later.

- **Joining a routing control plane** (OSPF adjacency, EIGRP, RIP solicitation) to pull a
  router's route table. Cheaply reachable only through nmap's NSE scripts; implementing it
  natively is weeks of protocol work for a capability that changes network state and
  invites blame for the next outage. If a router's table is ever genuinely needed, RIPv2
  is the one tractable exception — a plain UDP/520 request that many routers answer.
- **Cross-platform route enumeration.** `golang.org/x/net/route` is BSD/Darwin only;
  Windows needs a hand-written `GetIpForwardTable2` binding; `libp2p/go-netroute` unifies
  them but is query-only (`Route(dst)` answers "how do I reach X", not "what can I
  reach"); `tailscale.com/net/routetable` solves it properly but carries the entire
  `tailscale.com` module.
- **libpcap.** Only ever needed for cross-platform capture. `AF_PACKET` is enough here.
- **mDNS, SSDP, NetBIOS and other service discovery.** Finds hosts, not networks, and
  drags in a lot of surface for little scope information.
