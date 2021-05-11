package api_websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const heartbeatInterval int32 = 90 // Heartbeat interval in seconds

type Stat struct {
	UUID          uuid.UUID // The connection's unique ID
	Sequence      int32     // The amount of events sent by the server to this connection
	CreatedAt     time.Time // The time at which this connection became active
	Subscriptions []int8    // A list of active subscription types&
	Closed        bool      // True if the connection has been closed
}

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
	ws.Get("/", websocket.New(func(conn *websocket.Conn) {
		c := transform(conn)
		c.SendOpGreet() // Send an hello payload to the user

		log.Infof("<WS> Connect: %v", c.RemoteAddr().String())

		// Event channels
		chIdentified := make(chan bool)
		chHeartbeat := make(chan WebSocketMessageInbound)

		// Create context
		ctx := context.WithValue(context.Background(), WebSocketConnKey, c) // Add connection to context
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Wait for the client to send their first heartbeat
		// Failure to do so in time will disconnect the socket
		go awaitHeartbeat(ctx, c, chHeartbeat, 0)

		active := make(map[int8]bool)
		for { // Listen to client messages
			if _, b, err := c.ReadMessage(); err == nil {
				var msg WebSocketMessageInbound

				// Handle invalid payload
				if err = json.Unmarshal(b, &msg); err != nil {
					c.SendClosure(websocket.CloseInvalidFramePayloadData, fmt.Sprintf("Invalid JSON: %v", err))
					return
				}

				switch msg.Op {
				// Opcode: HEARTBEAT (Client signals the server that the connection is alive)
				case WebSocketMessageOpHeartbeat:
					chHeartbeat <- msg
					go awaitHeartbeat(ctx, c, chHeartbeat, time.Second*time.Duration(heartbeatInterval)) // Start waiting for the next heartbeat
				// Opcode: IDENTIFY (Client wants to sign in to make authorized commands)
				case WebSocketMessageOpIdentify:
					chIdentified <- true

				// Opcode: SUBSCRIBE (Client wants to start receiving events from a specified source)
				case WebSocketMessageOpSubscribe:
					var data WebSocketMessageDataSubscribe
					c.decodeMessageData(ctx, msg, &data) // Decode message data

					subscription := data.Type // The subscription that the client wants to create
					// Verify that the user is not already subscribed
					if active[subscription] {
						c.SendClosure(websocket.ClosePolicyViolation, "Already Subscribed")
						break
					}
					active[subscription] = true // Set subscription as active

					switch subscription {
					case WebSocketSubscriptionChannelEmotes: // Subscribe: CHANNEL EMOTES
						channel := data.Params["channel"]
						go createChannelEmoteSubscription(ctx, c, channel)

					default: // Unknown Subscription
						c.SendClosure(1003, "Unknown Subscription Type")
					}

				default:
					c.SendClosure(1003, "Invalid Opcode")
				}
			} else {
				break
			}
		}

		cancel() // Cancel the context so everything closes up

		log.Infof("<WS> Disconnect: %v", c.RemoteAddr().String())
		c.stat.Closed = true
		<-ctx.Done()
	}))
}

type Conn struct {
	*websocket.Conn
	helpers WebSocketHelpers
	stat    Stat
}

func transform(ws *websocket.Conn) *Conn {
	return &Conn{
		ws,
		WebSocketHelpers{},
		Stat{
			UUID:      uuid.New(),
			CreatedAt: time.Now(),
		},
	}
}

func (c *Conn) SendOpDispatch(data interface{}, t string) {
	// Increase sequence
	c.stat.Sequence++

	c.sendMessage(&WebSocketMessageOutbound{
		Op:       WebSocketMessageOpDispatch,
		Data:     data,
		Sequence: &c.stat.Sequence,
		Type:     &t,
	})
}

func (c *Conn) SendOpGreet() {
	c.sendMessage(&WebSocketMessageOutbound{
		Op: WebSocketMessageOpHello,
		Data: WebSocketMessageDataHello{
			Timestamp:         time.Now(),
			HeartbeatInterval: heartbeatInterval * 1000,
		},
	})
}

func (c *Conn) SendOpHeartbeatAck() {
	c.sendMessage(&WebSocketMessageOutbound{
		Op:   WebSocketMessageOpHeartbeatAck,
		Data: struct{}{},
	})
}

func (c *Conn) SendClosure(code int, message string) {
	if c.stat.Closed {
		return
	}

	b := websocket.FormatCloseMessage(code, message)

	if err := c.WriteMessage(websocket.CloseMessage, b); err != nil {
		log.Errorf("WebSocket, err=failed to write closure message, %v", err)
	}
	c.Close()
}

func (c *Conn) sendMessage(message *WebSocketMessageOutbound) {
	if c.stat.Closed {
		return
	}

	if err := c.WriteJSON(message); err != nil {
		log.Errorf("WebSocket, err=failed to write json message, %v", err)
	}
}

func (c *Conn) decodeMessageData(ctx context.Context, msg WebSocketMessageInbound, v interface{}) {
	if err := json.Unmarshal(msg.Data, &v); err != nil {
		c.SendClosure(websocket.CloseInvalidFramePayloadData, fmt.Sprintf("Invalid JSON: %v", err))
	}
}

type WebSocketMessageOutbound struct {
	Op       int8        `json:"op"` // The message operation code
	Data     interface{} `json:"d"`
	Sequence *int32      `json:"seq"`
	Type     *string     `json:"t"`
}

type WebSocketMessageInbound struct {
	Op   int8            `json:"op"`
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
	Type   int8 `json:"type"`
	Params map[string]string
}

const (
	WebSocketMessageOpDispatch int8 = iota
	WebSocketMessageOpHello
	WebSocketMessageOpHeartbeat
	WebSocketMessageOpHeartbeatAck
	WebSocketMessageOpIdentify
	WebSocketMessageOpServerClosure
	WebSocketMessageOpSubscribe
)

const (
	WebSocketSubscriptionChannelEmotes int8 = 1 + iota
)

const WebSocketConnKey = utils.Key("conn")
const WebSocketSeqKey = utils.Key("seq")
const WebSocketStatKey = utils.Key("stat")
