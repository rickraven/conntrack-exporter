package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"conntrack-exporter/internal/collector"
	"conntrack-exporter/internal/config"
	"conntrack-exporter/internal/logging"
	"conntrack-exporter/internal/procfs"
	"conntrack-exporter/internal/sysctl"
	"conntrack-exporter/internal/web"
)

// Run wires the application together and blocks until termination.
func Run(cfg config.Config, version string) int {
	level, err := logging.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = logging.Info
	}
	format, err := logging.ParseFormat(cfg.LogFormat)
	if err != nil {
		format = logging.Logfmt
	}
	log := logging.New(os.Stderr, level, format)

	if cfg.ShowHelp {
		// We delegate help rendering to the flag package in main.
		return 0
	}
	if cfg.ShowVersion {
		log.Info("version", "version", version)
		return 0
	}

	pfs := procfs.FS{Root: cfg.ProcfsPath}

	// sysctl check/configure.
	if cfg.ConfigureAcct {
		if err := sysctl.ConfigureNfConntrackAcct(pfs); err != nil {
			log.Warn("failed to configure nf_conntrack_acct", "err", err)
		} else {
			log.Info("configured nf_conntrack_acct", "value", 1)
		}
	}

	acct, err := sysctl.ReadNfConntrackAcct(pfs)
	if err != nil {
		log.Warn("failed to read nf_conntrack_acct", "err", err)
	} else if acct == 0 {
		log.Warn("nf_conntrack_acct is disabled; packets/bytes may be missing in nf_conntrack")
	}

	// Prometheus registry and exporter metrics control.
	reg := prometheus.NewRegistry()
	if !cfg.WebDisableExporterMetrics {
		reg.MustRegister(
			prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
			prometheus.NewGoCollector(),
		)
	}

	ctCollector := collector.NewConntrackCollector(pfs, cfg.CollectorInterval)
	ctCollector.MustRegister(reg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Stop on SIGINT/SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	ctCollector.Start(ctx)

	srv := &web.Server{
		Logger:          log,
		Registry:        reg,
		TelemetryPath:   cfg.WebTelemetryPath,
		ListenAddrs:     cfg.WebListenAddresses,
		MaxRequests:     cfg.WebMaxRequests,
		DisableExpMetrics: cfg.WebDisableExporterMetrics,
	}

	// Run HTTP server (blocks). When it returns, stop collector.
	err = srv.Start(ctx)
	ctCollector.Stop()

	if err != nil {
		log.Error("http server error", "err", err)
		// Non-zero to indicate runtime error.
		return 1
	}

	// Give background goroutines a tiny moment to flush logs (best-effort).
	time.Sleep(10 * time.Millisecond)
	return 0
}

