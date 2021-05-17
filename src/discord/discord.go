package discord

import (
	"fmt"
	"strconv"

	"github.com/SevenTV/ServerGo/src/configure"
	dgo "github.com/bwmarrin/discordgo"
)

// An empty Discord session for executing webhooks
var d, _ = dgo.New(fmt.Sprintf("Bot %v", configure.Config.GetString("discord.bot_token")))
var webhookID *string
var webhookToken *string

func init() {
	s := configure.Config.GetStringSlice("discord.webhook")
	if len(s) == 2 {
		webhookID = &s[0]
		webhookToken = &s[1]
	}
}

func toIntColor(s string) int {
	i, err := strconv.ParseInt(s, 16, 32)
	if err != nil {
		return 0
	}

	return int(i)
}

var Discord = d
