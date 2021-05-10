package api_websocket

import (
	"context"
	"strings"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
)

func createChannelEmoteSubscription(ctx context.Context, c *Conn, channel string) {
	// Subscribe to these events with Redis
	c.helpers.SubscriberChannelUserEmotes(ctx, strings.ToLower(channel), func(res emoteSubscriptionResult) {
		c.SendOpDispatch(res, "CHANNEL_EMOTES_UPDATE")
	})
}

type emoteSubscriptionResult struct {
	Emote   *datastructure.Emote `json:"emote"`
	Removed bool                 `json:"removed"`
	Actor   string               `json:"actor"`
}
