package auth

import (
	"sync"
	"testing"
	"time"
)

func TestBlacklistCache_SetAndContains_NonExpired(t *testing.T) {
	c := NewBlacklistCache()
	c.Set("jti-1", time.Now().Add(time.Minute))

	if !c.Contains("jti-1") {
		t.Error("expected Contains to return true for non-expired entry")
	}
}

func TestBlacklistCache_Contains_Expired(t *testing.T) {
	c := NewBlacklistCache()
	// Set expiry in the past.
	c.Set("jti-expired", time.Now().Add(-time.Second))

	if c.Contains("jti-expired") {
		t.Error("expected Contains to return false for expired entry")
	}
}

func TestBlacklistCache_Contains_Unknown(t *testing.T) {
	c := NewBlacklistCache()

	if c.Contains("jti-unknown") {
		t.Error("expected Contains to return false for unknown jti")
	}
}

func TestBlacklistCache_Evict_RemovesExpiredKeepsValid(t *testing.T) {
	c := NewBlacklistCache()
	c.Set("jti-old", time.Now().Add(-time.Second))
	c.Set("jti-live", time.Now().Add(time.Minute))

	c.Evict()

	if c.Contains("jti-old") {
		t.Error("expected expired entry to be removed after Evict")
	}
	if !c.Contains("jti-live") {
		t.Error("expected non-expired entry to remain after Evict")
	}
}

func TestBlacklistCache_Concurrent_NoRace(t *testing.T) {
	c := NewBlacklistCache()
	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers * 2)

	for i := range workers {
		go func(i int) {
			defer wg.Done()
			jti := "jti-concurrent"
			c.Set(jti, time.Now().Add(time.Duration(i)*time.Millisecond+time.Minute))
		}(i)
		go func() {
			defer wg.Done()
			c.Contains("jti-concurrent")
		}()
	}

	wg.Wait()
}

func TestBlacklistCache_Evict_Concurrent_NoRace(t *testing.T) {
	c := NewBlacklistCache()
	const workers = 10
	var wg sync.WaitGroup
	wg.Add(workers * 2)

	for range workers {
		go func() {
			defer wg.Done()
			c.Set("evict-jti", time.Now().Add(-time.Millisecond))
		}()
		go func() {
			defer wg.Done()
			c.Evict()
		}()
	}

	wg.Wait()
}
