package api_websocket

import (
	"context"
	"fmt"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/gofiber/websocket/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func createChannelEmoteSubscription(ctx context.Context, channel string) {
	// Get current user's channel emotes
	var user *mongo.User
	if err := cache.FindOne("users", "", bson.M{
		"login": channel,
	}, &user, &options.FindOneOptions{
		Projection: bson.M{
			"_id": 1,
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
	ch := make(chan *redis.PubSubMessage)
	channelName := fmt.Sprintf("users:%v:emotes", user.ID.Hex())
	topic := redis.Subscribe(ch, channelName)

	for {
		select {
		case ev := <-ch: // Listen for changes
			// Increase sequence
			seq := ctx.Value(WebSocketSeqKey).(int32)
			seq++
			ctx = context.WithValue(ctx, WebSocketSeqKey, seq)

			// Send dispatch
			sendOpDispatch(ctx, ev.Data, seq)
		case <-ctx.Done():
			topic.Unsubscribe(redis.Ctx, channelName)
			return
		}
	}
}
