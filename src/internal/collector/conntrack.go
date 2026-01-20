package collector

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"conntrack-exporter/internal/conntrack"
	"conntrack-exporter/internal/ports"
	"conntrack-exporter/internal/procfs"
)

// ConntrackCollector periodically reads `/proc/net/nf_conntrack` and maintains
// a cached set of Prometheus metrics.
//
// Design notes:
// - Per-connection metrics are Gauges, because conntrack provides snapshots.
// - On each refresh we RESET the GaugeVecs, effectively deleting old label pairs.
// - Total metrics are Counters and are NOT reset between refreshes.
//   They are incremented by deltas between snapshots.
//
// Label set is fixed for all metrics:
//   src, dst, l3protocol, l4protocol, l7protocol, dport
//
// For protocols without ports (icmp, etc) we use:
//   dport="0", l7protocol="na"
type ConntrackCollector struct {
	procfsFS procfs.FS
	interval time.Duration

	mu   sync.Mutex
	prev map[key]aggValues
	seen map[key]struct{}

	// Per-connection snapshot metrics (GaugeVec) - reset on each update.
	sentPackets  *prometheus.GaugeVec
	sentBytes    *prometheus.GaugeVec
	replyPackets *prometheus.GaugeVec
	replyBytes   *prometheus.GaugeVec

	// Totals (CounterVec) - monotonic, never reset.
	totalConnections  *prometheus.CounterVec
	totalSentPackets  *prometheus.CounterVec
	totalSentBytes    *prometheus.CounterVec
	totalReplyPackets *prometheus.CounterVec
	totalReplyBytes   *prometheus.CounterVec

	stopCh chan struct{}
	doneCh chan struct{}
}

var labelNames = []string{"src", "dst", "l3protocol", "l4protocol", "l7protocol", "dport"}

type key struct {
	Src, Dst string
	L3, L4   string
	DPort    string
	L7       string
}

type aggValues struct {
	SentPackets  uint64
	SentBytes    uint64
	ReplyPackets uint64
	ReplyBytes   uint64
}

func NewConntrackCollector(procfsFS procfs.FS, interval time.Duration) *ConntrackCollector {
	c := &ConntrackCollector{
		procfsFS: procfsFS,
		interval: interval,
		prev:     map[key]aggValues{},
		seen:     map[key]struct{}{},
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}

	c.sentPackets = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "conntrack_sent_packets",
		Help: "Number of packets sent (original direction) for the aggregated conntrack key.",
	}, labelNames)
	c.sentBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "conntrack_sent_bytes",
		Help: "Number of bytes sent (original direction) for the aggregated conntrack key.",
	}, labelNames)
	c.replyPackets = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "conntrack_reply_packets",
		Help: "Number of packets received (reply direction) for the aggregated conntrack key.",
	}, labelNames)
	c.replyBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "conntrack_reply_bytes",
		Help: "Number of bytes received (reply direction) for the aggregated conntrack key.",
	}, labelNames)

	c.totalConnections = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "conntrack_total_connections",
		Help: "Total number of aggregated conntrack keys observed since exporter start.",
	}, labelNames)
	c.totalSentPackets = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "conntrack_total_sent_packets",
		Help: "Monotonic total of sent packets (original direction), accumulated by deltas between snapshots.",
	}, labelNames)
	c.totalSentBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "conntrack_total_sent_bytes",
		Help: "Monotonic total of sent bytes (original direction), accumulated by deltas between snapshots.",
	}, labelNames)
	c.totalReplyPackets = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "conntrack_total_reply_packets",
		Help: "Monotonic total of reply packets (reply direction), accumulated by deltas between snapshots.",
	}, labelNames)
	c.totalReplyBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "conntrack_total_reply_bytes",
		Help: "Monotonic total of reply bytes (reply direction), accumulated by deltas between snapshots.",
	}, labelNames)

	return c
}

// MustRegister registers all metrics into the provided registry.
func (c *ConntrackCollector) MustRegister(reg prometheus.Registerer) {
	reg.MustRegister(
		c.sentPackets,
		c.sentBytes,
		c.replyPackets,
		c.replyBytes,
		c.totalConnections,
		c.totalSentPackets,
		c.totalSentBytes,
		c.totalReplyPackets,
		c.totalReplyBytes,
	)
}

// Start begins periodic collection in a background goroutine.
// It performs an initial update immediately.
func (c *ConntrackCollector) Start(ctx context.Context) {
	go func() {
		defer close(c.doneCh)

		// Initial update.
		_ = c.UpdateOnce(ctx)

		t := time.NewTicker(c.interval)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			case <-t.C:
				_ = c.UpdateOnce(ctx)
			}
		}
	}()
}

