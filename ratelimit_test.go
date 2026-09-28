package bunker

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestRateLimiterWindow(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	limiter := newRateLimiter(2, time.Minute)
	limiter.now = func() time.Time { return now }

	if ok, _ := limiter.Allowed("a"); !ok {
		t.Fatal("first attempt should be allowed")
	}
	limiter.Fail("a")
	limiter.Fail("a")
	if ok, wait := limiter.Allowed("a"); ok || wait <= 0 {
		t.Fatalf("limiter allowed=%v wait=%s", ok, wait)
	}
	limiter.Reset("a")
	if ok, _ := limiter.Allowed("a"); !ok {
		t.Fatal("reset should allow the key again")
	}

	limiter.Fail("a")
	limiter.Fail("a")
	now = now.Add(time.Minute + time.Second)
	if ok, _ := limiter.Allowed("a"); !ok {
		t.Fatal("window expiry should allow the key again")
	}
	if _, exists := limiter.entries["a"]; exists {
		t.Fatal("expired entry should be dropped")
	}
}

func TestRateLimiterCountsExactlyToTheLimit(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	limiter := newRateLimiter(2, time.Minute)
	limiter.now = func() time.Time { return now }

	limiter.Fail("a")
	if ok, _ := limiter.Allowed("a"); !ok {
		t.Fatal("one failure is still inside the limit")
	}
	limiter.Fail("a")
	if ok, wait := limiter.Allowed("a"); ok || wait != time.Minute {
		t.Fatalf("at limit allowed=%v wait=%s", ok, wait)
	}

	now = now.Add(time.Minute + time.Second)
	limiter.Fail("a")
	if ok, _ := limiter.Allowed("a"); !ok {
		t.Fatal("a failure after the window should start a fresh count")
	}
	if limiter.entries["a"].count != 1 {
		t.Fatalf("count = %d", limiter.entries["a"].count)
	}
}

func TestRateLimiterKeysAreIndependent(t *testing.T) {
	limiter := newRateLimiter(1, time.Minute)
	limiter.Fail("a")
	if ok, _ := limiter.Allowed("a"); ok {
		t.Fatal("a should be limited")
	}
	if ok, _ := limiter.Allowed("b"); !ok {
		t.Fatal("b should still be allowed")
	}
}

func TestRateLimiterPrunesExpiredEntries(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	limiter := newRateLimiter(1, time.Minute)
	limiter.now = func() time.Time { return now }
	for i := 0; i < 4096; i++ {
		limiter.entries[strconv.Itoa(i)] = &rateLimitEntry{count: 1, reset: now.Add(-time.Second)}
	}
	if ok, _ := limiter.Allowed("fresh"); !ok {
		t.Fatal("fresh key should be allowed")
	}
	if len(limiter.entries) != 0 {
		t.Fatalf("pruned map len = %d", len(limiter.entries))
	}
}

func TestRateLimiterConcurrent(t *testing.T) {
	limiter := newRateLimiter(1000, time.Minute)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 100; n++ {
				limiter.Allowed("k")
				limiter.Fail("k")
				if n%10 == 0 {
					limiter.Reset("k")
				}
			}
		}()
	}
	wg.Wait()
}
