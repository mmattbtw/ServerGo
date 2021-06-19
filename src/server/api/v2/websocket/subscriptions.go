package api_websocket

import (
	"context"
	"strings"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
)

func createChannelEmoteSubscription(ctx context.Context, c *Conn, data WebSocketSubscription) {
	channel := data.Params["channel"]
	for _, s := range c.Stat.Subscriptions {
		if strings.EqualFold(s.Params["channel"], channel) {
			return
		}
	}

	// Subscribe to these events with Redis
	c.Stat.Subscriptions = append(c.Stat.Subscriptions, data)
	c.helpers.SubscriberChannelUserEmotes(ctx, strings.ToLower(channel), func(res emoteSubscriptionResult) {
		c.SendOpDispatch(ctx, res, "CHANNEL_EMOTES_UPDATE")
	})
}

type emoteSubscriptionResult struct {
	Emote   *datastructure.Emote `json:"emote"`
	Removed bool                 `json:"removed"`
	Channel string               `json:"channel"`
	Actor   string               `json:"actor"`
}
