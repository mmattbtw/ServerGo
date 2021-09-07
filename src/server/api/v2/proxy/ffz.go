package api_proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/discord"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
)

const baseUrlFFZ = "https://api.frankerfacez.com/v1"

// Get channel emotes from the FFZ provider
func GetChannelEmotesFFZ(ctx context.Context, login string) ([]*datastructure.Emote, error) {
	// Get Twitch User from ID
	usr, err := GetTwitchUser(ctx, login)
	if err != nil {
		return nil, err
	}
	if usr == nil {
		return []*datastructure.Emote{}, nil
	}

	// Set Request URI
	uri := fmt.Sprintf("%v/rooms/id/%v", baseUrlFFZ, usr.ID)

	// Send the request
	resp, err := cache.CacheGetRequest(ctx, uri, time.Minute*10, time.Minute*15)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == fiber.StatusTooManyRequests {
		// Log 429s
		log.WithFields(log.Fields{
			"blame_provider":      "FFZ",
			"rl-limit-header":     resp.Header["ratelimit-limit"],
			"rl-remaining-header": resp.Header["ratelimit-remaining"],
			"rl-reset-header":     resp.Header["ratelimit-reset"],
			"rl-retry-after":      resp.Header["ratelimit-retry-after"],
		})
		go discord.SendWebhook("alerts", &discordgo.WebhookParams{
			Content: fmt.Sprintf("[FFZ] 429 Too Many Requests @ `%s`", uri),
		})
	}

	// Decode response
	var emoteResponse getEmotesResponseFFZ
	if err := json.Unmarshal(resp.Body, &emoteResponse); err != nil {
		return nil, err
	}

	emotes, err := ffzTo7TV(emoteResponse.Emotes)
	if err != nil {
		return nil, err
	}

	return emotes, nil
}

func GetGlobalEmotesFFZ(ctx context.Context) ([]*datastructure.Emote, error) {
	uri := fmt.Sprintf("%v/set/global", baseUrlFFZ)

	// Send request
	resp, err := cache.CacheGetRequest(ctx, uri, time.Hour*4, time.Minute*15)
	if err != nil {
		return nil, err
	}

	var emoteResponse getEmoteSetsResponseFFZ
	if err := json.Unmarshal(resp.Body, &emoteResponse); err != nil {
		return nil, err
	}

	var allEmotes []emoteFFZ
	for _, s := range emoteResponse.Sets {
		allEmotes = append(allEmotes, s.Emotes...)
	}

	emotes, err := ffzTo7TV(allEmotes)
	if err != nil {
		return nil, err
	}

	return emotes, nil
}

// Convert a FFZ emote object into 7TV
func ffzTo7TV(emotes []emoteFFZ) ([]*datastructure.Emote, error) {
	result := make([]*datastructure.Emote, len(emotes))

	for i, emote := range emotes {
		// Generate URLs list
		urls := make([][]string, 3)
		for i, s := range []int8{1, 2, 4} {
			a := make([]string, 2)
			a[0] = fmt.Sprintf("%d", s)
			a[1] = getCdnURL_FFZ(emote.ID, int8(s))

			urls[i] = a
		}

		result[i] = &datastructure.Emote{
			Name:       emote.Name,
			Width:      [4]int16{emote.Width, 0, 0, 0},
			Height:     [4]int16{emote.Height, 0, 0, 0},
			Visibility: 0,
			Mime:       "image/png",
			Status:     datastructure.EmoteStatusLive,
			Owner: &datastructure.User{
				Login:       emote.Owner.Name,
				DisplayName: emote.Owner.DisplayName,
				TwitchID:    "",
			},

			Provider:   "FFZ",
			ProviderID: utils.StringPointer(fmt.Sprint(emote.ID)),
			URLs:       urls,
		}
	}

	return result, nil
}

func getCdnURL_FFZ(emoteID int32, size int8) string {
	return fmt.Sprintf("https://cdn.frankerfacez.com/emoticon/%d/%d", emoteID, size)
}

type emoteFFZ struct {
	ID          int32     `json:"id"`
	Name        string    `json:"name"`
	Width       int16     `json:"width"`
	Height      int16     `json:"height"`
	Public      bool      `json:"public"`
	Hidden      bool      `json:"hidden"`
	Owner       userFFZ   `json:"owner"`
	Status      int32     `json:"status"`
	UsageCount  int32     `json:"usage_count"`
	CreatedAt   time.Time `json:"created_at"`
	Sizes       []int8    `json:"sizes"`
	LastUpdated time.Time `json:"last_updated"`
}

type userFFZ struct {
	ID          int32  `json:"_id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

type getEmotesResponseFFZ struct {
	Emotes []emoteFFZ `json:"emotes"`
}

type getEmoteSetsResponseFFZ struct {
	Sets map[string]emoteSetFFZ `json:"sets"`
}

type emoteSetFFZ struct {
	ID     int32      `json:"id"`
	Title  string     `json:"title"`
	Emotes []emoteFFZ `json:"emoticons"`
}
