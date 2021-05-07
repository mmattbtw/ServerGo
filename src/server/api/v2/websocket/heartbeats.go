package api_websocket

import (
	"context"
	"time"

	"github.com/gofiber/websocket/v2"
)

func awaitHeartbeat(ctx context.Context, waiter chan WebSocketMessage) {
	conn := ctx.Value(WebSocketConnKey).(*websocket.Conn)

	dur := time.Second * time.Duration(heartbeatInterval)
	ticker := time.NewTicker(dur + time.Second*30)
	defer ticker.Stop()

	// Wait for the user to send a heartbeat, or the socket will timeout
	for {
		select {
		case <-ticker.C: // Client does not send heartbeat: timeout
			sendClosure(ctx, 1000, "Client failed to send heartbeat")
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
