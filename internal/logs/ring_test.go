package logs

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRingBufferOverflowAndSequence(t *testing.T) {
	t.Parallel()

	ring := NewRingBuffer(5)

	for i := 1; i <= 10; i++ {
		line := ring.Push(time.Now(), "stdout", fmt.Sprintf("line %d", i))
		if line.Seq != uint64(i) {
			t.Errorf("expected seq %d, got %d", i, line.Seq)
		}
	}

	if ring.Count() != 5 {
		t.Fatalf("expected count 5 after overflow, got %d", ring.Count())
	}

	tail := ring.ReadTail(5)
	if len(tail) != 5 {
		t.Fatalf("expected tail of 5 lines, got %d", len(tail))
	}

	if tail[0].Text != "line 6" {
		t.Errorf("expected first line in tail to be line 6, got %s", tail[0].Text)
	}
	if tail[4].Text != "line 10" {
		t.Errorf("expected last line in tail to be line 10, got %s", tail[4].Text)
	}
}

func TestRingBufferReadSinceSeq(t *testing.T) {
	t.Parallel()

	ring := NewRingBuffer(10)
	for i := 1; i <= 5; i++ {
		ring.Push(time.Now(), "stdout", fmt.Sprintf("log %d", i))
	}

	lines := ring.ReadSinceSeq(3)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines since seq 3, got %d", len(lines))
	}
	if lines[0].Seq != 4 || lines[1].Seq != 5 {
		t.Errorf("expected seqs [4, 5], got [%d, %d]", lines[0].Seq, lines[1].Seq)
	}
}

func TestRingBufferConcurrency(t *testing.T) {
	t.Parallel()

	ring := NewRingBuffer(100)
	var wg sync.WaitGroup

	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				ring.Push(time.Now(), "stdout", fmt.Sprintf("goroutine %d log %d", id, i))
				_ = ring.ReadTail(10)
			}
		}(g)
	}

	wg.Wait()
	if ring.Count() != 100 {
		t.Errorf("expected count 100, got %d", ring.Count())
	}
}

func BenchmarkRingBufferPush(b *testing.B) {
	ring := NewRingBuffer(1000)
	now := time.Now()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = ring.Push(now, "stdout", "sample static log message payload line")
	}
}
