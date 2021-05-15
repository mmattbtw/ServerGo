package api_websocket

import (
	"context"
	"time"

	"github.com/gofiber/websocket/v2"
)

func awaitHeartbeat(ctx context.Context, c *Conn, waiter chan WebSocketMessageInbound, duration time.Duration) {
	ticker := time.NewTicker(duration + time.Second*30)
	defer ticker.Stop()

	// Wait for the user to send a heartbeat, or the socket will timeout
	for {
		select {
		case <-ctx.Done(): // Connection ends
			return
		case <-ticker.C: // Client does not send heartbeat: timeout
			c.SendClosure(websocket.ClosePolicyViolation, "Client failed to send heartbeat")
			return
		case <-waiter: // Client sends a heartbeat: OK
			// Acknowledge it
			c.SendOpHeartbeatAck()
			c.Refresh() // Refresh the connection's key expire in redis
			return
		}
	}
}
