package enrich

import (
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/oschwald/maxminddb-golang"
)

// GeoCache provides best-effort enrichment of IP addresses using MaxMind MMDB files.
//
// IMPORTANT requirements mapping (from AGENTS.md):
// - Labels must be ABSENT if the corresponding MMDB path is NOT provided in config.
//   This is enforced by the collector by not creating the label at all.
// - If the label exists (path is provided) but lookup fails / data is missing,
//   the label value must be an empty string.
//
// We also cache lookups (including negative results) to avoid repeatedly decoding
// MMDB records on each conntrack snapshot refresh.
type GeoCache struct {
	asPath      string
	countryPath string

	asReader      *maxminddb.Reader
	countryReader *maxminddb.Reader

	mu           sync.RWMutex
	asCache      map[string]string
	countryCache map[string]string
}

// NewGeoCache opens MMDB readers if paths are non-empty.
//
// If a non-empty path cannot be opened, we return a cache instance with a nil reader:
// lookups will just return empty strings (as required), while the caller may log
// the open error.
func NewGeoCache(asPath, countryPath string) (*GeoCache, error) {
	g := &GeoCache{
		asPath:        asPath,
		countryPath:   countryPath,
		asCache:       map[string]string{},
		countryCache:  map[string]string{},
		asReader:      nil,
		countryReader: nil,
	}

	var firstErr error

	// Open asn DB.
	if asPath != "" {
		r, err := maxminddb.Open(asPath)
		if err != nil {
			// Non-fatal: lookups will return empty strings.
			if firstErr == nil {
				firstErr = err
			}
		} else {
			g.asReader = r
		}
	}

	// Open country DB.
	if countryPath != "" {
		r, err := maxminddb.Open(countryPath)
		if err != nil {
			// Non-fatal: lookups will return empty strings.
			if firstErr == nil {
				firstErr = err
			}
		} else {
			g.countryReader = r
		}
	}

	return g, firstErr
}

func (g *GeoCache) Close() error {
	// Close is best-effort; this exporter is long-lived and typically closes only on exit.
	var first error
	if g.asReader != nil {
		if err := g.asReader.Close(); err != nil && first == nil {
			first = err
		}
	}
	if g.countryReader != nil {
		if err := g.countryReader.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// ASN returns the autonomous system label value in the format:
//
//   "<autonomous_system_number>_<autonomous_system_organization>"
//
// Example: "12345_Example Org"
//
// Empty string means "unknown" / "not available" (required by label rules).
func (g *GeoCache) ASN(ipStr string) string {
	// A missing reader means the path was not provided or could not be opened.
	if g.asReader == nil {
		return ""
	}

	g.mu.RLock()
	if v, ok := g.asCache[ipStr]; ok {
		g.mu.RUnlock()
		return v
	}
	g.mu.RUnlock()

	ip := net.ParseIP(ipStr)
	if ip == nil {
		g.mu.Lock()
		g.asCache[ipStr] = ""
		g.mu.Unlock()
		return ""
	}

	// GeoLite2-ASN schema.
	type asnRecord struct {
		ASN uint   `maxminddb:"autonomous_system_number"`
		Org string `maxminddb:"autonomous_system_organization"`
	}
	var rec asnRecord
	if err := g.asReader.Lookup(ip, &rec); err != nil {
		g.mu.Lock()
		g.asCache[ipStr] = ""
		g.mu.Unlock()
		return ""
	}

	val := ""
	if rec.ASN != 0 {
		// Organization name may contain spaces and other whitespace. The project
		// requirement is to NOT use spaces in label values, so we normalize any
		// whitespace runs to a single underscore.
		asn := strconv.FormatUint(uint64(rec.ASN), 10)
		org := strings.TrimSpace(rec.Org)
		if org == "" {
			val = asn
		} else {
			org = strings.Join(strings.Fields(org), "_")
			val = asn + "_" + org
		}
	}

	g.mu.Lock()
	g.asCache[ipStr] = val
	g.mu.Unlock()
	return val
}

// Country returns ISO country code (e.g. "US").
// Empty string means "unknown" / "not available" (required by label rules).
func (g *GeoCache) Country(ipStr string) string {
	if g.countryReader == nil {
		return ""
	}

	g.mu.RLock()
	if v, ok := g.countryCache[ipStr]; ok {
		g.mu.RUnlock()
		return v
	}
	g.mu.RUnlock()

	ip := net.ParseIP(ipStr)
	if ip == nil {
		g.mu.Lock()
		g.countryCache[ipStr] = ""
		g.mu.Unlock()
		return ""
	}

	// GeoLite2-Country schema (use "country" primarily, fallback to "registered_country").
	type countryPart struct {
		ISOCode string `maxminddb:"iso_code"`
	}
	type countryRecord struct {
		Country          countryPart `maxminddb:"country"`
		RegisteredCountry countryPart `maxminddb:"registered_country"`
	}
	var rec countryRecord
	if err := g.countryReader.Lookup(ip, &rec); err != nil {
		g.mu.Lock()
		g.countryCache[ipStr] = ""
		g.mu.Unlock()
		return ""
	}

	val := rec.Country.ISOCode
	if val == "" {
		val = rec.RegisteredCountry.ISOCode
	}

	g.mu.Lock()
	g.countryCache[ipStr] = val
	g.mu.Unlock()
	return val
}

// PathExists is a tiny helper used by the caller to decide whether to warn loudly.
// We keep it here to avoid sprinkling os.Stat across packages.
func PathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

