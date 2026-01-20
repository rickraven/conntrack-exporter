package conntrack

import (
	"strconv"
	"strings"
)

// ParseLine parses a single line from `/proc/net/nf_conntrack`.
//
// The format is not a strict key=value-only format. It usually begins with
// a few positional tokens, then contains repeated key=value tokens:
//
//   ipv4 2 tcp 6 431999 ESTABLISHED src=... dst=... sport=... dport=...
//     packets=... bytes=... src=... dst=... sport=... dport=... packets=... bytes=...
//     [mark=.. zone=.. use=..]
//
// We intentionally implement a tolerant parser:
// - missing packets/bytes (nf_conntrack_acct=0) => counters become 0
// - protocols without ports (icmp) => sport/dport remain empty
//
// NOTE: This parser does not attempt to validate IP formats. The collector
// will treat them as opaque label values.
func ParseLine(line string) (Entry, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Entry{}, false
	}

	fields := strings.Fields(line)
	if len(fields) < 1 {
		return Entry{}, false
	}

	var e Entry

	// Positional tokens: token0 is usually l3 protocol ("ipv4"/"ipv6").
	e.L3Proto = fields[0]
	// token2 is usually l4 protocol ("tcp"/"udp"/"icmp"). If unavailable, keep empty.
	if len(fields) >= 3 {
		e.L4Proto = fields[2]
	}

	// Collect occurrences of repeated keys in the order they appear.
	var (
		srcs    []string
		dsts    []string
		sports  []string
		dports  []string
		packets []uint64
		bytes   []uint64
	)

	for _, f := range fields {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			continue
		}

		switch k {
		case "src":
			srcs = append(srcs, v)
		case "dst":
			dsts = append(dsts, v)
		case "sport":
			sports = append(sports, v)
		case "dport":
			dports = append(dports, v)
		case "packets":
			if n, ok := parseUint64(v); ok {
				packets = append(packets, n)
			} else {
				packets = append(packets, 0)
			}
		case "bytes":
			if n, ok := parseUint64(v); ok {
				bytes = append(bytes, n)
			} else {
				bytes = append(bytes, 0)
			}
		}
	}

	// Original tuple (first occurrences).
	if len(srcs) >= 1 {
		e.Original.SrcIP = srcs[0]
	}
	if len(dsts) >= 1 {
		e.Original.DstIP = dsts[0]
	}
	if len(sports) >= 1 {
		e.Original.Sport = sports[0]
	}
	if len(dports) >= 1 {
		e.Original.Dport = dports[0]
	}
	if len(packets) >= 1 {
		e.OriginalStats.Packets = packets[0]
	}
	if len(bytes) >= 1 {
		e.OriginalStats.Bytes = bytes[0]
	}

	// Reply tuple (second occurrences).
	if len(srcs) >= 2 {
		e.Reply.SrcIP = srcs[1]
	}
	if len(dsts) >= 2 {
		e.Reply.DstIP = dsts[1]
	}
	if len(sports) >= 2 {
		e.Reply.Sport = sports[1]
	}
	if len(dports) >= 2 {
		e.Reply.Dport = dports[1]
	}
	if len(packets) >= 2 {
		e.ReplyStats.Packets = packets[1]
	}
	if len(bytes) >= 2 {
		e.ReplyStats.Bytes = bytes[1]
	}

	// We consider an entry valid if it at least has L3 proto and src/dst.
	if e.L3Proto == "" || e.Original.SrcIP == "" || e.Original.DstIP == "" {
		return Entry{}, false
	}

	return e, true
}

func parseUint64(s string) (uint64, bool) {
	// conntrack uses base-10 numbers.
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

