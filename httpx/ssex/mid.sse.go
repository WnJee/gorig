package ssex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Event represents an SSE event type name.
type Event string

func (event Event) String() string {
	return string(event)
}

// Payload represents a standard SSE message payload.
type Payload struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// Mid returns SSE headers setup middleware.
func Mid() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.AbortWithStatusJSON(http.StatusMethodNotAllowed, gin.H{
				"status":  "error",
				"message": "Only GET method is allowed for SSE",
			})
			return
		}

		header := c.Writer.Header()
		header.Set("Content-Type", "text/event-stream")
		header.Set("Cache-Control", "no-cache")
		header.Set("Connection", "keep-alive")
		header.Set("X-Accel-Buffering", "no")
		header.Set("Access-Control-Allow-Origin", "*")

		c.Writer.Flush()
		c.Next()
	}
}

// SendOK sends a success SSE event with data payload.
func SendOK(c *gin.Context, event Event, data any) error {
	return Send(c, string(event), Payload{
		Status: "ok",
		Data:   data,
	})
}

// SendError sends an error SSE event with message.
func SendError(c *gin.Context, event Event, message string) error {
	return Send(c, string(event), Payload{
		Status:  "error",
		Message: message,
	})
}

// Send sends an event with generic payload marshaled as JSON.
func Send(c *gin.Context, event string, payload any) error {
	var dataStr string
	switch v := payload.(type) {
	case string:
		dataStr = v
	case []byte:
		dataStr = string(v)
	default:
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		dataStr = string(jsonData)
	}

	if event != "" {
		_, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, dataStr)
		if err != nil {
			return err
		}
	} else {
		_, err := fmt.Fprintf(c.Writer, "data: %s\n\n", dataStr)
		if err != nil {
			return err
		}
	}

	c.Writer.Flush()
	return nil
}

// Streamer provides convenient streaming methods.
type Streamer struct {
	ctx *gin.Context
}

// NewStreamer creates a new Streamer instance.
func NewStreamer(c *gin.Context) *Streamer {
	return &Streamer{ctx: c}
}

// Send sends an event and data through the stream.
func (s *Streamer) Send(event string, data any) error {
	return Send(s.ctx, event, data)
}

// SendOK sends a success event.
func (s *Streamer) SendOK(event Event, data any) error {
	return SendOK(s.ctx, event, data)
}

// SendError sends an error event.
func (s *Streamer) SendError(event Event, message string) error {
	return SendError(s.ctx, event, message)
}

// Ping sends a heartbeat ping event.
func (s *Streamer) Ping() error {
	_, err := fmt.Fprintf(s.ctx.Writer, ": ping\n\n")
	if err == nil {
		s.ctx.Writer.Flush()
	}
	return err
}

// Heartbeat starts a background ticker that periodically pings until the client disconnects.
func (s *Streamer) Heartbeat(interval time.Duration) func() {
	ticker := time.NewTicker(interval)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := s.Ping(); err != nil {
					return
				}
			case <-done:
				ticker.Stop()
				return
			case <-s.ctx.Request.Context().Done():
				ticker.Stop()
				return
			}
		}
	}()
	return func() {
		close(done)
	}
}
