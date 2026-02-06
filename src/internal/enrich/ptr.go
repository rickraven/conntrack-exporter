package enrich

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"
)

// PTRCache caches reverse DNS (PTR) lookups for IP addresses.
//
// Requirements mapping:
// - If --collector.get-ptr is NOT set, the collector must NOT create the "ptr" label.
// - If the "ptr" label exists, but no PTR record exists or lookup fails,
//   the value must be an empty string.
//
// We cache both positive and negative results to avoid repeated DNS queries.
type PTRCache struct {
	resolver *net.Resolver
	timeout  time.Duration

	mu    sync.RWMutex
	cache map[string]string
}

func NewPTRCache(timeout time.Duration) *PTRCache {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &PTRCache{
		resolver: net.DefaultResolver,
		timeout:  timeout,
		cache:    map[string]string{},
	}
}

// PTR returns the first PTR hostname for ipStr, without a trailing dot.
// Empty string means "no PTR" or "lookup failed".
func (p *PTRCache) PTR(ctx context.Context, ipStr string) string {
	p.mu.RLock()
	if v, ok := p.cache[ipStr]; ok {
		p.mu.RUnlock()
		return v
	}
	p.mu.RUnlock()

	if ctx == nil {
		ctx = context.Background()
	}
	lookupCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	names, err := p.resolver.LookupAddr(lookupCtx, ipStr)
	if err != nil || len(names) == 0 {
		p.mu.Lock()
		p.cache[ipStr] = ""
		p.mu.Unlock()
		return ""
	}

	// DNS PTR answers commonly have a trailing dot; we normalize it.
	v := strings.TrimSuffix(names[0], ".")

	p.mu.Lock()
	p.cache[ipStr] = v
	p.mu.Unlock()
	return v
}

