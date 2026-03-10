package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// podDetailWSConn wraps a WebSocket connection with a per-connection send channel.
type podDetailWSConn struct {
	send chan []byte
	conn *websocket.Conn
}

// PodDetailWSHub allows broadcasting pod detail updates to subscribed clients (Phase 5.1).
type PodDetailWSHub struct {
	mu    sync.RWMutex
	conns map[string]map[*podDetailWSConn]struct{}
}

var defaultPodDetailHub *PodDetailWSHub

func init() {
	defaultPodDetailHub = NewPodDetailWSHub()
}

// NewPodDetailWSHub creates a new hub.
func NewPodDetailWSHub() *PodDetailWSHub {
	return &PodDetailWSHub{conns: make(map[string]map[*podDetailWSConn]struct{})}
}

// Register adds a connection for the given pod UID.
func (h *PodDetailWSHub) Register(uid string, c *podDetailWSConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conns[uid] == nil {
		h.conns[uid] = make(map[*podDetailWSConn]struct{})
	}
	h.conns[uid][c] = struct{}{}
}

// Unregister removes a connection for the given pod UID.
func (h *PodDetailWSHub) Unregister(uid string, c *podDetailWSConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m, ok := h.conns[uid]; ok {
		delete(m, c)
		if len(m) == 0 {
			delete(h.conns, uid)
		}
	}
}

// Broadcast sends msg to all connections subscribed to uid. Non-blocking; drops if send buffer full.
func (h *PodDetailWSHub) Broadcast(uid string, msg []byte) {
	h.mu.RLock()
	m, ok := h.conns[uid]
	h.mu.RUnlock()
	if !ok || len(m) == 0 {
		return
	}
	for c := range m {
		select {
		case c.send <- msg:
		default:
			// client slow; skip this update
		}
	}
}

// BroadcastPodDetailUpdate is called by ingest handlers after successful DB write. dataType: "metrics"|"processes"|"network"|"events".
func BroadcastPodDetailUpdate(uid string, dataType string) {
	if uid == "" {
		return
	}
	msg, err := json.Marshal(map[string]string{"type": dataType})
	if err != nil {
		return
	}
	defaultPodDetailHub.Broadcast(uid, msg)
}

// PodDetailWS handles GET /api/v1/ws/pod/:uid — upgrades to WebSocket and pushes updates when ingest completes for that UID.
func PodDetailWS() gin.HandlerFunc {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	return func(c *gin.Context) {
		uid := c.Param("uid")
		if uid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "uid required"})
			return
		}
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("[PodDetail WS] upgrade error: %v", err)
			return
		}
		wc := &podDetailWSConn{send: make(chan []byte, 8), conn: conn}
		defaultPodDetailHub.Register(uid, wc)
		defer func() {
			defaultPodDetailHub.Unregister(uid, wc)
			close(wc.send)
			conn.Close()
		}()

		// Write loop: send pushed messages to client
		go func() {
			for msg := range wc.send {
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			}
		}()

		// Read loop: detect client close (ignore incoming messages)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}
}
