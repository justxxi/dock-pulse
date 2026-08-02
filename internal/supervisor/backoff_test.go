package supervisor

import (
	"testing"
	"time"
)

type FakeClock struct {
	now time.Time
}

func (f *FakeClock) Now() time.Time {
	return f.now
}

func (f *FakeClock) Advance(d time.Duration) {
	f.now = f.now.Add(d)
}

type ConstantRandom struct {
	val float64
}

func (c ConstantRandom) Float64() float64 {
	return c.val
}

func TestBackoffExponentialAndJitter(t *testing.T) {
	t.Parallel()

	clock := &FakeClock{now: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)}
	rand := ConstantRandom{val: 0.5}

	cfg := BackoffConfig{
		BaseInterval:       2 * time.Second,
		MaxInterval:        30 * time.Second,
		MaxAttempts:        3,
		StabilityThreshold: 10 * time.Second,
	}

	bm := NewBackoffManager(cfg, clock, rand)

	delay1, attempt1, ok1 := bm.NextDelay("c1")
	if !ok1 || attempt1 != 1 || delay1 != 1*time.Second {
		t.Errorf("attempt 1: got delay %v, attempt %d, ok %v; want 1s, 1, true", delay1, attempt1, ok1)
	}

	clock.Advance(1 * time.Second)

	delay2, attempt2, ok2 := bm.NextDelay("c1")
	if !ok2 || attempt2 != 2 || delay2 != 2*time.Second {
		t.Errorf("attempt 2: got delay %v, attempt %d, ok %v; want 2s, 2, true", delay2, attempt2, ok2)
	}

	clock.Advance(1 * time.Second)

	delay3, attempt3, ok3 := bm.NextDelay("c1")
	if !ok3 || attempt3 != 3 || delay3 != 4*time.Second {
		t.Errorf("attempt 3: got delay %v, attempt %d, ok %v; want 4s, 3, true", delay3, attempt3, ok3)
	}

	clock.Advance(1 * time.Second)

	_, attempt4, ok4 := bm.NextDelay("c1")
	if ok4 || attempt4 != 4 {
		t.Errorf("attempt 4: expected ok=false when max attempts exceeded, got ok=%v attempt=%d", ok4, attempt4)
	}
}

func TestBackoffStabilityReset(t *testing.T) {
	t.Parallel()

	clock := &FakeClock{now: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)}
	rand := ConstantRandom{val: 1.0}

	cfg := BackoffConfig{
		BaseInterval:       1 * time.Second,
		MaxInterval:        10 * time.Second,
		MaxAttempts:        2,
		StabilityThreshold: 15 * time.Second,
	}

	bm := NewBackoffManager(cfg, clock, rand)

	_, attempt1, ok1 := bm.NextDelay("c1")
	if !ok1 || attempt1 != 1 {
		t.Fatalf("expected attempt 1 success")
	}

	clock.Advance(20 * time.Second)

	_, attempt2, ok2 := bm.NextDelay("c1")
	if !ok2 || attempt2 != 1 {
		t.Errorf("expected attempt counter reset to 1 after stability duration, got attempt=%d ok=%v", attempt2, ok2)
	}
}

func TestBackoffIntentionalStop(t *testing.T) {
	t.Parallel()

	clock := &FakeClock{now: time.Now()}
	bm := NewBackoffManager(DefaultBackoffConfig(), clock, ConstantRandom{val: 1.0})

	bm.MarkIntentionalStop("c1")

	_, _, ok := bm.NextDelay("c1")
	if ok {
		t.Errorf("expected ok=false for intentionally stopped container")
	}

	bm.ClearIntentionalStop("c1")

	_, attempt, ok := bm.NextDelay("c1")
	if !ok || attempt != 1 {
		t.Errorf("expected attempt 1 success after clearing intentional stop")
	}
}
