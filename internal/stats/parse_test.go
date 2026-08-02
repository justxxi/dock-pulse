package stats

import (
	"testing"
	"time"
)

func TestParseRawStats(t *testing.T) {
	t.Parallel()

	sampleJSON := []byte(`{
		"read": "2026-08-03T01:00:02Z",
		"cpu_stats": {
			"cpu_usage": { "total_usage": 1000000000 },
			"system_cpu_usage": 20000000000,
			"online_cpus": 2
		},
		"precpu_stats": {
			"cpu_usage": { "total_usage": 500000000 },
			"system_cpu_usage": 19000000000
		},
		"memory_stats": {
			"usage": 104857600,
			"limit": 1073741824,
			"stats": { "inactive_file": 4194304 }
		},
		"networks": {
			"eth0": { "rx_bytes": 2048, "tx_bytes": 4096 }
		},
		"blkio_stats": {
			"io_service_bytes_recursive": [
				{ "op": "Read", "value": 8192 },
				{ "op": "Write", "value": 16384 }
			]
		}
	}`)

	prevState := ParseState{
		PrevTime:       time.Date(2026, 8, 3, 1, 0, 0, 0, time.UTC),
		PrevNetRx:      1024,
		PrevNetTx:      2048,
		PrevBlockRead:  4096,
		PrevBlockWrite: 8192,
		HasPrev:        true,
	}

	pt, newState, err := ParseRawStats(sampleJSON, &prevState)
	if err != nil {
		t.Fatalf("ParseRawStats failed: %v", err)
	}

	if pt.CPUPercent != 100.0 {
		t.Errorf("expected CPU 100%%, got %f", pt.CPUPercent)
	}

	expectedMem := uint64(104857600 - 4194304)
	if pt.MemoryBytes != expectedMem {
		t.Errorf("expected MemoryBytes %d, got %d", expectedMem, pt.MemoryBytes)
	}

	if pt.NetRxBytes != 512 {
		t.Errorf("expected NetRxBytes 512, got %d", pt.NetRxBytes)
	}

	if pt.BlockRead != 2048 {
		t.Errorf("expected BlockRead 2048, got %d", pt.BlockRead)
	}

	if !newState.HasPrev {
		t.Errorf("expected state to have prev")
	}
}

func TestCalculateCPUPercentZeroDelta(t *testing.T) {
	t.Parallel()

	cpu := rawCPUStats{
		CPUUsage:       rawCPUUsage{TotalUsage: 100},
		SystemCPUUsage: 1000,
		OnlineCPUs:     2,
	}
	precpu := rawCPUStats{
		CPUUsage:       rawCPUUsage{TotalUsage: 100},
		SystemCPUUsage: 1000,
	}

	pct := CalculateCPUPercent(cpu, precpu)
	if pct != 0.0 {
		t.Errorf("expected 0.0 for zero delta, got %f", pct)
	}
}

func BenchmarkParseRawStats(b *testing.B) {
	sampleJSON := []byte(`{
		"read": "2026-08-03T01:00:02Z",
		"cpu_stats": {
			"cpu_usage": { "total_usage": 1000000000 },
			"system_cpu_usage": 20000000000,
			"online_cpus": 2
		},
		"precpu_stats": {
			"cpu_usage": { "total_usage": 500000000 },
			"system_cpu_usage": 19000000000
		},
		"memory_stats": {
			"usage": 104857600,
			"limit": 1073741824,
			"stats": { "inactive_file": 4194304 }
		}
	}`)

	state := ParseState{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = ParseRawStats(sampleJSON, &state)
	}
}
