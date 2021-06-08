package discord

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/SevenTV/discordgo"
	dgo "github.com/SevenTV/discordgo"
	"github.com/bsm/redislock"
	log "github.com/sirupsen/logrus"
)

// An empty Discord session for executing webhooks
var ctx context.Context
var d, _ = dgo.New(fmt.Sprintf("Bot %v", configure.Config.GetString("discord.bot_token")))
var systemGuildID string = configure.Config.GetString("discord.guild_id")
var webhookID *string
var webhookToken *string

func init() {
	s := configure.Config.GetStringSlice("discord.webhook")
	if len(s) == 2 {
		webhookID = &s[0]
		webhookToken = &s[1]
	}

	ctx = context.Background()
	StartSession()
}

func StartSession() error {
	lock, _ := redis.GetLocker().Obtain(ctx, "lock:discordsession", time.Hour*1, &redislock.Options{})
	ctx = context.WithValue(ctx, "rlock", lock)
	defer func() {
		lock.Release(ctx)
	}()

	// Restore session?
	sid := redis.Client.Get(ctx, sessionKey).String()
	if sid != "" {
		d.State.SessionID = sid
	}

	// Add ready handler
	d.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		fmt.Println("Ready:", r)
		log.Info("discord, ready with %d guilds", len(r.Guilds))

		// Store SessionID in Redis
		redis.Client.Set(ctx, sessionKey, r.SessionID, time.Minute*6)

		// Register slash commands.
		RegisterCommands()
	})

	log.Info("discord, opening session")
	d.Open()
	return nil
}

func Close(asError bool) error {
	log.Info("discord, closing session")

	d.CloseWithCode(utils.Ternary(asError, 1006, 1000).(int))
	if asError == true {
		if _, err := redis.Client.Del(ctx, sessionKey, seqKey).Result(); err != nil {

		}
	}

	return nil
}

func toIntColor(s string) int {
	i, err := strconv.ParseInt(s, 16, 32)
	if err != nil {
		return 0
	}

	return int(i)
}

var Discord = d

var sessionKey = "discord:sessionid"
var seqKey = "discord:seq"
