package dns

import (
	"context"
	"net"
	"sync"
	"time"

	madns "github.com/multiformats/go-multiaddr-dns"

	"github.com/onflow/flow-go/module"
)

type Resolver struct {
	sync.RWMutex
	ttl       time.Duration
	res       madns.BasicResolver
	collector module.NetworkMetrics
	ipCache   map[string]*ipCacheEntry
	txtCache  map[string]*txtCacheEntry
}

type ipCacheEntry struct {
	addresses []net.IPAddr
	timestamp time.Time
}

type txtCacheEntry struct {
	addresses []string
	timestamp time.Time
}

func NewResolver(collector module.NetworkMetrics) (*madns.Resolver, error) {
	return madns.NewResolver(madns.WithDefaultResolver(&Resolver{
		res:       madns.DefaultResolver,
		collector: collector,
	}))
}

// LookupIPAddr implements BasicResolver interface for libp2p.
func (r *Resolver) LookupIPAddr(ctx context.Context, domain string) ([]net.IPAddr, error) {
	started := time.Now()

	addr, err := r.lookupIPAddr(ctx, domain)

	r.collector.DNSLookupDuration(time.Since(started))
	return addr, err
}

func (r *Resolver) lookupIPAddr(ctx context.Context, domain string) ([]net.IPAddr, error) {
	if addr, ok := r.resolveIPCache(domain); ok {
		// resolving address from cache
		return addr, nil
	}

	// resolves domain through underlying resolver
	addr, err := r.res.LookupIPAddr(ctx, domain)
	if err != nil {
		return nil, err
	}

	r.updateIPCache(domain, addr) // updates cache

	return addr, nil
}

// LookupTXT implements BasicResolver interface for libp2p.
func (r *Resolver) LookupTXT(ctx context.Context, txt string) ([]string, error) {
	started := time.Now()

	addr, err := r.lookupTXT(ctx, txt)

	r.collector.DNSLookupDuration(time.Since(started))
	return addr, err
}

func (r *Resolver) lookupTXT(ctx context.Context, txt string) ([]string, error) {
	if addr, ok := r.resolveTXTCache(txt); ok {
		// resolving address from cache
		return addr, nil
	}

	// resolves dtxt through underlying resolver
	addr, err := r.res.LookupTXT(ctx, txt)
	if err != nil {
		return nil, err
	}

	r.updateTXTCache(txt, addr) // updates cache

	return addr, err
}

// resolveIPCache resolves the domain through the cache if it is available.
func (r *Resolver) resolveIPCache(domain string) ([]net.IPAddr, bool) {
	r.Lock()
	defer r.Unlock()

	entry, ok := r.ipCache[domain]

	if !ok {
		return nil, false
	}

	if time.Now().After(entry.timestamp.Add(r.ttl)) {
		// invalidates cache entry
		delete(r.ipCache, domain)
		return nil, false
	}

	return entry.addresses, true
}

// resolveIPCache resolves the txt through the cache if it is available.
func (r *Resolver) resolveTXTCache(txt string) ([]string, bool) {
	r.Lock()
	defer r.Unlock()

	entry, ok := r.txtCache[txt]

	if !ok {
		return nil, false
	}

	if time.Now().After(entry.timestamp.Add(r.ttl)) {
		// invalidates cache entry
		delete(r.txtCache, txt)
		return nil, false
	}

	return entry.addresses, true
}

// updateIPCache updates the cache entry for the domain.
func (r *Resolver) updateIPCache(domain string, addr []net.IPAddr) {
	r.Lock()
	defer r.Unlock()

	r.ipCache[domain] = &ipCacheEntry{
		addresses: addr,
		timestamp: time.Now(),
	}
}

// updateTXTCache updates the cache entry for the txt.
func (r *Resolver) updateTXTCache(txt string, addr []string) {
	r.Lock()
	defer r.Unlock()

	r.txtCache[txt] = &txtCacheEntry{
		addresses: addr,
		timestamp: time.Now(),
	}
}