func (c *ConntrackCollector) Stop() {
	close(c.stopCh)
	<-c.doneCh
}

// UpdateOnce reads conntrack file and updates metrics.
func (c *ConntrackCollector) UpdateOnce(ctx context.Context) error {
	_ = ctx // reserved for future (e.g. timeouts around file reads)

	raw, err := c.procfsFS.ReadFile("net/nf_conntrack")
	if err != nil {
		return err
	}

	snapshot, err := parseAndAggregate(raw)
	if err != nil {
		return err
	}

	c.applySnapshot(snapshot)
	return nil
}

func parseAndAggregate(raw []byte) (map[key]aggValues, error) {
	out := map[key]aggValues{}

	sc := bufio.NewScanner(bytes.NewReader(raw))
	// conntrack lines are typically below 4K, but let's be safe.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var any bool
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		e, ok := conntrack.ParseLine(line)
		if !ok {
			continue
		}
		any = true

		dport := e.Original.Dport
		l7 := ports.L7ProtocolFromDPort(dport)

		// Protocols without ports: use explicit values as agreed.
		if !e.HasPorts() {
			dport = "0"
			l7 = "na"
		}

		k := key{
			Src:  e.Original.SrcIP,
			Dst:  e.Original.DstIP,
			L3:   e.L3Proto,
			L4:   e.L4Proto,
			DPort: dport,
			L7:   l7,
		}

		v := out[k]
		v.SentPackets += e.OriginalStats.Packets
		v.SentBytes += e.OriginalStats.Bytes
		v.ReplyPackets += e.ReplyStats.Packets
		v.ReplyBytes += e.ReplyStats.Bytes
		out[k] = v
	}

	if err := sc.Err(); err != nil {
		return nil, err
	}
	if !any {
		return nil, errors.New("no conntrack entries parsed from nf_conntrack")
	}

	return out, nil
}

func (c *ConntrackCollector) applySnapshot(cur map[key]aggValues) {
	// Reset per-connection metrics (delete previous label pairs).
	c.sentPackets.Reset()
	c.sentBytes.Reset()
	c.replyPackets.Reset()
	c.replyBytes.Reset()

	// Update per-connection gauges.
	for k, v := range cur {
		labels := labelValues(k)
		c.sentPackets.WithLabelValues(labels...).Set(float64(v.SentPackets))
		c.sentBytes.WithLabelValues(labels...).Set(float64(v.SentBytes))
		c.replyPackets.WithLabelValues(labels...).Set(float64(v.ReplyPackets))
		c.replyBytes.WithLabelValues(labels...).Set(float64(v.ReplyBytes))
	}

	// Totals require previous snapshot and seen set; protect with a mutex.
	c.mu.Lock()
	defer c.mu.Unlock()

	for k, v := range cur {
		labels := labelValues(k)

		if _, ok := c.seen[k]; !ok {
			c.totalConnections.WithLabelValues(labels...).Inc()
			c.seen[k] = struct{}{}
		}

		prev := c.prev[k]

		// Accumulate only positive deltas to keep counters monotonic.
		if d := delta(prev.SentPackets, v.SentPackets); d > 0 {
			c.totalSentPackets.WithLabelValues(labels...).Add(float64(d))
		}
		if d := delta(prev.SentBytes, v.SentBytes); d > 0 {
			c.totalSentBytes.WithLabelValues(labels...).Add(float64(d))
		}
		if d := delta(prev.ReplyPackets, v.ReplyPackets); d > 0 {
			c.totalReplyPackets.WithLabelValues(labels...).Add(float64(d))
		}
		if d := delta(prev.ReplyBytes, v.ReplyBytes); d > 0 {
			c.totalReplyBytes.WithLabelValues(labels...).Add(float64(d))
		}
	}

	// Drop prev entries for keys that no longer exist.
	// If a key disappears and later re-appears (new connections),
	// we want to treat the new snapshot as a fresh baseline.
	for k := range c.prev {
		if _, ok := cur[k]; !ok {
			delete(c.prev, k)
		}
	}

	// Replace prev snapshot with current values.
	for k, v := range cur {
		c.prev[k] = v
	}
}

func labelValues(k key) []string {
	return []string{k.Src, k.Dst, k.L3, k.L4, k.L7, k.DPort}
}

func delta(prev, cur uint64) uint64 {
	if cur < prev {
		// Counter reset in conntrack table (connection restart),
		// or key baseline was dropped; treat as 0 to preserve monotonicity.
		return 0
	}
	return cur - prev
}

