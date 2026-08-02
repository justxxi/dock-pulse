package stats

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/owner/dock-pulse/internal/protocol"
)

type rawCPUUsage struct {
	TotalUsage  uint64   `json:"total_usage"`
	PercpuUsage []uint64 `json:"percpu_usage"`
}

type rawCPUStats struct {
	CPUUsage       rawCPUUsage `json:"cpu_usage"`
	SystemCPUUsage uint64      `json:"system_cpu_usage"`
	OnlineCPUs     uint32      `json:"online_cpus"`
}

type rawMemStats struct {
	Usage uint64 `json:"usage"`
	Limit uint64 `json:"limit"`
	Stats struct {
		InactiveFile  uint64 `json:"inactive_file"`
		TotalInactive uint64 `json:"total_inactive_file"`
	} `json:"stats"`
}

type rawNetStats struct {
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
}

type rawBlkioStat struct {
	Op    string `json:"op"`
	Value uint64 `json:"value"`
}

type rawBlkio struct {
	IOServiceBytesRecursive []rawBlkioStat `json:"io_service_bytes_recursive"`
}

type rawDockerStats struct {
	Read        time.Time              `json:"read"`
	Preread     time.Time              `json:"preread"`
	CPUStats    rawCPUStats            `json:"cpu_stats"`
	PreCPUStats rawCPUStats            `json:"precpu_stats"`
	MemoryStats rawMemStats            `json:"memory_stats"`
	Networks    map[string]rawNetStats `json:"networks"`
	BlkioStats  rawBlkio               `json:"blkio_stats"`
}

type ParseState struct {
	PrevTime       time.Time
	PrevNetRx      uint64
	PrevNetTx      uint64
	PrevBlockRead  uint64
	PrevBlockWrite uint64
	HasPrev        bool
}

func ParseRawStats(data []byte, state *ParseState) (protocol.StatsPoint, ParseState, error) {
	var raw rawDockerStats
	if err := json.Unmarshal(data, &raw); err != nil {
		return protocol.StatsPoint{}, ParseState{}, err
	}

	timestamp := raw.Read.UnixMilli()
	if timestamp == 0 {
		timestamp = time.Now().UnixMilli()
	}

	cpuPercent := CalculateCPUPercent(raw.CPUStats, raw.PreCPUStats)
	memBytes, memLimit, memPercent := CalculateMemory(raw.MemoryStats)
	netRx, netTx := CalculateNetTotal(raw.Networks)
	blkRead, blkWrite := CalculateBlkioTotal(raw.BlkioStats)

	var netRxRate, netTxRate, blkReadRate, blkWriteRate uint64

	newState := ParseState{
		PrevTime:       raw.Read,
		PrevNetRx:      netRx,
		PrevNetTx:      netTx,
		PrevBlockRead:  blkRead,
		PrevBlockWrite: blkWrite,
		HasPrev:        true,
	}

	if state != nil && state.HasPrev {
		duration := raw.Read.Sub(state.PrevTime).Seconds()
		if duration > 0 {
			if netRx >= state.PrevNetRx {
				netRxRate = uint64(float64(netRx-state.PrevNetRx) / duration)
			}
			if netTx >= state.PrevNetTx {
				netTxRate = uint64(float64(netTx-state.PrevNetTx) / duration)
			}
			if blkRead >= state.PrevBlockRead {
				blkReadRate = uint64(float64(blkRead-state.PrevBlockRead) / duration)
			}
			if blkWrite >= state.PrevBlockWrite {
				blkWriteRate = uint64(float64(blkWrite-state.PrevBlockWrite) / duration)
			}
		}
	}

	pt := protocol.StatsPoint{
		CPUPercent:    math.Round(cpuPercent*100) / 100,
		MemoryBytes:   memBytes,
		MemoryLimit:   memLimit,
		MemoryPercent: math.Round(memPercent*100) / 100,
		NetRxBytes:    netRxRate,
		NetTxBytes:    netTxRate,
		BlockRead:     blkReadRate,
		BlockWrite:    blkWriteRate,
		Timestamp:     timestamp,
	}

	return pt, newState, nil
}

func CalculateCPUPercent(cpu rawCPUStats, precpu rawCPUStats) float64 {
	cpuDelta := float64(cpu.CPUUsage.TotalUsage) - float64(precpu.CPUUsage.TotalUsage)
	systemDelta := float64(cpu.SystemCPUUsage) - float64(precpu.SystemCPUUsage)

	if systemDelta <= 0 || cpuDelta < 0 {
		return 0.0
	}

	cpus := float64(cpu.OnlineCPUs)
	if cpus == 0 {
		cpus = float64(len(cpu.CPUUsage.PercpuUsage))
	}
	if cpus == 0 {
		cpus = 1.0
	}

	return (cpuDelta / systemDelta) * cpus * 100.0
}

func CalculateMemory(mem rawMemStats) (uint64, uint64, float64) {
	inactive := mem.Stats.InactiveFile
	if inactive == 0 {
		inactive = mem.Stats.TotalInactive
	}

	var usage uint64
	if mem.Usage > inactive {
		usage = mem.Usage - inactive
	} else {
		usage = mem.Usage
	}

	limit := mem.Limit
	var percent float64
	if limit > 0 {
		percent = (float64(usage) / float64(limit)) * 100.0
	}

	return usage, limit, percent
}

func CalculateNetTotal(nets map[string]rawNetStats) (uint64, uint64) {
	var rx, tx uint64
	for _, n := range nets {
		rx += n.RxBytes
		tx += n.TxBytes
	}
	return rx, tx
}

func CalculateBlkioTotal(blk rawBlkio) (uint64, uint64) {
	var read, write uint64
	for _, entry := range blk.IOServiceBytesRecursive {
		switch strings.ToLower(entry.Op) {
		case "read":
			read += entry.Value
		case "write":
			write += entry.Value
		}
	}
	return read, write
}
