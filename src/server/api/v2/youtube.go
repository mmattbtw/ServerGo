package v2

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/restutil"
	"github.com/SevenTV/ServerGo/src/server/middleware"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

func YouTube(app fiber.Router) fiber.Router {
	yCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	route := app.Group("/auth/youtube")
	yts, err := youtube.NewService(yCtx, option.WithAPIKey(configure.Config.GetString("google.api_key")))
	if err != nil {
		logrus.WithError(err).Error("youtube")
		return route
	}

	route.Get("/request-verification", middleware.UserAuthMiddleware(true), func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/json")
		channelID := c.Query("channel_id")
		if channelID == "" {
			return restutil.ErrBadRequest().Send(c, "channel_id is a required query parameter")
		}
		ctx := c.Context()

		// Try to find the channel given in the channel_id query
		res, err := yts.Channels.List([]string{"snippet", "statistics"}).Id(channelID).Do()
		if err != nil || len(res.Items) == 0 {
			res, err = yts.Channels.List([]string{"snippet", "statistics"}).ForUsername(strings.ToLower(channelID)).Do()
			// If the channel couldn't be found via its ID, we attempt to find it by username
			if err != nil || len(res.Items) == 0 {
				logrus.WithError(err).Errorf("could not find channel with id/username %s", channelID)
				return restutil.ErrBadRequest().Send(c, "Could not find that channel")
			}
		}
		channel := res.Items[0]

		// Make sure this channnel hasn't already been attributed to another account
		if count, _ := mongo.Collection(mongo.CollectionNameUsers).CountDocuments(ctx, bson.M{"yt_id": channel.Id}); count > 0 {
			return restutil.ErrAccessDenied().Send(c, "This channel is already bound to another account")
		}

		// Generate a random string that will be used to verify the requester owns the channel
		r, err := utils.GenerateRandomBytes(24)
		if err != nil {
			logrus.WithError(err).Error("youtube, couldn't generate verification token")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		token := hex.EncodeToString(r)

		// Save the token to state, bound to the channel
		user := c.Locals("user").(*datastructure.User)
		if _, err = redis.Client.SetEX(ctx, fmt.Sprintf("yt_verify:%s:%s", user.ID.Hex(), channel.Id), token, time.Hour*1).Result(); err != nil {
			logrus.WithError(err).Error("youtube, redis")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}

		j, _ := json.Marshal(&verificationRequestResult{
			Token:              token,
			VerificationString: fmt.Sprintf(`[7TV VERIFY]:"%s"`, token),
			ManageChannelURL:   fmt.Sprintf("https://studio.youtube.com/channel/%s/editing/details", channel.Id),
			ChannelID:          channel.Id,
			Channel:            channel,
		})
		return c.Send(j)
	})

	route.Get("/verify", middleware.UserAuthMiddleware(true), func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/json")
		channelID := c.Query("channel_id")
		if channelID == "" {
			return restutil.ErrBadRequest().Send(c, "channel_id is a required query parameter")
		}
		ctx := c.Context()

		// Retrieve the verification token
		user := c.Locals("user").(*datastructure.User)
		rkey := fmt.Sprintf("yt_verify:%s:%s", user.ID.Hex(), channelID)
		tok, err := redis.Client.Get(ctx, rkey).Result()
		if err != nil {
			if err == redis.ErrNil {
				return restutil.ErrAccessDenied().Send(c, "No verification flow is ongoing for that channel")
			}

			logrus.WithError(err).Error("youtube, redis")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		if tok == "" {
			return restutil.ErrAccessDenied().Send(c, "No verification flow is ongoing for that channel")
		}

		// Fetch the channel again
		res, err := yts.Channels.List([]string{"snippet", "statistics"}).Id(channelID).Do()
		if err != nil || len(res.Items) == 0 {
			logrus.WithError(err).Error("youtube")
			return restutil.ErrInternalServer().Send(c, "An error occured trying fetch the youtube channel")
		}

		// Test the channel for its description matching
		channel := res.Items[0]
		regex, err := regexp.Compile(fmt.Sprintf(`\[7TV VERIFY\]:"(%s?)"`, tok))
		if err != nil {
			logrus.WithError(err).Error("youtube, regexp")
			return restutil.ErrInternalServer().Send(c)
		}

		// Check that the string matched
		if ok := regex.MatchString(channel.Snippet.Description); !ok {
			return restutil.ErrAccessDenied().Send(c, fmt.Sprint("The token was not found in the channel's description"))
		}

		// Confirmed user owns the channel!
		// Attribute it to their structure's youtube id
		if _, err = mongo.Collection(mongo.CollectionNameUsers).UpdateByID(ctx, user.ID, bson.M{
			"$set": bson.M{
				"yt_id":                channel.Id,
				"yt_description":       channel.Snippet.Description,
				"yt_profile_image_url": channel.Snippet.Thumbnails.Medium.Url,
				"yt_view_count":        channel.Statistics.ViewCount,
				"yt_subscriber_count":  channel.Statistics.SubscriberCount,
			},
		}); err != nil {
			logrus.WithError(err).Error("youtube, mongo")
			return restutil.ErrInternalServer().Send(c)
		}
		// Remove the key in redis
		if _, err = redis.Client.Del(ctx, rkey).Result(); err != nil {
			logrus.WithError(err).Error("youtube, redis")
		}

		j, _ := json.Marshal(&verifyResult{
			Channel:   channel,
			ChannelID: channel.Id,
			Verified:  true,
			UserID:    user.ID.Hex(),
		})
		return c.Send(j)
	})

	return route
}

type verificationRequestResult struct {
	Token              string           `json:"token"`
	VerificationString string           `json:"verification_string"`
	ManageChannelURL   string           `json:"manage_channel_url"`
	ChannelID          string           `json:"channel_id"`
	Channel            *youtube.Channel `json:"channel"`
}

type verifyResult struct {
	Channel   *youtube.Channel `json:"channel"`
	ChannelID string           `json:"channel_id"`
	Verified  bool             `json:"verified"`
	UserID    string           `json:"user_id"`
}
