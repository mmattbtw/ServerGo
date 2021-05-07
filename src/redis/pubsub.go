package redis

import (
	"encoding/json"
	"math/rand"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

var Publisher *redis.Client
var Subscriber *redis.Client

// Publish to a redis channel
func Publish(channel string, data interface{}) error {
	id := rand.Int63()
	payload := PubSubMessage{
		ID:   id,
		Data: data,
	}

	p, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	cmd := Client.Publish(Ctx, channel, p)
	if cmd.Err() != nil {
		return nil
	}

	return nil
}

// Subscribe to a channel on Redis
func Subscribe(ch chan *PubSubMessage, subscribeTo ...string) *redis.PubSub {
	topic := Subscriber.Subscribe(Ctx, subscribeTo...)
	channel := topic.Channel() // Get a channel for this subscription

	go func() {
		for m := range channel { // Begin listening for messages
			var msg PubSubMessage
			// Decode message
			if err := json.Unmarshal([]byte(m.Payload), &msg); err != nil {
				log.Errorf("redis, could not decode message in subscription, err=%v", err)
				continue
			}

			ch <- &msg // Send to subscriber
		}
	}()

	return topic
}

type UserEmotes struct {
	List  []string            `json:"list"`
	Actor *datastructure.User `json:"actor"`
}
