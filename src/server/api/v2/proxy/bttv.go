package api_proxy

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SevenTV/ServerGo/src/mongo"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const baseUrl = "https://api.betterttv.net/3"

func GetGlobalEmotesBTTV() ([]*mongo.Emote, error) {
	// Set Request URI
	uri := fmt.Sprintf("%v/cached/emotes/global", baseUrl)
	fmt.Println(uri)

	// Send request to BTTV
	resp, err := http.Get(uri)
	if err != nil {
		return nil, err
	}

	// Decode response into json
	var emotes []emoteBTTV
	err = json.NewDecoder(resp.Body).Decode(&emotes)
	if err != nil {
		return nil, err
	}

	result := make([]*mongo.Emote, len(emotes))
	for i, e := range emotes {
		if !primitive.IsValidObjectID(e.ID) {
			continue
		}
		id, _ := primitive.ObjectIDFromHex(e.ID)

		if e.User == nil {
			e.User = &userBTTV{}
		}
		result[i] = &mongo.Emote{
			ID:         id,
			Name:       e.Code,
			Visibility: 0,
			Mime:       "image/" + e.ImageType,
			Status:     mongo.EmoteStatusLive,
			Owner: &mongo.User{
				DisplayName: e.User.DisplayName,
				Login:       e.User.Name,
				TwitchID:    e.User.ProviderID,
			},
		}
	}

	return result, nil
}

func GetChannelEmotesBTTV(userID string) ([]*mongo.Emote, error) {
	// Set Requesst URI
	uri := fmt.Sprintf("%v/cached/users/twitch/%v", baseUrl, userID)

	// Send request to BTTV
	resp, err := http.Get(uri)
	if err != nil {
		return nil, err
	}

	var userResponse userResponseBTTV
	err = json.NewDecoder(resp.Body).Decode(&userResponse)
	if err != nil {
		return nil, err
	}

	// result := make([]*mongo.Emote, len(userResponse.Emotes) + len(userResponse.SharedEmotes))

	return nil, nil
}

type emoteBTTV struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	ImageType string    `json:"imageType"`
	User      *userBTTV `json:"user"`
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
