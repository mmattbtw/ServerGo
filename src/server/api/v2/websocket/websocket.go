package api_websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	log "github.com/sirupsen/logrus"
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
		log.Infof("<WS> Connect: %v", c.RemoteAddr().String())

		// Event channels
		chIdentified := make(chan bool)
		chHeartbeat := make(chan WebSocketMessageInbound)
		sendOpGreet(c) // Send an hello payload to the user

		// Create context
		ctx := context.WithValue(context.Background(), WebSocketConnKey, c) // Add connection to context
		ctx = context.WithValue(ctx, WebSocketSeqKey, int32(0))
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Wait for the client to send their first heartbeat
		// Failure to do so in time will disconnect the socket
		go awaitHeartbeat(ctx, chHeartbeat)

		subscriptions := make(map[int32]bool)
		for { // Listen to client messages
			if _, b, err := c.ReadMessage(); err == nil {
				var msg WebSocketMessageInbound

				// Handle invalid payload
				if err = json.Unmarshal(b, &msg); err != nil {
					sendClosure(ctx, websocket.CloseInvalidFramePayloadData, fmt.Sprintf("Invalid JSON: %v", err))
					return
				}

				switch msg.Op {
				// Opcode: HEARTBEAT (Client signals the server that the connection is alive)
				case WebSocketMessageOpHeartbeat:
					chHeartbeat <- msg
					go awaitHeartbeat(ctx, chHeartbeat) // Start waiting for the next heartbeat
				// Opcode: IDENTIFY (Client wants to sign in to make authorized commands)
				case WebSocketMessageOpIdentify:
					chIdentified <- true

					// Opcode: SUBSCRIBE (Client wants to start receiving events from a specified source)
				case WebSocketMessageOpSubscribe:
					var data WebSocketMessageDataSubscribe
					decodeMessageData(ctx, msg, &data) // Decode message data

					subscription := data.Type // The subscription that the client wants to create
					// Verify that the user is not already subscribed
					if subscriptions[subscription] {
						sendClosure(ctx, websocket.ClosePolicyViolation, "Subscription Already Active")
						break
					}
					subscriptions[subscription] = true // Set subscription as active

					switch int(subscription) {
					case WebSocketSubscriptionChannelEmotes: // Subscribe: CHANNEL EMOTES
						channel := data.Params["channel"]
						go createChannelEmoteSubscription(ctx, channel)

					default: // Unknown Subscription
						sendClosure(ctx, 1003, "Unknown Subscription ID")
					}

				default:
					sendClosure(ctx, 1003, "Invalid Opcode")
				}
			} else {
				break
			}
		}

		cancel()

		log.Infof("<WS> Disconnect: %v", c.RemoteAddr().String())
		<-ctx.Done()
	}))
}

func sendOpDispatch(ctx context.Context, data interface{}, seq int32) {
	conn := ctx.Value(WebSocketConnKey).(*websocket.Conn)

	_ = conn.WriteJSON(WebSocketMessageOutbound{
		Op:       WebSocketMessageOpDispatch,
		Data:     data,
		Sequence: &seq,
	})
}

func sendOpGreet(c *websocket.Conn) {
	_ = c.WriteJSON(WebSocketMessageOutbound{
		Op: WebSocketMessageOpHello,
		Data: WebSocketMessageDataHello{
			Timestamp:         time.Now(),
			HeartbeatInterval: heartbeatInterval * 1000,
		},
	})
}

func sendOpHeartbeatAck(c *websocket.Conn) {
	_ = c.WriteJSON(WebSocketMessageOutbound{
		Op:   WebSocketMessageOpHeartbeatAck,
		Data: struct{}{},
	})
}

func sendClosure(ctx context.Context, code int, message string) {
	conn := ctx.Value(WebSocketConnKey).(*websocket.Conn)

	b := websocket.FormatCloseMessage(code, message)
	_ = conn.WriteJSON(WebSocketMessageOutbound{
		Op: WebSocketMessageOpServerClosure,
		Data: WebSocketMessageDataServerClosure{
			Code:    code,
			Message: message,
		},
	})
	_ = conn.WriteMessage(websocket.CloseMessage, b)
	conn.Close()
}

func decodeMessageData(ctx context.Context, msg WebSocketMessageInbound, v interface{}) {
	if err := json.Unmarshal(msg.Data, &v); err != nil {
		sendClosure(ctx, websocket.CloseInvalidFramePayloadData, fmt.Sprintf("Invalid JSON: %v", err))
	}
}

type WebSocketMessageOutbound struct {
	Op       int         `json:"op"` // The message operation code
	Data     interface{} `json:"d"`
	Sequence *int32      `json:"seq"`
}

type WebSocketMessageInbound struct {
	Op   int             `json:"op"`
	Data json.RawMessage `json:"d"`
}

type WebSocketMessageDataHello struct {
	Timestamp         time.Time `json:"timestamp"`          // The timestamp at which HELLO was sent
	HeartbeatInterval int32     `json:"heartbeat_interval"` // How often the user is expected to send heartbeats in milliseconds
}

type WebSocketMessageDataServerClosure struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type WebSocketMessageDataSubscribe struct {
	Type   int32 `json:"type"`
	Params map[string]string
}

const (
	WebSocketMessageOpDispatch int = iota
	WebSocketMessageOpHello
	WebSocketMessageOpHeartbeat
	WebSocketMessageOpHeartbeatAck
	WebSocketMessageOpIdentify
	WebSocketMessageOpServerClosure
	WebSocketMessageOpSubscribe
)

const (
	WebSocketSubscriptionChannelEmotes int = 1 + iota
)

const WebSocketConnKey = utils.Key("conn")
const WebSocketSeqKey = utils.Key("seq")
