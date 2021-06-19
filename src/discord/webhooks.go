package discord

import (
	"context"
	"fmt"
	"sync"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/utils"
	dgo "github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
)

func SendEmoteCreate(emote datastructure.Emote, actor datastructure.User) {
	if webhookID == nil || webhookToken == nil {
		return
	}

	_, err := d.WebhookExecute(*webhookID, *webhookToken, true, &dgo.WebhookParams{
		Content: fmt.Sprintf("**[activity]** 🆕 emote [%s](%v) created by %s", emote.Name, utils.GetEmotePageURL(emote.ID.Hex()), actor.DisplayName),
		Embeds: []*dgo.MessageEmbed{
			{
				Title: emote.Name,
				Image: &dgo.MessageEmbedImage{
					URL: utils.GetEmoteImageURL(emote.ID.Hex()),
				},
				Color: toIntColor("24e575"),
			},
		},
	})
	if err != nil {
		log.Errorf("discord, SendEmoteCreate, err=%v", err)
		return
	}
}

func SendEmoteEdit(emote datastructure.Emote, actor datastructure.User, logs []*datastructure.AuditLogChange, reason *string) {
	if webhookID == nil || webhookToken == nil {
		return
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

	_ = cache.FindOne(context.Background(), "users", "", bson.M{
		"_id": emote.OwnerID,
	}, &emote.Owner)

	ownerName := datastructure.DeletedUser.DisplayName
	if emote.Owner != nil {
		ownerName = emote.Owner.DisplayName
	}
	_, err := d.WebhookExecute(*webhookID, *webhookToken, true, &dgo.WebhookParams{
		Content: fmt.Sprintf("**[activity]** ✏️ emote [%s](%v) edited by [%s](%v)", emote.Name, utils.GetEmotePageURL(emote.ID.Hex()), actor.DisplayName, utils.GetUserPageURL(actor.ID.Hex())),
		Embeds: []*dgo.MessageEmbed{
			{
				Title:       emote.Name,
				Description: fmt.Sprintf("by %v", ownerName),
				Thumbnail: &dgo.MessageEmbedThumbnail{
					URL: utils.GetEmoteImageURL(emote.ID.Hex()),
				},
				Fields: fields,
				Color:  toIntColor("e3b464"),
			},
		},
	})
	if err != nil {
		log.Errorf("discord, SendEmoteEdit, err=%v", err)
		return
	}
}

func SendEmoteDelete(emote datastructure.Emote, actor datastructure.User, reason string) {
	if webhookID == nil || webhookToken == nil {
		return
	}

	_, err := d.WebhookExecute(*webhookID, *webhookToken, true, &dgo.WebhookParams{
		Content: fmt.Sprintf("**[activity]** ❌ emote [%s](%v) deleted by [%s](%v)", emote.Name, utils.GetEmotePageURL(emote.ID.Hex()), actor.DisplayName, utils.GetUserPageURL(actor.ID.Hex())),
		Embeds: []*dgo.MessageEmbed{
			{Description: fmt.Sprintf("Reason: %s", reason)},
		},
	})
	if err != nil {
		log.Errorf("discord, SendEmoteDelete, err=%v", err)
		return
	}
}

func SendPopularityCheckUpdateNotice(wg *sync.WaitGroup) {
	_, err := d.WebhookExecute(*webhookID, *webhookToken, true, &dgo.WebhookParams{
		Content: "**[routine]** ⚙️ updating emote popularities...",
	})
	if err != nil {
		log.WithError(err).Error("task failed")
	}

	wg.Wait()

	_, err = d.WebhookExecute(*webhookID, *webhookToken, true, &dgo.WebhookParams{
		Content: "**[routine]** ⚙️ successfully updated emote popularities!",
	})
	if err != nil {
		log.WithError(err).Error("task failed")
	}
}
