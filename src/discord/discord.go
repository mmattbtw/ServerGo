package discord

import (
	"fmt"
	"strconv"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/mongo"
	dgo "github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

// An empty Discord session for executing webhooks
var d, _ = dgo.New()
var webhookID *string
var webhookToken *string

func init() {
	s := configure.Config.GetStringSlice("discord.webhook")
	webhookID = &s[0]
	webhookToken = &s[1]
}

func SendEmoteCreate(emote mongo.Emote, actor mongo.User) (*dgo.Message, error) {
	if webhookID == nil || webhookToken == nil {
		return nil, fmt.Errorf("webhook is not configured")
	}

	msg, err := d.WebhookExecute(*webhookID, *webhookToken, true, &dgo.WebhookParams{
		Content: fmt.Sprintf("**[activity]** 🆕 emote [%s](%v) created by %s", emote.Name, getEmotePageURL(emote), actor.DisplayName),
		Embeds: []*dgo.MessageEmbed{
			{
				Title: emote.Name,
				Image: &dgo.MessageEmbedImage{
					URL: getEmoteImageURL(emote),
				},
				Color: toIntColor("24e575"),
			},
		},
	})
	if err != nil {
		log.Errorf("discord, SendEmoteCreate, err=%v", err)
		return nil, err
	}

	return msg, nil
}

func SendEmoteEdit(emote mongo.Emote, actor mongo.User, logs []*mongo.AuditLogChange, reason *string) (*dgo.Message, error) {
	if webhookID == nil || webhookToken == nil {
		return nil, fmt.Errorf("webhook is not configured")
	}

	fields := make([]*dgo.MessageEmbedField, len(logs))
	for i, ch := range logs {
		fields[i] = &dgo.MessageEmbedField{
			Name:   ch.Key,
			Value:  fmt.Sprintf("%s ➡️ %s", fmt.Sprint(ch.OldValue), fmt.Sprint(ch.NewValue)),
			Inline: true,
		}
	}
	if reason != nil && len(*reason) > 0 {
		fields = append(fields, &dgo.MessageEmbedField{
			Name:  "Reason",
			Value: *reason,
		})
	}

	msg, err := d.WebhookExecute(*webhookID, *webhookToken, true, &dgo.WebhookParams{
		Content: fmt.Sprintf("**[activity]** ✏️ emote [%s](%v) edited by [%s](%v)", emote.Name, getEmotePageURL(emote), actor.DisplayName, getUserPageURL(actor)),
		Embeds: []*dgo.MessageEmbed{
			{
				Title:       emote.Name,
				Description: fmt.Sprintf("by %v", actor.DisplayName),
				Thumbnail: &dgo.MessageEmbedThumbnail{
					URL: getEmoteImageURL(emote),
				},
				Fields: fields,
				Color:  toIntColor("e3b464"),
			},
		},
	})
	if err != nil {
		log.Errorf("discord, SendEmoteEdit, err=%v", err)
		return nil, err
	}

	return msg, nil
}

func SendEmoteDelete(emote mongo.Emote, actor mongo.User, reason string) (*dgo.Message, error) {
	if webhookID == nil || webhookToken == nil {
		return nil, fmt.Errorf("webhook is not configured")
	}

	msg, err := d.WebhookExecute(*webhookID, *webhookToken, true, &dgo.WebhookParams{
		Content: fmt.Sprintf("**[activity]** ❌ emote [%s](%v) deleted by [%s](%v)", emote.Name, getEmotePageURL(emote), actor.DisplayName, getUserPageURL(actor)),
		Embeds: []*dgo.MessageEmbed{
			{Description: fmt.Sprintf("Reason: %s", reason)},
		},
	})
	if err != nil {
		log.Errorf("discord, SendEmoteDelete, err=%v", err)
		return nil, err
	}

	return msg, nil
}

func getEmoteImageURL(emote mongo.Emote) string {
	return configure.Config.GetString("cdn_url") + fmt.Sprintf("/emote/%s/%dx", emote.ID.Hex(), 4)
}

func getEmotePageURL(emote mongo.Emote) string {
	return configure.Config.GetString("website_url") + fmt.Sprintf("/emotes/%s", emote.ID.Hex())
}

func getUserPageURL(user mongo.User) string {
	return configure.Config.GetString("website_url") + fmt.Sprintf("/users/%s", user.ID.Hex())
}

func toIntColor(s string) int {
	i, err := strconv.ParseInt(s, 16, 32)
	if err != nil {
		return 0
	}

	return int(i)
}
