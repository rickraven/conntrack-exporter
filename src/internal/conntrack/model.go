package conntrack

// This package is responsible ONLY for parsing `/proc/net/nf_conntrack` lines
// into a structured representation that the collector can use.
//
// IMPORTANT:
// - Keep this package free from Prometheus dependencies.
// - Keep parsing logic tolerant to missing fields: nf_conntrack format varies
//   by kernel/config/protocol (e.g. nf_conntrack_acct may be disabled).

// DirectionStats describes bytes/packets counters in one direction.
// In conntrack terminology this is usually the "original" direction and
// the "reply" direction.
type DirectionStats struct {
	Packets uint64
	Bytes   uint64
}

// ConntrackTuple describes a network tuple inside the conntrack entry.
// For protocols without ports (e.g. ICMP), Sport/Dport will be empty.
type ConntrackTuple struct {
	SrcIP string
	DstIP string
	Sport string
	Dport string
}

// Entry is a parsed representation of a single line from `/proc/net/nf_conntrack`.
//
// We store l3/l4 protocol strings as they appear in the file (e.g. "ipv4", "tcp").
type Entry struct {
	L3Proto string // e.g. ipv4, ipv6
	L4Proto string // e.g. tcp, udp, icmp

	Original ConntrackTuple
	Reply    ConntrackTuple

	OriginalStats DirectionStats
	ReplyStats    DirectionStats
}

// HasPorts reports whether this entry has L4 ports (sport/dport) in the conntrack file.
func (e Entry) HasPorts() bool {
	return e.Original.Dport != "" || e.Original.Sport != ""
}

