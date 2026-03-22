package platformservices

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/movebigrocks/platform/pkg/logger"
)

// formNonce represents a single-use token for form submissions
type formNonce struct {
	createdAt time.Time
	used      bool
}

// NonceService manages single-use form nonces to prevent duplicate submissions
// and CSRF attacks. It handles generation, validation, and automatic cleanup
// of expired nonces.
type NonceService struct {
	nonces  map[string]*formNonce
	mu      sync.RWMutex
	ttl     time.Duration
	cleanup chan struct{}
	logger  *logger.Logger
}

// NonceServiceOption is a functional option for configuring NonceService
type NonceServiceOption func(*NonceService)

// WithNonceLogger sets the logger for the service
func WithNonceLogger(log *logger.Logger) NonceServiceOption {
	return func(ns *NonceService) {
		ns.logger = log
	}
}

// NewNonceService creates a new nonce service with the given options.
// The service starts a background goroutine for cleanup - call Stop() to clean up.
func NewNonceService(opts ...NonceServiceOption) *NonceService {
	ns := &NonceService{
		nonces:  make(map[string]*formNonce),
		ttl:     15 * time.Minute, // Default TTL
		cleanup: make(chan struct{}),
		logger:  logger.NewNop(),
	}

	for _, opt := range opts {
		opt(ns)
	}

	// Start background cleanup
	go ns.cleanupLoop()

	return ns
}

// Generate creates a new single-use form nonce
func (ns *NonceService) Generate() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	nonce := hex.EncodeToString(bytes)

	ns.mu.Lock()
	ns.nonces[nonce] = &formNonce{
		createdAt: time.Now(),
		used:      false,
	}
	ns.mu.Unlock()

	return nonce, nil
}

// ValidateAndConsume checks if a nonce is valid and marks it as used.
// Returns true if the nonce was valid and has been consumed.
// Returns false if the nonce is empty, doesn't exist, already used, or expired.
func (ns *NonceService) ValidateAndConsume(nonce string) bool {
	if nonce == "" {
		return false
	}

	ns.mu.Lock()
	defer ns.mu.Unlock()

	data, exists := ns.nonces[nonce]
	if !exists {
		return false
	}

	// Check if already used
	if data.used {
		return false
	}

	// Check if expired
	if time.Since(data.createdAt) > ns.ttl {
		delete(ns.nonces, nonce)
		return false
	}

	// Mark as used (don't delete yet, cleanup will handle it)
	data.used = true
	return true
}

// cleanupLoop periodically removes expired and used nonces
func (ns *NonceService) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ns.cleanup:
			return
		case <-ticker.C:
			ns.cleanupExpired()
		}
	}
}

// cleanupExpired removes expired and used nonces from the map
func (ns *NonceService) cleanupExpired() {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	now := time.Now()
	for nonce, data := range ns.nonces {
		// Remove nonces that are expired or already used
		if data.used || now.Sub(data.createdAt) > ns.ttl {
			delete(ns.nonces, nonce)
		}
	}
}
