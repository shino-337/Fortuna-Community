package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const risksWSChannel = "risks"

// risksWSConnCountByIP tracks active WS connections per IP for rate limiting (Phase 3).
var risksWSConnCountByIP = struct {
	sync.Mutex
	m map[string]int
}{m: make(map[string]int)}

func getRisksWSMaxConnsPerIP() int {
	if v := os.Getenv("RISKS_WS_MAX_CONNS_PER_IP"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 10
}

type risksWSConn struct {
	send     chan []byte
	conn     *websocket.Conn
	clientIP string
}

// RisksWSHub broadcasts risk-center updates to all connected clients (e.g. when insights are created/updated).
type RisksWSHub struct {
	mu    sync.RWMutex
	conns map[*risksWSConn]struct{}
}

var defaultRisksHub *RisksWSHub

func init() {
	defaultRisksHub = &RisksWSHub{conns: make(map[*risksWSConn]struct{})}
}

// Register adds a connection to the risks channel.
func (h *RisksWSHub) Register(c *risksWSConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[c] = struct{}{}
}

// Unregister removes a connection.
func (h *RisksWSHub) Unregister(c *risksWSConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, c)
}

// Broadcast sends msg to all connected clients. Non-blocking; drops if send buffer full.
func (h *RisksWSHub) Broadcast(msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.conns {
		select {
		case c.send <- msg:
		default:
		}
	}
}

// RisksUpdatePayload is sent over WebSocket when insights change (delta: client can refetch only changed_ids if present).
type RisksUpdatePayload struct {
	Type       string   `json:"type"`                 // "insights_updated"
	ChangedIDs []string `json:"changed_ids,omitempty"` // insight IDs created/updated/deleted
	ChangeType string   `json:"change_type,omitempty"` // "create" | "update" | "delete" (optional)
}

// BroadcastRisksUpdate notifies all Risk Center clients to refetch (e.g. after insights created/updated).
// Call from NATS subscriber in main when "fortuna.insights.updated" is received, or from workers via a shared callback.
func BroadcastRisksUpdate() {
	BroadcastRisksUpdateWithPayload(nil)
}

// BroadcastRisksUpdateWithPayload sends a delta payload to WS clients and invalidates risks cache.
// If payload is nil or ChangedIDs is empty, clients should full refetch; otherwise they may refetch only affected data.
func BroadcastRisksUpdateWithPayload(payload *RisksUpdatePayload) {
	if defaultRisksHub == nil {
		return
	}
	if payload == nil {
		payload = &RisksUpdatePayload{Type: "insights_updated"}
	}
	if payload.Type == "" {
		payload.Type = "insights_updated"
	}
	msg, err := json.Marshal(payload)
	if err != nil {
		return
	}
	// Invalidate list, summary and histogram caches so next GET returns fresh data
	if defaultRisksCache != nil {
		defaultRisksCache.ClearByPrefix("risks:list:")
		defaultRisksCache.ClearByPrefix("insights:summary:")
		defaultRisksCache.ClearByPrefix("risk:histogram:")
	}
	defaultRisksHub.Broadcast(msg)
}

// RisksWS handles GET /api/v1/ws/risks — WebSocket for Risk Center live updates.
func RisksWS() gin.HandlerFunc {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
	return func(c *gin.Context) {
		upgrade := c.GetHeader("Upgrade")
		if !strings.EqualFold(strings.TrimSpace(upgrade), "websocket") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "WebSocket upgrade required"})
			return
		}
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("[RisksWS] upgrade error: %v", err)
			return
		}
		clientIP := c.ClientIP()
		maxPerIP := getRisksWSMaxConnsPerIP()
		risksWSConnCountByIP.Lock()
		if risksWSConnCountByIP.m[clientIP] >= maxPerIP {
			risksWSConnCountByIP.Unlock()
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "too many connections"))
			conn.Close()
			return
		}
		risksWSConnCountByIP.m[clientIP]++
		risksWSConnCountByIP.Unlock()
		wc := &risksWSConn{send: make(chan []byte, 8), conn: conn, clientIP: clientIP}
		defaultRisksHub.Register(wc)
		defer func() {
			defaultRisksHub.Unregister(wc)
			risksWSConnCountByIP.Lock()
			risksWSConnCountByIP.m[wc.clientIP]--
			if risksWSConnCountByIP.m[wc.clientIP] <= 0 {
				delete(risksWSConnCountByIP.m, wc.clientIP)
			}
			risksWSConnCountByIP.Unlock()
			close(wc.send)
			conn.Close()
		}()

		go func() {
			for msg := range wc.send {
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			}
		}()

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}
}
