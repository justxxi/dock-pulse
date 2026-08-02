package logs

import (
	"sync"
	"sync/atomic"
	"time"
)

type LogLine struct {
	Seq       uint64    `json:"seq"`
	Timestamp time.Time `json:"timestamp"`
	Stream    string    `json:"stream"`
	Text      string    `json:"text"`
}

type RingBuffer struct {
	mu       sync.RWMutex
	lines    []LogLine
	head     int
	count    int
	capacity int
	seq      atomic.Uint64
}

func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1000
	}
	return &RingBuffer{
		lines:    make([]LogLine, capacity),
		capacity: capacity,
	}
}

func (r *RingBuffer) Push(timestamp time.Time, stream, text string) LogLine {
	r.mu.Lock()
	defer r.mu.Unlock()

	seq := r.seq.Add(1)
	line := LogLine{
		Seq:       seq,
		Timestamp: timestamp,
		Stream:    stream,
		Text:      text,
	}

	r.lines[r.head] = line
	r.head = (r.head + 1) % r.capacity
	if r.count < r.capacity {
		r.count++
	}

	return line
}

func (r *RingBuffer) ReadTail(n int) []LogLine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if n <= 0 || r.count == 0 {
		return nil
	}
	if n > r.count {
		n = r.count
	}

	result := make([]LogLine, n)
	startIndex := (r.head - n + r.capacity) % r.capacity

	for i := 0; i < n; i++ {
		idx := (startIndex + i) % r.capacity
		result[i] = r.lines[idx]
	}

	return result
}

func (r *RingBuffer) ReadSinceSeq(fromSeq uint64) []LogLine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.count == 0 {
		return nil
	}

	startIndex := (r.head - r.count + r.capacity) % r.capacity
	result := make([]LogLine, 0, r.count)

	for i := 0; i < r.count; i++ {
		idx := (startIndex + i) % r.capacity
		line := r.lines[idx]
		if line.Seq > fromSeq {
			result = append(result, line)
		}
	}

	return result
}

func (r *RingBuffer) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.count
}

func (r *RingBuffer) LastSeq() uint64 {
	return r.seq.Load()
}
