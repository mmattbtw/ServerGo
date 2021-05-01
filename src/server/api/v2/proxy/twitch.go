package api_proxy

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/src/auth"
	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/configure"
)

const baseUrlTwitch = "https://api.twitch.tv"

func GetTwitchUser(login string) (*userTwitch, error) {
	// Set Request URI
	uri := fmt.Sprintf("%v/helix/users?login=%v", baseUrlTwitch, login)

	// Get auth
	token, err := auth.GetAuth()
	if err != nil {
		return nil, err
	}

	// Send request
	resp, err := cache.CacheGetRequest(uri, time.Minute*30, time.Minute*15, struct {
		Key   string
		Value string
	}{Key: "Client-ID", Value: configure.Config.GetString("twitch_client_id")}, struct {
		Key   string
		Value string
	}{Key: "Authorization", Value: fmt.Sprintf("Bearer %v", token)})
	if err != nil {
		return nil, err
	}

	// Decode
	var userResponse userResponseTwitch
	if err := json.Unmarshal(resp.Body, &userResponse); err != nil {
		return nil, err
	}

	return &userResponse.Data[0], nil
}

type userResponseTwitch struct {
	Data []userTwitch `json:"data"`
}

type userTwitch struct {
	ID              string    `json:"id"`
	Description     string    `json:"description"`
	CreatedAt       time.Time `json:"created_at"`
	DisplayName     string    `json:"display_name"`
	Logo            string    `json:"logo"`
	Login           string    `json:"login"`
	Type            string    `json:"type"`
	BroadcasterType string    `json:"broadcaster_type"`
}
