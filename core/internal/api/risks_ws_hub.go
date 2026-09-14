package api

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/fortuna/core/pkg/authorization"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
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

// RisksUpdatePayload accepts producer delta metadata. Browser notifications omit IDs until producers carry authoritative cluster ownership.
type RisksUpdatePayload struct {
	Type       string   `json:"type"`                  // "insights_updated"
	ChangedIDs []string `json:"changed_ids,omitempty"` // insight IDs created/updated/deleted
	ChangeType string   `json:"change_type,omitempty"` // "create" | "update" | "delete" (optional)
}

// BroadcastRisksUpdate notifies all Risk Center clients to refetch (e.g. after insights created/updated).
// Call from NATS subscriber in main when "fortuna.insights.updated" is received, or from workers via a shared callback.
func BroadcastRisksUpdate() {
	BroadcastRisksUpdateWithPayload(nil)
}

// BroadcastRisksUpdateWithPayload invalidates caches and sends a generic refetch notification.
// ChangedIDs and ChangeType are intentionally not sent on the global channel.
func BroadcastRisksUpdateWithPayload(payload *RisksUpdatePayload) {
	if defaultRisksHub == nil {
		return
	}
	// Producers do not carry authoritative cluster ownership. A generic
	// invalidation makes each client refetch through scoped HTTP authorization.
	msg := []byte(`{"type":"insights_updated"}`)
	// Invalidate list, summary and histogram caches so next GET returns fresh data
	if defaultRisksCache != nil {
		defaultRisksCache.ClearByPrefix("risks:list:")
		defaultRisksCache.ClearByPrefix("insights:summary:")
		defaultRisksCache.ClearByPrefix("risk:histogram:")
	}
	defaultRisksHub.Broadcast(msg)
}

// RisksWS handles GET /api/v1/ws/risks — WebSocket for Risk Center live updates.
func RisksWS(db *gorm.DB) gin.HandlerFunc {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     wsAllowedOrigin,
	}
	return func(c *gin.Context) {
		upgrade := c.GetHeader("Upgrade")
		if !strings.EqualFold(strings.TrimSpace(upgrade), "websocket") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "WebSocket upgrade required"})
			return
		}
		guard, err := newWSAuthorization(db, c, authorization.PermissionFindingsRead, "")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "websocket authorization required"})
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

		serveAuthorizedWebSocket(conn, wc.send, guard, wsAuthorizationInterval)
	}
}
