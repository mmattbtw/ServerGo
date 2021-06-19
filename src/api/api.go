package api

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/SevenTV/ServerGo/src/auth"
	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/pasztorpisti/qs"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type TwitchUserResp struct {
	Data []TwitchUser `json:"data"`
}

type TwitchUser struct {
	ID              string    `json:"id" bson:"id"`
	Login           string    `json:"login" bson:"login"`
	DisplayName     string    `json:"display_name" bson:"display_name"`
	BroadcasterType string    `json:"broadcaster_type" bson:"broadcaster_type"`
	Description     string    `json:"description" bson:"description"`
	ProfileImageURL string    `json:"profile_image_url" bson:"profile_image_url"`
	OfflineImageURL string    `json:"offline_image_url" bson:"offline_image_url"`
	ViewCount       int       `json:"view_count" bson:"view_count"`
	Email           string    `json:"email" bson:"email"`
	CreatedAt       time.Time `json:"created_at" bson:"twitch_created_at"`
}

func GetUsers(ctx context.Context, oauth string, ids []string, logins []string) ([]TwitchUser, error) {
	returnv := []TwitchUser{}
	for len(ids) != 0 || len(logins) != 0 {
		var temp []string
		var temp2 []string
		if len(ids) > 100 {
			temp = ids[:100]
			ids = ids[100:]
		} else {
			temp = ids
			ids = []string{}
			if len(logins)+len(temp) > 100 {
				temp2 = logins[:100-len(temp)]
				logins = logins[100-len(temp):]
			} else {
				temp2 = logins
				logins = []string{}
			}
		}

		params, _ := qs.Marshal(map[string][]string{
			"id":    temp,
			"login": temp2,
		})

		var token string
		var err error

		if oauth == "" {
			token, err = auth.GetAuth(ctx)
			if err != nil {
				return nil, err
			}
		} else {
			token = oauth
		}

		req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://api.twitch.tv/helix/users?%s", params), nil)
		if err != nil {
			return nil, err
		}

		req.Header.Add("Client-Id", configure.Config.GetString("twitch_client_id"))
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

		if err != nil {
			return nil, err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()

		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		respData := TwitchUserResp{}

		if err := json.Unmarshal(data, &respData); err != nil {
			return nil, err
		}
		returnv = append(returnv, respData.Data...)
	}

	if oauth != "" && len(ids) == 0 && len(logins) == 0 {
		req, err := http.NewRequestWithContext(ctx, "GET", "https://api.twitch.tv/helix/users", nil)
		if err != nil {
			return nil, err
		}

		req.Header.Add("Client-Id", configure.Config.GetString("twitch_client_id"))
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", oauth))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()

		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		respData := TwitchUserResp{}

		if err := json.Unmarshal(data, &respData); err != nil {
			return nil, err
		}
		return respData.Data, nil
	}

	return returnv, nil
}
