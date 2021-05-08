package api_websocket

import (
	"context"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/gofiber/websocket/v2"
	"go.mongodb.org/mongo-driver/bson"
)

func createChannelEmoteSubscription(ctx context.Context, c *Conn, channel string) {
	// Get current user's channel emotes
	var user *datastructure.User
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
	c.helpers.SubscriberChannelUserEmotes(ctx, user.ID.Hex(), func(res emoteSubscriptionResult) {
		c.SendOpDispatch(res, "CHANNEL_EMOTES_UPDATE")
	})
}

type emoteSubscriptionResult struct {
	Emote   *datastructure.Emote `json:"emote"`
	Removed bool                 `json:"removed"`
	Actor   string               `json:"actor"`
}
