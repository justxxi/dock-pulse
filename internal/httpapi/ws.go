package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/owner/dock-pulse/internal/auth"
	"github.com/owner/dock-pulse/internal/hub"
	"github.com/owner/dock-pulse/internal/logs"
	"github.com/owner/dock-pulse/internal/protocol"
	"github.com/owner/dock-pulse/internal/registry"
	"github.com/owner/dock-pulse/internal/version"
)

type WSHandler struct {
	hub           *hub.Hub
	registry      *registry.Registry
	streamer      *logs.Streamer
	authenticator *auth.Authenticator
	logger        *slog.Logger
}

func NewWSHandler(h *hub.Hub, reg *registry.Registry, streamer *logs.Streamer, auth *auth.Authenticator, logger *slog.Logger) *WSHandler {
	return &WSHandler{
		hub:           h,
		registry:      reg,
		streamer:      streamer,
		authenticator: auth,
		logger:        logger,
	}
}

func (ws *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if ws.authenticator.IsRequired() && !ws.authenticator.AuthenticateRequest(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: false,
	})
	if err != nil {
		ws.logger.Error("websocket accept failed", "error", err)
		return
	}

	b := make([]byte, 8)
	_, _ = rand.Read(b)
	clientID := hex.EncodeToString(b)

	ctx, cancel := context.WithCancel(r.Context())
	client := hub.NewClient(clientID, conn, ws.hub, 256, cancel)

	if err := ws.hub.Register(client); err != nil {
		ws.logger.Warn("websocket client rejected", "reason", err.Error())
		conn.Close(websocket.StatusPolicyViolation, err.Error())
		return
	}

	defer func() {
		ws.hub.Unregister(client)
		conn.Close(websocket.StatusNormalClosure, "connection closed")
	}()

	containers, seq := ws.registry.List()
	snapshotPayload := protocol.SnapshotData{
		Version:    version.Version,
		Containers: containers,
		Seq:        seq,
	}
	snapshotBytes, err := protocol.EncodeEnvelope(protocol.TypeSnapshot, seq, snapshotPayload)
	if err == nil {
		_ = conn.Write(ctx, websocket.MessageText, snapshotBytes)
	}

	go ws.writeLoop(ctx, conn, client)

	ws.readLoop(ctx, conn, client)
}

func (ws *WSHandler) readLoop(ctx context.Context, conn *websocket.Conn, client *hub.Client) {
	conn.SetReadLimit(64 * 1024)

	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return
		}

		env, err := protocol.DecodeEnvelope(raw)
		if err != nil {
			ws.logger.Debug("invalid websocket envelope", "error", err)
			continue
		}

		switch env.Type {
		case protocol.TypePing:
			pongBytes, _ := protocol.EncodeEnvelope(protocol.TypePong, env.Seq, map[string]int64{"time": time.Now().UnixMilli()})
			_ = conn.Write(ctx, websocket.MessageText, pongBytes)

		case protocol.TypeSubscribeLogs:
			var sub protocol.SubscribeLogsData
			if err := json.Unmarshal(env.Data, &sub); err == nil && sub.ContainerID != "" {
				ws.hub.SubscribeLogs(client, sub.ContainerID)
				ws.streamer.Subscribe(ctx, sub.ContainerID)

				ring := ws.streamer.GetRingBuffer(sub.ContainerID)
				var lines []logs.LogLine
				if sub.FromSeq > 0 {
					lines = ring.ReadSinceSeq(sub.FromSeq)
				} else {
					tail := sub.Tail
					if tail <= 0 {
						tail = 200
					}
					lines = ring.ReadTail(tail)
				}

				for _, l := range lines {
					lineData := protocol.LogLineData{
						ContainerID: sub.ContainerID,
						Seq:         l.Seq,
						Timestamp:   l.Timestamp,
						Stream:      l.Stream,
						Text:        l.Text,
					}
					data, err := protocol.EncodeEnvelope(protocol.TypeLog, l.Seq, lineData)
					if err == nil {
						_ = conn.Write(ctx, websocket.MessageText, data)
					}
				}
			}

		case protocol.TypeUnsubscribeLogs:
			var unsub protocol.UnsubscribeLogsData
			if err := json.Unmarshal(env.Data, &unsub); err == nil && unsub.ContainerID != "" {
				ws.hub.UnsubscribeLogs(client, unsub.ContainerID)
				ws.streamer.Unsubscribe(unsub.ContainerID)
			}
		}
	}
}

func (ws *WSHandler) writeLoop(ctx context.Context, conn *websocket.Conn, client *hub.Client) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-client.Send:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Write(writeCtx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		case <-ticker.C:
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			pingBytes, _ := protocol.EncodeEnvelope(protocol.TypePing, 0, map[string]int64{"time": time.Now().UnixMilli()})
			err := conn.Write(writeCtx, websocket.MessageText, pingBytes)
			cancel()
			if err != nil {
				return
			}
		}
	}
}
