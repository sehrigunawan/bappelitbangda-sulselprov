package auth

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A21: Hasher never runs more than 2 hashes concurrently.
// 5 goroutines Hash; instrumented onHash records max in-flight; max == 2.
func TestHasherConcurrencyLimit(t *testing.T) {
	t.Parallel()

	h := NewHasher()
	var inFlight atomic.Int64
	var maxInFlight atomic.Int64

	h.onHash = func() {
		cur := inFlight.Add(1)
		for {
			prev := maxInFlight.Load()
			if cur <= prev || maxInFlight.CompareAndSwap(prev, cur) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		inFlight.Add(-1)
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = h.Hash(ctx, "rahasia-panjang-01")
		}()
	}
	wg.Wait()

	if got := maxInFlight.Load(); got != 2 {
		t.Fatalf("max concurrent hashes = %d, want 2", got)
	}
}
