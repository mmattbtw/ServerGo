package redis

import (
	"encoding/json"

	"github.com/go-redis/redis/v8"
)

// Publish to a redis channel
func Publish(channel string, data interface{}) error {
	j, err := json.Marshal(data)
	if err != nil {
		return err
	}

	cmd := Client.Publish(Ctx, channel, j)
	if cmd.Err() != nil {
		return nil
	}

	return nil
}

// Subscribe to a channel on Redis
func Subscribe(ch chan []byte, subscribeTo ...string) *redis.PubSub {
	topic := Client.Subscribe(Ctx, subscribeTo...)
	channel := topic.Channel() // Get a channel for this subscription

	go func() {
		for m := range channel { // Begin listening for messages

			ch <- []byte(m.Payload) // Send to subscriber
		}
	}()

	return topic
}

type PubSubPayloadUserEmotes struct {
	Removed bool   `json:"removed"`
	ID      string `json:"id"`
	Actor   string `json:"actor"`
}
