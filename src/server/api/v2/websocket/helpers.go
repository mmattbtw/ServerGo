package api_websocket

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WebSocketHelpers struct {
}

var subscriberChannelsUserEmotes = make(map[string]chan emoteSubscriptionResult)
var subscriberCallersUserEmotes = make(map[string]map[uuid.UUID]func(emoteSubscriptionResult))

/*
* SUBSCRIBER CHANNEL: User Emotes
*
* It listens for updates on a user's channel emotes and forwards it to all active callers
 */
func (h *WebSocketHelpers) SubscriberChannelUserEmotes(ctx context.Context, userID string, cb func(emoteSubscriptionResult)) {
	c := ctx.Value(WebSocketConnKey).(*Conn)
	if subscriberCallersUserEmotes[userID] == nil {
		subscriberCallersUserEmotes[userID] = make(map[uuid.UUID]func(emoteSubscriptionResult))
	}
	subscriberCallersUserEmotes[userID][c.stat.UUID] = cb

	// Subscribe to these events with Redis
	ch := subscriberChannelsUserEmotes[userID] // Try to find an existing channel made for the selected user
	if ch == nil {
		ch = make(chan emoteSubscriptionResult) // Create the channel
		subscriberChannelsUserEmotes[userID] = ch

		rCh := make(chan []byte)
		key := fmt.Sprintf("users:%v:emotes", userID)
		_ = redis.Subscribe(rCh, key)

		go func() {
			for b := range rCh {
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
					continue
				}
				urls := datastructure.GetEmoteURLs(*emote)
				emote.URLs = urls
				emote.Provider = "7TV"

				ch <- emoteSubscriptionResult{
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
				}
			}
		}()

		go func() {
			for result := range ch {
				for _, fn := range subscriberCallersUserEmotes[userID] {
					fn(result)
				}
			}
		}()

	}

	// nolint:gosimple
	for {
		select {
		case <-ctx.Done():
			delete(subscriberCallersUserEmotes[userID], c.stat.UUID)
			return
		}
	}
}
