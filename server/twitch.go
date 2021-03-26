package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/SevenTV/ServerGo/api"
	"github.com/SevenTV/ServerGo/jwt"
	"github.com/SevenTV/ServerGo/mongo"
	"github.com/SevenTV/ServerGo/session"
	"github.com/SevenTV/ServerGo/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/SevenTV/ServerGo/configure"
	"github.com/gofiber/fiber/v2"

	"github.com/pasztorpisti/qs"

	log "github.com/sirupsen/logrus"
)

type TwitchTokenResp struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	Scope        []string `json:"scope"`
	TokenType    string   `json:"token_type"`
}

type TwitchCallback struct {
	Challenge    string                     `json:"challenge"`
	Subscription TwitchCallbackSubscription `json:"subscription"`
	Event        map[string]interface{}     `json:"event"`
}

type TwitchCallbackSubscription struct {
	ID        string                  `json:"id"`
	Status    string                  `json:"status"`
	Type      string                  `json:"type"`
	Version   string                  `json:"version"`
	Condition map[string]interface{}  `json:"condition"`
	Transport TwitchCallbackTransport `json:"transport"`
}

type TwitchCallbackTransport struct {
	Method   string `json:"method"`
	Callback string `json:"callback"`
	Secret   string `json:"secret"`
}

func Twitch(app fiber.Router) fiber.Router {
	twitch := app.Group("/twitch")

	twitch.Get("/login", func(c *fiber.Ctx) error {
		sessionStore, err := session.Store.Get(c)

		if err != nil {
			log.Errorf("session, err=%v", err)
			return c.Status(500).JSON(&fiber.Map{
				"message": "Internal server error.",
				"status":  500,
			})
		}
		defer func() {
			if err := sessionStore.Save(); err != nil {
				log.Errorf("session, err=%v", err)
			}
		}()

		csrfToken, err := utils.GenerateRandomString(64)
		if err != nil {
			log.Errorf("secure bytes, err=%v", err)
			return c.Status(500).JSON(&fiber.Map{
				"message": "Internal server error.",
				"status":  500,
			})
		}

		scopes := []string{}

		scopes = append(scopes, "user:read:email")

		sessionStore.Set("csrf_token", csrfToken)

		params, _ := qs.Marshal(map[string]string{
			"client_id":     configure.Config.GetString("twitch_client_id"),
			"redirect_uri":  configure.Config.GetString("twitch_redirect_uri"),
			"response_type": "code",
			"scope":         strings.Join(scopes, " "),
			"state":         csrfToken,
		})

		u := fmt.Sprintf("https://id.twitch.tv/oauth2/authorize?%s", params)

		return c.Redirect(u)
	})

	twitch.Get("/login/callback", func(c *fiber.Ctx) error {
		sessionStore, err := session.Store.Get(c)
		if err != nil {
			log.Errorf("session, err=%v", err)
			return c.Status(500).JSON(&fiber.Map{
				"message": "Internal server error.",
				"status":  500,
			})
		}
		defer func() {
			err := sessionStore.Save()
			if err != nil {
				log.Errorf("session, err=%v", err)
			}
		}()

		twitchToken := c.Query("state")

		if twitchToken == "" {
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, missing state paramater.",
			})
		}

		sessionToken, ok := sessionStore.Get("csrf_token").(string)
		if !ok {
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from sessiom store.",
			})
		}

		if sessionToken == "" {
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from sessiom store.",
			})
		}

		if twitchToken != sessionToken {
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, csrf_token token missmatch.",
			})
		}

		if err != nil {
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid return url.",
			})
		}

		sessionStore.Delete("csrf_token")

		code := c.Query("code")

		params, _ := qs.Marshal(map[string]string{
			"client_id":     configure.Config.GetString("twitch_client_id"),
			"client_secret": configure.Config.GetString("twitch_client_secret"),
			"redirect_uri":  configure.Config.GetString("twitch_redirect_uri"),
			"code":          code,
			"grant_type":    "authorization_code",
		})

		u, _ := url.Parse(fmt.Sprintf("https://id.twitch.tv/oauth2/token?%s", params))

		resp, err := http.DefaultClient.Do(&http.Request{
			Method: "POST",
			URL:    u,
		})

		if err != nil {
			log.Errorf("twitch, err=%v", err)
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, failed to convert code to access token.",
			})
		}

		defer resp.Body.Close()

		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Errorf("ioutils, err=%v", err)
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, failed to convert code to access token.",
			})
		}

		tokenResp := TwitchTokenResp{}

		if err := json.Unmarshal(data, &tokenResp); err != nil {
			log.Errorf("twitch, err=%v, data=%s, url=%s", err, data, u)
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, failed to convert code to access token.",
			})
		}

		users, err := api.GetUsers(tokenResp.AccessToken, nil, nil)
		if err != nil || len(users) != 1 {
			log.Errorf("twitch, err=%v, resp=%v, token=%v", err, users, tokenResp)
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, failed to convert access token to user account.",
			})
		}

		user := users[0]

		findOne := mongo.Database.Collection("users").FindOne(mongo.Ctx, bson.M{
			"id": user.ID,
		})

		var mongoUser *mongo.User
		err = findOne.Err()
		if err == mongo.ErrNoDocuments {
			mongoUser = &mongo.User{
				TwitchID:        user.ID,
				DisplayName:     user.DisplayName,
				Login:           user.Login,
				ProfileImageURL: user.ProfileImageURL,
				Email:           user.Email,
				Rank:            mongo.UserRankDefault,
				EmoteIDs:        []primitive.ObjectID{},
				EditorIDs:       []primitive.ObjectID{},
			}
			res, err := mongo.Database.Collection("users").InsertOne(mongo.Ctx, mongoUser)
			if err != nil {
				log.Errorf("mongo, err=%v", err)
				return c.Status(500).JSON(&fiber.Map{
					"status":  500,
					"message": "Failed to create new account.",
				})
			}
			mongoUser.ID, ok = res.InsertedID.(primitive.ObjectID)
			if !ok {
				log.Errorf("mongo, v=%v", res)
				return c.Status(500).JSON(&fiber.Map{
					"status":  500,
					"message": "Failed to read account.",
				})
			}
		} else if err == nil {
			mongoUser = &mongo.User{}
			if err := findOne.Decode(mongoUser); err != nil {
				log.Errorf("mongo, err=%v", err)
				return c.Status(500).JSON(&fiber.Map{
					"status":  500,
					"message": "Failed to read account.",
				})
			}
		} else {
			log.Errorf("mongo, err=%v", err)
			return c.Status(500).JSON(&fiber.Map{
				"status":  500,
				"message": "Failed to fetch account.",
			})
		}

		pl := &jwt.Payload{
			ID:   mongoUser.ID.Hex(),
			TWID: mongoUser.TwitchID,
		}

		token, err := jwt.Sign(pl)
		if err != nil {
			log.Errorf("jwt, err=%v", err)
			return c.Status(500).JSON(&fiber.Map{
				"status":  500,
				"message": "Failed to create user auth.",
			})
		}

		c.Cookie(&fiber.Cookie{
			Name:     "auth",
			Value:    token,
			Domain:   configure.Config.GetString("cookie_domain"),
			HTTPOnly: false,
			Secure:   configure.Config.GetBool("cookie_secure"),
			SameSite: "LAX",
			Expires:  time.Now().Add(time.Hour * 24 * 14),
		})

		return c.SendStatus(200)
	})

	return twitch
}
