package testutil

import (
	"context"
	"sync"
	"time"
)

// StubBlacklister is an in-memory Blacklister for unit tests.
// It eliminates the need for miniredis in auth app handler tests.
type StubBlacklister struct {
	mu               sync.Mutex
	tokens           map[string]time.Time // jti → expiry
	BlacklistErr     error                // injected error for Blacklist calls
	IsBlacklistedErr error                // injected error for IsBlacklisted calls
}

// NewStubBlacklister returns a ready-to-use StubBlacklister.
func NewStubBlacklister() *StubBlacklister {
	return &StubBlacklister{tokens: make(map[string]time.Time)}
}

// Blacklist records the jti with its expiry time.
func (s *StubBlacklister) Blacklist(_ context.Context, jti string, expiry time.Time) error {
	if s.BlacklistErr != nil {
		return s.BlacklistErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tokens == nil {
		s.tokens = make(map[string]time.Time)
	}
	s.tokens[jti] = expiry
	return nil
}

// IsBlacklisted returns true if jti was blacklisted and has not expired yet.
func (s *StubBlacklister) IsBlacklisted(_ context.Context, jti string) (bool, error) {
	if s.IsBlacklistedErr != nil {
		return false, s.IsBlacklistedErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.tokens[jti]
	if !ok {
		return false, nil
	}
	return time.Now().Before(exp), nil
}
