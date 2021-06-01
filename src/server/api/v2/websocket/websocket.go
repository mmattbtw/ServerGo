package api_websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const heartbeatInterval int32 = 90 // Heartbeat interval in seconds

var Connections = make(map[uuid.UUID]*Conn)

type Stat struct {
	UUID          uuid.UUID               `json:"id"`     // The connection's unique ID
	Sequence      int32                   `json:"seq"`    // The amount of events sent by the server to this connection
	CreatedAt     time.Time               `json:"age"`    // The time at which this connection became active
	Subscriptions []WebSocketSubscription `json:"subs"`   // A list of active subscription types&
	Closed        bool                    `json:"closed"` // True if the connection has been closed
	IP            string                  `json:"ip"`     // The client's IP Address
	Lock          *sync.Mutex
	RedisKey      string
}

func WebSocket(app fiber.Router) {
	ws := app.Group("/ws")

	ws.Use("/", func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol.
		if websocket.IsWebSocketUpgrade(c) {
			var ip string
			if len(c.IPs()) > 0 {
				ip = c.IPs()[0]
			} else {
				ip = c.Context().RemoteIP().String()
			}

			c.Locals("ClientIP", ip)
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// WebSocket Endpoint:
	// Subscribe to event channels
	ws.Get("/", websocket.New(func(conn *websocket.Conn) {
		c := transform(conn)
		c.SendOpGreet() // Send an hello payload to the user

		// This socket has connected
		log.Infof("<WS> Connect: %v", c.RemoteAddr().String())
		ctx, cancel := context.WithCancel(context.WithValue(context.Background(), WebSocketConnKey, c))
		c.Register(ctx)
		Connections[c.Stat.UUID] = c

		// Event channels
		chIdentified := make(chan bool)
		chHeartbeat := make(chan WebSocketMessageInbound)

		// Wait for the client to send their first heartbeat
		// Failure to do so in time will disconnect the socket
		go awaitHeartbeat(ctx, c, chHeartbeat, 0)

		// We will disconnect clients who don't create a subscription
		// These connections are considered stale, as they serve no purpose
		noOpTimeout := time.AfterFunc(time.Second*45, func() {
			if len(c.Stat.Subscriptions) == 0 {
				c.SendClosure(websocket.CloseNormalClosure, "Connection is stale")
			}
		})

		for { // Listen to client messages
			if _, b, err := c.ReadMessage(); err == nil {
				var msg WebSocketMessageInbound

				// Handle invalid payload
				if err = json.Unmarshal(b, &msg); err != nil {
					c.SendClosure(websocket.CloseInvalidFramePayloadData, fmt.Sprintf("Invalid JSON: %v", err))
					break
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
					var data WebSocketSubscription
					c.decodeMessageData(ctx, msg, &data) // Decode message data

					subscription := data.Type // The subscription that the client wants to create

					c.Stat.Subscriptions = append(c.Stat.Subscriptions, data)
					c.Register(ctx)
					noOpTimeout.Stop() // Prevent a no-op timeout from happening: the user has done something

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

		// Handle connection removal
		c.Unregister(ctx)
		c.Stat.Closed = true             // Set closed stat to true
		delete(Connections, c.Stat.UUID) // Remove from connections map
		close(chIdentified)
		close(chHeartbeat)
	}))
}

type Conn struct {
	*websocket.Conn
	helpers WebSocketHelpers
	Stat    Stat
}

func transform(ws *websocket.Conn) *Conn {
	id := uuid.New()
	return &Conn{
		ws,
		WebSocketHelpers{},
		Stat{
			UUID:          id,
			Subscriptions: []WebSocketSubscription{},
			CreatedAt:     time.Now(),
			Lock:          &sync.Mutex{},
			RedisKey:      fmt.Sprintf("ws:connections:%v", id.String()),
			IP:            ws.Locals("ClientIP").(string),
		},
	}
}

func (c *Conn) SendOpDispatch(ctx context.Context, data interface{}, t string) {
	// Increase sequence
	c.Stat.Sequence++
	_ = redis.Client.HIncrBy(ctx, c.Stat.RedisKey, "seq", 1)

	c.sendMessage(&WebSocketMessageOutbound{
		Op:       WebSocketMessageOpDispatch,
		Data:     data,
		Sequence: &c.Stat.Sequence,
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
	if c == nil || c.Stat.Closed {
		return
	}
	c.Stat.Lock.Lock()
	c.Stat.Closed = true

	b := websocket.FormatCloseMessage(code, message)

	if err := c.WriteMessage(websocket.CloseMessage, b); err != nil {
		log.Errorf("WebSocket, err=failed to write closure message, %v", err)
	}
	c.Close()
	c.Stat.Lock.Unlock()
}

// Register the connection in the global redis store
func (c *Conn) Register(ctx context.Context) {
	data := make([]string, 8)
	data = append(data, "id", c.Stat.UUID.String())
	data = append(data, "ip", c.Stat.IP)
	data = append(data, "seq", strconv.Itoa(int(c.Stat.Sequence)))
	data = append(data, "age", c.Stat.CreatedAt.String())
	if j, err := json.Marshal(c.Stat.Subscriptions); err == nil {
		data = append(data, "subs", string(j))
	}

	if err := redis.Client.HSet(ctx, c.Stat.RedisKey, data).Err(); err != nil {
		log.Errorf("WebSocket, err=could not register socket, %v", err)
	}
	redis.Client.Expire(ctx, c.Stat.RedisKey, time.Second*90)
}

// Bump the EXPIRE for this connection in the global redis store
func (c *Conn) Refresh(ctx context.Context) {
	redis.Client.Expire(ctx, c.Stat.RedisKey, time.Second*time.Duration(heartbeatInterval)+time.Second*60)
}

// Unregister the connection in the global redis store
func (c *Conn) Unregister(ctx context.Context) {
	redis.Client.Del(ctx, c.Stat.RedisKey) // Remove key
}

func (c *Conn) sendMessage(message *WebSocketMessageOutbound) {
	if c.Stat.Closed {
		return
	}
	c.Stat.Lock.Lock()

	if err := c.WriteJSON(message); err != nil {
		log.Errorf("WebSocket, err=failed to write json message, %v", err)
	}
	c.Stat.Lock.Unlock()
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

type WebSocketSubscription struct {
	Type   int8              `json:"type"`
	Params map[string]string `json:"params"`
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
