package mutation_resolvers

import (
	"context"
	"fmt"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/discord"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//
// REPORT EMOTE
//
func (*MutationResolver) ReportEmote(ctx context.Context, args struct {
	EmoteID string
	Reason  string
}) (*response, error) {
	if configure.Config.GetBool("maintenance_mode") {
		return nil, fmt.Errorf("Maintenance Mode")
	}
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	id, err := primitive.ObjectIDFromHex(args.EmoteID)
	if err != nil {
		return nil, resolvers.ErrUnknownEmote
	}

	res := mongo.Collection(mongo.CollectionNameEmotes).FindOne(ctx, bson.M{
		"_id":    id,
		"status": datastructure.EmoteStatusLive,
	})

	emote := &datastructure.Emote{}
	reason := args.Reason
	if len(reason) < 6 {
		return nil, fmt.Errorf("please write at least 6 characters")
	} else if len(reason) > 4000 {
		return nil, fmt.Errorf("your report is too long. it should be under 4,000 characters")
	}

	err = res.Err()

	if err == nil {
		err = res.Decode(emote)
	}
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, resolvers.ErrUnknownEmote
		}
		logrus.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}
	if emote.Owner == nil {
		emote.Owner = datastructure.DeletedUser
	}

	opts := options.Update().SetUpsert(true)

	_, err = mongo.Collection(mongo.CollectionNameReports).UpdateOne(ctx, bson.M{
		"target.id":   emote.ID,
		"target.type": "emotes",
		"cleared":     false,
		"reporter_id": usr.ID,
	}, bson.M{
		"$set": bson.M{
			"target.id":   emote.ID,
			"target.type": "emotes",
			"cleared":     false,
			"reporter_id": usr.ID,
			"reason":      args.Reason,
		},
	}, opts)

	if err != nil {
		return nil, resolvers.ErrInternalServer
	}

	_, err = mongo.Collection(mongo.CollectionNameAudit).InsertOne(ctx, &datastructure.AuditLog{
		Type:      datastructure.AuditLogTypeReport,
		CreatedBy: usr.ID,
		Target:    &datastructure.Target{ID: &id, Type: "emotes"},
		Changes:   nil,
		Reason:    &args.Reason,
	})
	if err != nil {
		logrus.WithError(err).Error("mongo")
	}

	// Post to Discord
	go func() {
		discord.SendWebhook("reports", &discordgo.WebhookParams{
			Content: fmt.Sprintf(
				"Report on [%s](%s) submitted by [%s](%s)",
				fmt.Sprintf("%s by [%s](%s)",
					emote.Name,
					emote.Owner.DisplayName,
					utils.GetUserPageURL(emote.Owner.ID.Hex()),
				),
				utils.GetEmotePageURL(emote.ID.Hex()),
				usr.Login,
				utils.GetUserPageURL(usr.ID.Hex()),
			),
			Embeds: []*discordgo.MessageEmbed{
				{
					URL:         utils.GetEmotePageURL(emote.ID.Hex()),
					Description: reason,
					Color:       5124776,
					Thumbnail: &discordgo.MessageEmbedThumbnail{
						URL: utils.GetEmoteImageURL(emote.ID.Hex()),
					},
				},
			},
		})
	}()

	return &response{
		OK:      true,
		Status:  200,
		Message: "success",
	}, nil
}

//
// REPORT USER
//
func (*MutationResolver) ReportUser(ctx context.Context, args struct {
	UserID string
	Reason *string
}) (*response, error) {
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	id, err := primitive.ObjectIDFromHex(args.UserID)
	if err != nil {
		return nil, resolvers.ErrUnknownUser
	}

	if id.Hex() == usr.ID.Hex() {
		return nil, resolvers.ErrYourself
	}

	banned, _ := actions.Bans.IsUserBanned(id)
	if banned {
		return nil, resolvers.ErrUserBanned
	}

	res := mongo.Database.Collection("user").FindOne(ctx, bson.M{
		"_id": id,
	})

	user := &datastructure.User{}

	err = res.Err()

	if err == nil {
		err = res.Decode(user)
	}

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, resolvers.ErrUnknownUser
		}
		logrus.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}

	opts := options.Update().SetUpsert(true)

	_, err = mongo.Collection(mongo.CollectionNameReports).UpdateOne(ctx, bson.M{
		"target.id":   user.ID,
		"target.type": "users",
		"cleared":     false,
		"reporter_id": usr.ID,
	}, bson.M{
		"$set": bson.M{
			"target.id":   user.ID,
			"target.type": "users",
			"cleared":     false,
			"reporter_id": usr.ID,
			"reason":      args.Reason,
		},
	}, opts)

	if err != nil {
		return nil, resolvers.ErrInternalServer
	}

	_, err = mongo.Collection(mongo.CollectionNameAudit).InsertOne(ctx, &datastructure.AuditLog{
		Type:      datastructure.AuditLogTypeReport,
		CreatedBy: usr.ID,
		Target:    &datastructure.Target{ID: &id, Type: "emotes"},
		Changes:   nil,
		Reason:    args.Reason,
	})
	if err != nil {
		logrus.WithError(err).Error("mongo")
	}

	return &response{
		OK:      true,
		Status:  200,
		Message: "success",
	}, nil
}
