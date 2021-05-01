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

	emotes, err := getEmotesUsage()
	if err != nil {
		fmt.Println("You're shit", err)
	} else {
		for _, emote := range emotes {
			fmt.Println("", *emote.ChannelCount, emote.Name)
			/* Spam the fuck out of general poggers
			if _, err := d.ChannelMessageSend("817075418640678964", emote.Name); err != nil {
				fmt.Println("You're more shit", err)
			}
			*/
		}
	}
}

func toIntColor(s string) int {
	i, err := strconv.ParseInt(s, 16, 32)
	if err != nil {
		return 0
	}

	return int(i)
}
