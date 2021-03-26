package api

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/SevenTV/ServerGo/auth"
	"github.com/SevenTV/ServerGo/configure"
	"github.com/pasztorpisti/qs"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type TwitchUserResp struct {
	Data []TwitchUser `json:"data"`
}

type TwitchUser struct {
	ID              string    `json:"id"`
	Login           string    `json:"login"`
	DisplayName     string    `json:"display_name"`
	BroadcasterType string    `json:"broadcaster_type"`
	Description     string    `json:"description"`
	ProfileImageURL string    `json:"profile_image_url"`
	OfflineImageURL string    `json:"offline_image_url"`
	ViewCount       int       `json:"view_count"`
	Email           string    `json:"email"`
	CreatedAt       time.Time `json:"created_at"`
}

func GetUsers(oauth string, ids []string, logins []string) ([]TwitchUser, error) {
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

		u, _ := url.Parse(fmt.Sprintf("https://api.twitch.tv/helix/users?%s", params))

		var token string
		var err error

		if oauth == "" {
			token, err = auth.GetAuth()
			if err != nil {
				return nil, err
			}
		} else {
			token = oauth
		}

		resp, err := http.DefaultClient.Do(&http.Request{
			Method: "GET",
			URL:    u,
			Header: http.Header{
				"Client-Id":     []string{configure.Config.GetString("twitch_client_id")},
				"Authorization": []string{fmt.Sprintf("Bearer %s", token)},
			},
		})
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
		u, _ := url.Parse("https://api.twitch.tv/helix/users")

		var err error

		resp, err := http.DefaultClient.Do(&http.Request{
			Method: "GET",
			URL:    u,
			Header: http.Header{
				"Client-Id":     []string{configure.Config.GetString("twitch_client_id")},
				"Authorization": []string{fmt.Sprintf("Bearer %s", oauth)},
			},
		})
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
