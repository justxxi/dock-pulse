package protocol

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/owner/dock-pulse/internal/dockerx"
)

const ProtocolVersion = "1.0"

const (
	TypeSnapshot         = "snapshot"
	TypeContainerUpdated = "container.updated"
	TypeContainerRemoved = "container.removed"
	TypeStats            = "stats"
	TypeLog              = "log"
	TypeSupervisor       = "supervisor"
	TypeError            = "error"
	TypePong             = "pong"

	TypeSubscribeLogs   = "subscribe.logs"
	TypeUnsubscribeLogs = "unsubscribe.logs"
	TypePing            = "ping"
)

type Envelope struct {
	Type string          `json:"type"`
	Seq  uint64          `json:"seq,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

type SnapshotData struct {
	Version    string              `json:"version"`
	Containers []dockerx.Container `json:"containers"`
	Seq        uint64              `json:"seq"`
}

type ContainerUpdatedData struct {
	Container dockerx.Container `json:"container"`
}

type ContainerRemovedData struct {
	ID string `json:"id"`
}

type StatsPoint struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryBytes   uint64  `json:"memory_bytes"`
	MemoryLimit   uint64  `json:"memory_limit"`
	MemoryPercent float64 `json:"memory_percent"`
	NetRxBytes    uint64  `json:"net_rx_bytes"`
	NetTxBytes    uint64  `json:"net_tx_bytes"`
	BlockRead     uint64  `json:"block_read"`
	BlockWrite    uint64  `json:"block_write"`
	Timestamp     int64   `json:"timestamp"`
}

type ContainerStatsData struct {
	ID    string     `json:"id"`
	Stats StatsPoint `json:"stats"`
}

type LogLineData struct {
	ContainerID string    `json:"container_id"`
	Seq         uint64    `json:"seq"`
	Timestamp   time.Time `json:"timestamp"`
	Stream      string    `json:"stream"`
	Text        string    `json:"text"`
}

type SupervisorEventData struct {
	ContainerID string    `json:"container_id"`
	Action      string    `json:"action"`
	Attempt     int       `json:"attempt"`
	NextRetry   time.Time `json:"next_retry,omitempty"`
	Reason      string    `json:"reason"`
	Exhausted   bool      `json:"exhausted"`
}

type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type SubscribeLogsData struct {
	ContainerID string `json:"container_id"`
	Tail        int    `json:"tail,omitempty"`
	FromSeq     uint64 `json:"from_seq,omitempty"`
}

type UnsubscribeLogsData struct {
	ContainerID string `json:"container_id"`
}

func EncodeEnvelope(msgType string, seq uint64, payload interface{}) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload for %s: %w", msgType, err)
	}
	env := Envelope{
		Type: msgType,
		Seq:  seq,
		Data: data,
	}
	return json.Marshal(env)
}

func DecodeEnvelope(raw []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return Envelope{}, fmt.Errorf("unmarshal envelope: %w", err)
	}
	return env, nil
}
