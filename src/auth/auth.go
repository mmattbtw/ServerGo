package auth

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/utils"

	jsoniter "github.com/json-iterator/go"
	"github.com/pasztorpisti/qs"
	log "github.com/sirupsen/logrus"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var mutex = &sync.Mutex{}

var auth string

type AuthResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

var ErrInvalidRespTwitch = fmt.Errorf("invalid resp from twitch")

func GetAuth(ctx context.Context) (string, error) {
	mutex.Lock()
	defer mutex.Unlock()
	if auth != "" {
		return auth, nil
	}

	val, err := redis.Client.Get(ctx, "twitch:auth").Result()
	if err != nil && err != redis.ErrNil {
		return "", err
	}
	if val != "" {
		return val, nil
	}
	params, _ := qs.Marshal(map[string]string{
		"client_id":     configure.Config.GetString("twitch_client_id"),
		"client_secret": configure.Config.GetString("twitch_client_secret"),
		"grant_type":    "client_credentials",
	})
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("https://id.twitch.tv/oauth2/token?%s", params), nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode > 200 {
		log.WithField("resp", utils.B2S(data)).Error("twitch")
		return "", ErrInvalidRespTwitch
	}

	resData := AuthResp{}
	if err := json.Unmarshal(data, &resData); err != nil {
		return "", err
	}

	auth = resData.AccessToken

	expiry := time.Second * time.Duration(int64(float64(resData.ExpiresIn)*0.75))

	if err := redis.Client.SetNX(ctx, "twitch:auth", auth, expiry).Err(); err != nil {
		log.WithError(err).Error("redis")
	}

	go func() {
		if int64(float64(resData.ExpiresIn)*0.75) < 3600 {
			time.Sleep(time.Second * time.Duration(int64(float64(resData.ExpiresIn)*0.75)))
		} else {
			time.Sleep(time.Second * time.Duration(3600))
		}
		mutex.Lock()
		defer mutex.Unlock()
		auth = ""
	}()
	return auth, nil
}
