package api_websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/gofiber/websocket/v2"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func main() {
}

func createChannelEmoteSubscription(ctx context.Context, c *Conn, channel string) {
	// Get current user's channel emotes
	var user *datastructure.User
	var lock sync.Mutex
	if err := cache.FindOne("users", "", bson.M{
		"login": channel,
	}, &user); err != nil {
		if err == mongo.ErrNoDocuments {
			c.SendClosure(websocket.CloseInvalidFramePayloadData, "Unknown User")
		} else {
			c.SendClosure(websocket.CloseInternalServerErr, err.Error())
		}

		return
	}

	// Subscribe to these events with Redis
	ch := make(chan []byte)
	channelName := fmt.Sprintf("users:%v:emotes", user.ID.Hex())
	topic := redis.Subscribe(ch, channelName)

	for {
		select {
		case b := <-ch: // Listen for changes
			lock.Lock()

			// Get new emote list
			var d redis.PubSubPayloadUserEmotes
			if err := json.Unmarshal(b, &d); err != nil {
				log.Errorf("websocket, err=%v", err)
				continue
			}

			// Get full emote objects for added
			var emote *datastructure.Emote
			id, err := primitive.ObjectIDFromHex(d.ID)
			if err != nil {
				continue
			}

			if err := cache.FindOne("emotes", "", bson.M{"_id": id}, &emote); err != nil {
				fmt.Println("err", err)
			}
			urls := datastructure.GetEmoteURLs(*emote)
			emote.URLs = urls
			emote.Provider = "7TV"

			// Send dispatch
			c.SendOpDispatch(emoteSubscriptionResult{
				Emote: &datastructure.Emote{
					ID:         emote.ID,
					Provider:   emote.Provider,
					Visibility: emote.Visibility,
					Mime:       emote.Mime,
					Name:       emote.Name,
					URLs:       emote.URLs,
				},
				Removed: d.Removed,
				Actor:   d.Actor,
			}, "CHANNEL_EMOTES_UPDATE")
			lock.Unlock()
		case <-ctx.Done():
			_ = topic.Unsubscribe(redis.Ctx, channelName)
			return
		}
	}
}

type emoteSubscriptionResult struct {
	Emote   *datastructure.Emote `json:"emote"`
	Removed bool                 `json:"removed"`
	Actor   string               `json:"actor"`
}
