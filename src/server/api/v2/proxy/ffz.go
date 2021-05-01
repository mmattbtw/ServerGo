package api_proxy

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/utils"
)

const baseUrlFFZ = "https://api.frankerfacez.com/v1"

// Get channel emotes from the FFZ provider
func GetChannelEmotesFFZ(login string) ([]*mongo.Emote, error) {
	// Set Request URI
	uri := fmt.Sprintf("%v/emotes?owner=%v&sensitive=%v", baseUrlFFZ, login, "true")

	// Send the request
	resp, err := cache.CacheGetRequest(uri, time.Minute*5, time.Minute*15)
	if err != nil {
		return nil, err
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

func GetGlobalEmotesFFZ() ([]*mongo.Emote, error) {
	uri := fmt.Sprintf("%v/set/3", baseUrlFFZ)

	// Send request
	resp, err := cache.CacheGetRequest(uri, time.Hour*4, time.Minute*15)
	if err != nil {
		return nil, err
	}

	var emoteResponse getEmoteSetResponseFFZ
	if err := json.Unmarshal(resp.Body, &emoteResponse); err != nil {
		return nil, err
	}

	emotes, err := ffzTo7TV(emoteResponse.Set.Emotes)
	if err != nil {
		return nil, err
	}

	return emotes, nil
}

// Convert a FFZ emote object into 7TV
func ffzTo7TV(emotes []emoteFFZ) ([]*mongo.Emote, error) {
	result := make([]*mongo.Emote, len(emotes))

	for i, emote := range emotes {
		// Generate URLs list
		urls := make([]*[]*string, 3)
		for i, s := range []int8{1, 2, 4} {
			a := make([]*string, 2)
			a[0] = utils.StringPointer(fmt.Sprintf("%dx", s))
			a[1] = utils.StringPointer(getCdnURL_FFZ(emote.ID, int8(s)))

			urls[i] = &a
		}

		result[i] = &mongo.Emote{
			Name:       emote.Name,
			Visibility: 0,
			Mime:       "image/png",
			Status:     mongo.EmoteStatusLive,
			Owner: &mongo.User{
				Login:       emote.Owner.Name,
				DisplayName: emote.Owner.DisplayName,
				TwitchID:    fmt.Sprint(emote.Owner.ID),
			},

			Provider:   "FFZ",
			ProviderID: utils.StringPointer(fmt.Sprint(emote.ID)),
			URLs:       &urls,
		}
	}

	return result, nil
}

func getCdnURL_FFZ(emoteID int32, size int8) string {
	return fmt.Sprintf("https://cdn.frankerfacez.com/emoticon/%d/%d", emoteID, size)
}

type emoteFFZ struct {
	ID          int32             `json:"id"`
	Name        string            `json:"name"`
	Public      bool              `json:"public"`
	Hidden      bool              `json:"hidden"`
	Owner       userFFZ           `json:"owner"`
	Status      int32             `json:"status"`
	UsageCount  int32             `json:"usage_count"`
	CreatedAt   time.Time         `json:"created_at"`
	Urls        map[string]string `json:"urls"`
	LastUpdated time.Time         `json:"last_updated"`
}

type userFFZ struct {
	ID          int32  `json:"_id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

type getEmotesResponseFFZ struct {
	Pages  int32      `json:"_pages"`
	Total  int32      `json:"_total"`
	Emotes []emoteFFZ `json:"emoticons"`
}

type getEmoteSetResponseFFZ struct {
	Set emoteSetFFZ `json:"set"`
}

type emoteSetFFZ struct {
	ID     int32      `json:"id"`
	Title  string     `json:"title"`
	Emotes []emoteFFZ `json:"emoticons"`
}
