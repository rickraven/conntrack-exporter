module conntrack-exporter

go 1.25

require (
	github.com/prometheus/client_golang v1.23.2
)

// We keep module sources under src/. Internal imports use the module name
// (conntrack-exporter/...) to avoid relying on relative paths.

