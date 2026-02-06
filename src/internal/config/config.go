package config

import (
	"flag"
	"strings"
	"time"
)

// Config holds runtime configuration for the exporter.
type Config struct {
	CollectorInterval time.Duration
	CollectorGetPTR   bool
	ConfigureAcct     bool
	ProcfsPath        string
	MMDBASPath        string
	MMDBCountryPath   string

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
	flag.BoolVar(&cfg.CollectorGetPTR, "collector.get-ptr", false, "Execute DNS queries to retrieve PTR records.")
	flag.BoolVar(&cfg.ConfigureAcct, "configure.nf_conntrack_acct", false, "Set systemctl variable to store packets/bytes counts.")
	flag.StringVar(&cfg.ProcfsPath, "path.procfs", "/proc", "Procfss mountpoint.")

	// MMDB paths are OPTIONAL. Per AGENTS.md label rules:
	// - the label must be ABSENT unless the user explicitly configures the MMDB path
	// - if configured but lookup fails => label value must be an empty string
	//
	// To support both "nice defaults in --help output" and "disabled unless explicitly set",
	// we use a tracked flag type that records whether the user provided the flag.
	var mmdbAS trackedString = trackedString{value: "/usr/share/conntrack-exporter/as.mmdb"}
	var mmdbCountry trackedString = trackedString{value: "/usr/share/conntrack-exporter/country.mmdb"}
	flag.Var(&mmdbAS, "path.mmdbas", "AS maxmind database.")
	flag.Var(&mmdbCountry, "path.mmdbcountry", "GeoIP maxmind database.")

	flag.StringVar(&cfg.WebTelemetryPath, "web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	flag.BoolVar(&cfg.WebDisableExporterMetrics, "web.disable-exporter-metrics", false, "Exclude metrics about the exporter itself (promhttp_*, process_*, go_*).")
	flag.IntVar(&cfg.WebMaxRequests, "web.max-requests", 40, "Maximum number of parallel scrape requests. Use 0 to disable.")
	flag.Var(&cfg.WebListenAddresses, "web.listen-address", "Addresses on which to expose metrics and web interface. Repeatable for multiple addresses. Examples: :9095 or [::1]:9095")

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
		cfg.WebListenAddresses = append(cfg.WebListenAddresses, ":9095")
	}

	// Only enable MMDB-based labels when the user explicitly set the flag.
	if mmdbAS.set && mmdbAS.value != "" {
		cfg.MMDBASPath = mmdbAS.value
	} else {
		cfg.MMDBASPath = ""
	}
	if mmdbCountry.set && mmdbCountry.value != "" {
		cfg.MMDBCountryPath = mmdbCountry.value
	} else {
		cfg.MMDBCountryPath = ""
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

// trackedString is a flag.Value that remembers whether it was explicitly set.
// This allows us to distinguish between "default value" and "user provided".
type trackedString struct {
	value string
	set   bool
}

func (t *trackedString) String() string { return t.value }

func (t *trackedString) Set(v string) error {
	t.value = v
	t.set = true
	return nil
}

