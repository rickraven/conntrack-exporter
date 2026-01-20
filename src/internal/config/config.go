package config

import (
	"flag"
	"strings"
	"time"
)

// Config holds runtime configuration for the exporter.
type Config struct {
	CollectorInterval time.Duration
	ConfigureAcct     bool
	ProcfsPath        string

	WebTelemetryPath         string
	WebDisableExporterMetrics bool
	WebMaxRequests           int
	WebListenAddresses       multiString

	LogLevel  string
	LogFormat string

	ShowHelp    bool
	ShowVersion bool
}

// ParseFlags parses CLI flags according to AGENTS.md requirements.
func ParseFlags() Config {
	var cfg Config

	// Note: We keep flag names identical to the spec; many are compatible
	// with Prometheus exporter conventions.

	intervalSeconds := flag.Int("collector.interval", 60, "Seconds between collecting info about connections.")
	flag.BoolVar(&cfg.ConfigureAcct, "configure.nf_conntrack_acct", false, "Set systemctl variable to store packets/bytes counts.")
	flag.StringVar(&cfg.ProcfsPath, "path.procfs", "/proc", "Procfss mountpoint.")

	flag.StringVar(&cfg.WebTelemetryPath, "web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	flag.BoolVar(&cfg.WebDisableExporterMetrics, "web.disable-exporter-metrics", false, "Exclude metrics about the exporter itself (promhttp_*, process_*, go_*).")
	flag.IntVar(&cfg.WebMaxRequests, "web.max-requests", 40, "Maximum number of parallel scrape requests. Use 0 to disable.")
	flag.Var(&cfg.WebListenAddresses, "web.listen-address", "Addresses on which to expose metrics and web interface. Repeatable for multiple addresses. Examples: :9100 or [::1]:9100")

	flag.StringVar(&cfg.LogLevel, "log.level", "info", "Only log messages with the given severity or above. One of: [debug, info, warn, error]")
	flag.StringVar(&cfg.LogFormat, "log.format", "logfmt", "Output format of log messages. One of: [logfmt, json]")

	// Help flags (Go's flag package supports -h/-help, but we explicitly provide
	// -h and --help as requested in AGENTS.md).
	flag.BoolVar(&cfg.ShowHelp, "h", false, "Show help and exit.")
	flag.BoolVar(&cfg.ShowHelp, "help", false, "Show help and exit.")

	// Aliases.
	flag.BoolVar(&cfg.ShowVersion, "v", false, "Show application version and exit.")
	flag.BoolVar(&cfg.ShowVersion, "version", false, "Show application version and exit.")

	flag.Parse()

	cfg.CollectorInterval = time.Duration(*intervalSeconds) * time.Second
	if len(cfg.WebListenAddresses) == 0 {
		cfg.WebListenAddresses = append(cfg.WebListenAddresses, ":9100")
	}

	return cfg
}

type multiString []string

func (m *multiString) String() string {
	if m == nil {
		return ""
	}
	return strings.Join(*m, ",")
}

func (m *multiString) Set(value string) error {
	*m = append(*m, value)
	return nil
}

