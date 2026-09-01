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
