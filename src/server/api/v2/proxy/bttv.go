package api_proxy

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/utils"
)

const baseUrlBTTV = "https://api.betterttv.net/3"

func GetGlobalEmotesBTTV() ([]*mongo.Emote, error) {
	// Set Request URI
	uri := fmt.Sprintf("%v/cached/emotes/global", baseUrlBTTV)

	// Get global bttv emotes
	resp, err := cache.CacheGetRequest(uri, time.Hour*4, time.Minute*15) // This request is cached for 4 hours as global emotes rarely change
	if err != nil {
		return nil, err
	}

	// Decode response into json
	var emotes []emoteBTTV
	err = json.Unmarshal(resp.Body, &emotes)
	if err != nil {
		return nil, err
	}

	// Convert these bttv emotes into a 7TV emote object
	result := make([]*mongo.Emote, len(emotes))
	for i, e := range emotes {
		emote, err := bttvTo7TV([]emoteBTTV{e})
		if err != nil {
			continue
		}

		result[i] = emote[0]
	}

	return result, nil
}

func GetChannelEmotesBTTV(login string) ([]*mongo.Emote, error) {
	// Get Twitch User from ID
	usr, err := GetTwitchUser(login)
	if err != nil {
		return nil, err
	}

	// Set Requesst URI
	uri := fmt.Sprintf("%v/cached/users/twitch/%v", baseUrlBTTV, usr.ID)

	// Get bttv user response
	resp, err := cache.CacheGetRequest(uri, time.Minute*5, time.Minute*15)
	if err != nil {
		return nil, err
	}

	// Decode response into json
	var userResponse userResponseBTTV
	err = json.Unmarshal(resp.Body, &userResponse)
	if err != nil {
		return nil, err
	}

	// Add these emotes to the final result
	// Merging "channel" and "shared" emotes, as 7TV sees no distinction.
	result := make([]*mongo.Emote, len(userResponse.Emotes)+len(userResponse.SharedEmotes))

	// Add user data to non-shared emotes
	for i := range userResponse.Emotes {
		userResponse.Emotes[i].User = &userBTTV{
			ID:          usr.ID,
			Name:        usr.Login,
			DisplayName: usr.DisplayName,
			ProviderID:  usr.ID,
		}
	}

	// Convert emotes to 7TV
	channel, _ := bttvTo7TV(userResponse.Emotes)
	shared, _ := bttvTo7TV(userResponse.SharedEmotes)

	copy(result, channel)
	for i, e := range shared {
		result[i+len(channel)] = e
	}

	return result, nil
}

// Convert a BTTV emote object into 7TV
func bttvTo7TV(emotes []emoteBTTV) ([]*mongo.Emote, error) {
	result := make([]*mongo.Emote, len(emotes))

	for i, emote := range emotes {
		if emote.User == nil { // Add empty user if missing
			emote.User = &userBTTV{}
		}

		// Generate URLs list
		urls := make([]*[]*string, 3)
		for i := 1; i <= 3; i++ {
			a := make([]*string, 2)
			a[0] = utils.StringPointer(fmt.Sprintf("%d", i))
			a[1] = utils.StringPointer(getCdnURL_BTTV(emote.ID, int8(i)))

			urls[i-1] = &a
		}

		result[i] = &mongo.Emote{
			Name:       emote.Code,
			Visibility: 0,
			Mime:       "image/" + emote.ImageType,
			Status:     mongo.EmoteStatusLive,
			Owner: &mongo.User{
				DisplayName: emote.User.DisplayName,
				Login:       emote.User.Name,
				TwitchID:    emote.User.ProviderID,
			},

			Provider:   "BTTV",
			ProviderID: utils.StringPointer(emote.ID),
			URLs:       &urls,
		}
	}

	return result, nil
}

func getCdnURL_BTTV(emoteID string, size int8) string {
	return fmt.Sprintf("https://cdn.betterttv.net/emote/%v/%dx", emoteID, size)
}

type emoteBTTV struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	ImageType string    `json:"imageType"`
	User      *userBTTV `json:"user"`
	UserID    *string   `json:"userId"`
}

type userBTTV struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	ProviderID  string `json:"providerId"`
}

type userResponseBTTV struct {
	ID           string      `json:"id"`
	Emotes       []emoteBTTV `json:"channelEmotes"`
	SharedEmotes []emoteBTTV `json:"sharedEmotes"`
}
