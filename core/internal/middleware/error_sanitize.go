package middleware

import (
	"bytes"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// responseBuffer buffers the response so we can sanitize 5xx body before sending.
type responseBuffer struct {
	gin.ResponseWriter
	body   *bytes.Buffer
	status int
	wrote  bool
}

func (w *responseBuffer) Write(b []byte) (int, error) {
	if !w.wrote {
		w.status = http.StatusOK
		w.wrote = true
	}
	return w.body.Write(b)
}

func (w *responseBuffer) WriteHeader(code int) {
	w.status = code
	w.wrote = true
	// Do not write to real ResponseWriter yet
}

func (w *responseBuffer) WriteToReal() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.ResponseWriter.WriteHeader(w.status)
	if w.status >= 500 && w.status < 600 && w.body.Len() > 0 {
		log.Printf("[API] 5xx sanitized for client: status=%d preview=%s", w.status, truncate(w.body.Bytes(), 200))
		w.ResponseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.ResponseWriter.Write([]byte(`{"error":"Internal server error"}`))
	} else {
		w.ResponseWriter.Write(w.body.Bytes())
	}
}

func truncate(b []byte, max int) string {
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}

// ErrorSanitize prevents leaking internal error details. On 5xx, replaces body with generic message.
func ErrorSanitize() gin.HandlerFunc {
	return func(c *gin.Context) {
		buf := &responseBuffer{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = buf
		c.Next()
		buf.WriteToReal()
	}
}
