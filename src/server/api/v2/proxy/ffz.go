package api_proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/utils"
)

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
	uri := fmt.Sprintf("%v/cached/frankerfacez/users/twitch/%s", baseUrlBTTV, usr.ID)

	// Send request
	resp, err := cache.CacheGetRequest(ctx, uri, time.Minute*30, time.Minute*15)
	if err != nil {
		return nil, err
	}

	var emotes []emoteBTTVFFZ
	err = json.Unmarshal(resp.Body, &emotes)
	if err != nil {
		return nil, err
	}

	// Convert these emotes into a 7TV emote object
	result := make([]*datastructure.Emote, len(emotes))
	for i, e := range emotes {
		emote, err := ffzTo7TV([]emoteBTTVFFZ{e})

		if err != nil {
			continue
		}

		result[i] = emote[0]
	}

	return result, nil
}

func GetGlobalEmotesFFZ(ctx context.Context) ([]*datastructure.Emote, error) {
	uri := fmt.Sprintf("%v/cached/frankerfacez/emotes/global", baseUrlBTTV)

	// Send request
	resp, err := cache.CacheGetRequest(ctx, uri, time.Hour*4, time.Minute*15)
	if err != nil {
		return nil, err
	}

	var emotes []emoteBTTVFFZ
	err = json.Unmarshal(resp.Body, &emotes)
	if err != nil {
		return nil, err
	}

	// Convert these emotes into a 7TV emote object
	result := make([]*datastructure.Emote, len(emotes))
	for i, e := range emotes {
		emote, err := ffzTo7TV([]emoteBTTVFFZ{e})

		if err != nil {
			continue
		}

		result[i] = emote[0]
	}

	return result, nil
}

// Convert a FFZ emote object into 7TV
func ffzTo7TV(emotes []emoteBTTVFFZ) ([]*datastructure.Emote, error) {
	result := make([]*datastructure.Emote, len(emotes))

	for i, emote := range emotes {
		if emote.User == nil { // Add empty user if missing
			emote.User = &userBTTVFFZ{}
		}
		visibility := int32(0)
		// Set zero-width flag if emote is a hardcoded bttv zerowidth
		if utils.Contains(zeroWidthBTTV, emote.Code) {
			visibility |= datastructure.EmoteVisibilityZeroWidth
		}

		// Generate URLs list
		urls := make([][]string, 3)
		for i, s := range []int8{1, 2, 4} {
			a := make([]string, 2)
			a[0] = fmt.Sprintf("%d", s)
			a[1] = getCdnURL_FFZ(emote.ID, int8(s))

			urls[i] = a
		}

		result[i] = &datastructure.Emote{
			Name:       emote.Code,
			Width:      [4]int16{28, 0, 0, 0},
			Height:     [4]int16{28, 0, 0, 0},
			Visibility: visibility,
			Mime:       "image/" + emote.ImageType,
			Status:     datastructure.EmoteStatusLive,
			Owner: &datastructure.User{
				DisplayName: emote.User.DisplayName,
				Login:       emote.User.Name,
				TwitchID:    emote.User.ProviderID,
			},

			Provider:   "FFZ",
			ProviderID: utils.StringPointer(strconv.Itoa(int(emote.ID))),
			URLs:       urls,
		}
	}

	return result, nil
}

func getCdnURL_FFZ(emoteID int32, size int8) string {
	return fmt.Sprintf("https://cdn.betterttv.net/frankerfacez_emote/%d/%d", emoteID, size)
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
