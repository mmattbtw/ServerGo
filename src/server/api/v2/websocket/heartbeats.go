package api_websocket

import (
	"context"
	"time"

	"github.com/gofiber/websocket/v2"
)

func awaitHeartbeat(ctx context.Context, waiter chan WebSocketMessageInbound, duration time.Duration) {
	conn := ctx.Value(WebSocketConnKey).(*websocket.Conn)

	ticker := time.NewTicker(duration + time.Second*30)
	defer ticker.Stop()

	// Wait for the user to send a heartbeat, or the socket will timeout
	for {
		select {
		case <-ticker.C: // Client does not send heartbeat: timeout
			sendClosure(ctx, websocket.ClosePolicyViolation, "Client failed to send heartbeat")
			return
		case <-waiter: // Client sends a heartbeat: OK
			// Acknowledge it
			sendOpHeartbeatAck(conn)
			return
		case <-ctx.Done(): // Connection ends
			return
		}
	}
}
