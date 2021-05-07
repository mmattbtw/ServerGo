package api_websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

const heartbeatInterval int32 = 90 // Heartbeat interval in seconds

func WebSocket(app fiber.Router) {
	ws := app.Group("/ws")

	ws.Use("/", func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol.
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// WebSocket Endpoint:
	// Subscribe to changes of db collection/document
	ws.Get("/", websocket.New(func(c *websocket.Conn) {
		// Event channels
		chIdentified := make(chan bool)
		chHeartbeat := make(chan WebSocketMessage)
		sendOpGreet(c) // Send an hello payload to the user

		// Create context
		ctx := context.WithValue(context.Background(), WebSocketConnKey, c) // Add connection to context
		ctx = context.WithValue(ctx, WebSocketSeqKey, int32(0))
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Wait for the client to send their first heartbeat
		// Failure to do so in time will disconnect the socket
		go awaitHeartbeat(ctx, chHeartbeat)

		// Get requested subscription
		subscription, err := strconv.ParseInt(c.Query("subscription"), 10, 2)
		if err != nil {
			sendClosure(ctx, 1003, "Missing Subscription Query")
			return
		}
		switch int(subscription) {
		case WebSocketSubscriptionChannelEmotes: // Subscribe: CHANNEL EMOTES
			go createEmoteSubscription(ctx)
		default: // Unknown Subscription
			sendClosure(ctx, 1003, "Unknown Subscription ID")
		}

		for { // Listen to client messages
			if _, b, err := c.ReadMessage(); err == nil {
				var msg WebSocketMessage

				// Handle invalid payload
				if err = json.Unmarshal(b, &msg); err != nil {
					sendClosure(ctx, 1007, fmt.Sprintf("Invalid JSON: %v", err))
					return
				}

				switch msg.Op {
				case WebSocketMessageOpHeartbeat: // Opcode: HEARTBEAT (Client signals the server that the connection is alive)
					chHeartbeat <- msg
					go awaitHeartbeat(ctx, chHeartbeat) // Start waiting for the next heartbeat
				case WebSocketMessageOpIdentify: // Opcode: IDENTIFY (Client wants to sign in to make authorized commands)
					chIdentified <- true
				default:
					sendClosure(ctx, 1003, "Invalid Opcode")
				}
			} else {
				break
			}
		}

		cancel()
		<-ctx.Done()
	}))
}

func sendOpDispatch(ctx context.Context, data interface{}, seq int32) {
	conn := ctx.Value(WebSocketConnKey).(*websocket.Conn)

	_ = conn.WriteJSON(WebSocketMessage{
		Op:       WebSocketMessageOpDispatch,
		Data:     data,
		Sequence: &seq,
	})
}

func sendOpGreet(c *websocket.Conn) {
	_ = c.WriteJSON(WebSocketMessage{
		Op: WebSocketMessageOpHello,
		Data: WebSocketMessageDataHello{
			Timestamp:         time.Now(),
			HeartbeatInterval: heartbeatInterval * 1000,
		},
	})
}

func sendOpHeartbeatAck(c *websocket.Conn) {
	_ = c.WriteJSON(WebSocketMessage{
		Op:   WebSocketMessageOpHeartbeatAck,
		Data: struct{}{},
	})
}

func sendClosure(ctx context.Context, code int, message string) {
	conn := ctx.Value(WebSocketConnKey).(*websocket.Conn)

	b := websocket.FormatCloseMessage(code, message)
	_ = conn.WriteJSON(WebSocketMessage{
		Op: WebSocketMessageOpServerClosure,
		Data: WebSocketMessageDataServerClosure{
			Code:    code,
			Message: message,
		},
	})
	_ = conn.WriteMessage(websocket.CloseMessage, b)
	conn.Close()
}

type WebSocketMessage struct {
	Op       int         `json:"op"` // The message operation code
	Data     interface{} `json:"d"`
	Sequence *int32      `json:"seq"`
}

type WebSocketMessageDataHello struct {
	Timestamp         time.Time `json:"timestamp"`          // The timestamp at which HELLO was sent
	HeartbeatInterval int32     `json:"heartbeat_interval"` // How often the user is expected to send heartbeats in milliseconds
}

type WebSocketMessageDataServerClosure struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	WebSocketMessageOpDispatch int = iota
	WebSocketMessageOpHello
	WebSocketMessageOpHeartbeat
	WebSocketMessageOpHeartbeatAck
	WebSocketMessageOpIdentify
	WebSocketMessageOpServerClosure
)

const (
	WebSocketSubscriptionChannelEmotes int = 1 + iota
)

const WebSocketConnKey = utils.Key("conn")
const WebSocketSeqKey = utils.Key("seq")
