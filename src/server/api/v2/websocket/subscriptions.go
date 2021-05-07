package api_websocket

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/websocket/v2"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func createChannelEmoteSubscription(ctx context.Context, channel string) {
	// Get current user's channel emotes
	var user *datastructure.User
	if err := cache.FindOne("users", "", bson.M{
		"login": channel,
	}, &user, &options.FindOneOptions{
		Projection: bson.M{
			"_id":    1,
			"emotes": 1,
		},
	}); err != nil {
		if err == mongo.ErrNoDocuments {
			sendClosure(ctx, websocket.CloseInvalidFramePayloadData, "Unknown User")
		} else {
			sendClosure(ctx, websocket.CloseInternalServerErr, err.Error())
		}

		return
	}

	// Subscribe to these events with Redis
	ch := make(chan []byte)
	channelName := fmt.Sprintf("users:%v:emotes", user.ID.Hex())
	topic := redis.Subscribe(ch, channelName)

	currentEmoteList := make([]string, len(user.EmoteIDs))
	for i, id := range user.EmoteIDs {
		currentEmoteList[i] = id.Hex()
	}
	for {
		select {
		case b := <-ch: // Listen for changes
			// Get new emote list
			var d redis.PubSubPayloadUserEmotes
			if err := json.Unmarshal(b, &d); err != nil {
				log.Errorf("websocket, err=%v", err)
				continue
			}
			newEmoteList := make([]string, (len(d.List)))
			for i, id := range d.List {
				newEmoteList[i] = id
			}

			// Copy new emote list to a slice
			// We will remove existing emotes, then use the remaining ones to figure which have been added
			added := make([]string, len(newEmoteList))
			copy(added, newEmoteList)

			removed := make([]string, 0)
			for _, id := range currentEmoteList {
				if utils.Contains(newEmoteList, id) {
					index := utils.SliceIndexOf(currentEmoteList, id)
					if index >= len(added) {
						continue
					}
					added[index] = ""

					continue
				}

				// We add emotes not in the new emote list value to the removed slice
				removed = append(removed, id)
			}
			// Remove empty strings from added slice
			{
				r := make([]string, 0)
				for _, s := range added {
					if s != "" {
						r = append(r, s)
					}
				}
				added = r
			}
			currentEmoteList = newEmoteList

			// Increase sequence
			seq := ctx.Value(WebSocketSeqKey).(int32)
			seq++
			ctx = context.WithValue(ctx, WebSocketSeqKey, seq)

			// Send dispatch
			sendOpDispatch(ctx, emoteSubscriptionResult{
				Added:   added,
				Removed: removed,
				Actor:   d.Actor,
			}, "CHANNEL_EMOTES_UPDATE", seq)
		case <-ctx.Done():
			_ = topic.Unsubscribe(redis.Ctx, channelName)
			return
		}
	}
}

type emoteSubscriptionResult struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Actor   string   `json:"actor"`
}
