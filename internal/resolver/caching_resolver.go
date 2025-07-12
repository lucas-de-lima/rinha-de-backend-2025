package resolver

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"sync"
	"time"
)

// CachingKeyResolver implementa um cache para chaves públicas.
type CachingKeyResolver struct {
	mu         sync.RWMutex
	keySource  func(kid string) (ed25519.PublicKey, error)
	cache      map[string]ed25519.PublicKey
	cacheTTL   time.Duration
	ttlManager map[string]time.Time
}

// NewCachingKeyResolver cria um novo resolver com cache.
func NewCachingKeyResolver(
	keySource func(kid string) (ed25519.PublicKey, error),
	cacheTTL time.Duration,
) *CachingKeyResolver {
	return &CachingKeyResolver{
		keySource:  keySource,
		cache:      make(map[string]ed25519.PublicKey),
		cacheTTL:   cacheTTL,
		ttlManager: make(map[string]time.Time),
	}
}

// ResolverFunc é a implementação do signet.KeyResolverFunc.
func (r *CachingKeyResolver) ResolverFunc(ctx context.Context, kid string) (ed25519.PublicKey, error) {
	r.mu.RLock()
	key, found := r.cache[kid]
	ttl, ttlFound := r.ttlManager[kid]
	r.mu.RUnlock()

	if found && ttlFound && time.Now().Before(ttl) {
		return key, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	key, found = r.cache[kid]
	ttl, ttlFound = r.ttlManager[kid]
	if found && ttlFound && time.Now().Before(ttl) {
		return key, nil
	}

	newKey, err := r.keySource(kid)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar chave da fonte para kid %s: %w", kid, err)
	}

	r.cache[kid] = newKey
	r.ttlManager[kid] = time.Now().Add(r.cacheTTL)
	return newKey, nil
}
