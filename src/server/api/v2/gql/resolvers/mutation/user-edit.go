package mutation_resolvers

import (
	"context"
	"fmt"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	query_resolvers "github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers/query"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (*MutationResolver) EditUser(ctx context.Context, args struct {
	User   userInput
	Reason *string
}) (*query_resolvers.UserResolver, error) {
	if configure.Config.GetBool("maintenance_mode") {
		return nil, fmt.Errorf("Maintenance Mode")
	}
	// Get the actor user
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	// Get the target user
	targetID, err := primitive.ObjectIDFromHex(args.User.ID)
	if err != nil {
		return nil, err
	}
	var target *datastructure.User
	if err := mongo.Collection(mongo.CollectionNameUsers).FindOne(ctx, bson.M{"_id": targetID}).Decode(&target); err != nil {
		return nil, err
	}
	// Get target's role
	targetRole := datastructure.GetRole(target.RoleID)
	target.Role = &targetRole

	if !usr.HasPermission(datastructure.RolePermissionManageUsers) {
		if usr.ID != target.ID {
			return nil, resolvers.ErrAccessDenied
		}
	}

	update := bson.M{}
	logChanges := []*datastructure.AuditLogChange{}
	notifications := []actions.NotificationBuilder{}
	req := args.User

	// Update: Role
	if req.RoleID != nil {
		// Check actor can manage roles
		if !usr.HasPermission(datastructure.RolePermissionManageRoles) {
			return nil, resolvers.ErrAccessDenied
		}

		// Check actor user's permission to edit target's role
		if usr.Role.Position <= target.Role.Position {
			return nil, resolvers.ErrAccessDenied
		}

		// Role is empty?
		if *req.RoleID == "" {
			update["role"] = nil
			logChanges = append(logChanges, &datastructure.AuditLogChange{
				Key:      "role",
				OldValue: target.RoleID,
				NewValue: nil,
			})
		} else {
			// Find role with ID
			roleID, err := primitive.ObjectIDFromHex(*req.RoleID)
			if err != nil {
				return nil, err
			}
			role := datastructure.GetRole(&roleID)
			if role.Default {
				return nil, resolvers.ErrUnknownRole
			}
			// Make sure the target role isn't higher than the actor's role
			if role.Position >= usr.Role.Position {
				return nil, resolvers.ErrAccessDenied
			}

			// Update role
			update["role"] = role.ID
			logChanges = append(logChanges, &datastructure.AuditLogChange{
				Key:      "role",
				OldValue: target.RoleID,
				NewValue: role.ID,
			})
			notifications = append(notifications, actions.Notifications.Create().
				SetTitle("Role Changed").
				AddTargetUsers(targetID).
				AddTextMessagePart("Your global role was changed from ").
				AddTextMessagePart(datastructure.GetRole(target.RoleID).Name).
				AddTextMessagePart(" to ").
				AddTextMessagePart(utils.Ternary(role.Default, "none", role.Name).(string)).
				AddTextMessagePart(" by ").
				AddUserMentionPart(usr.ID),
			)
		}
	}

	// Update: Channel Emote Slots
	if req.EmoteSlots != nil {
		slots := *req.EmoteSlots

		// If amount of slots requested is higher than the configured default:
		// Check actor can manage users
		if slots > configure.Config.GetInt32("limits.meta.channel_emote_slots") {
			if !usr.HasPermission(datastructure.RolePermissionManageUsers) {
				return nil, resolvers.ErrAccessDenied
			}
		}

		update["emote_slots"] = slots
		logChanges = append(logChanges, &datastructure.AuditLogChange{
			Key:      "emote_slots",
			OldValue: target.EmoteSlots,
			NewValue: slots,
		})
		notifications = append(notifications, actions.Notifications.Create().
			SetTitle("Maximum Channel Emote Slots Changed").
			AddTargetUsers(targetID).
			AddTextMessagePart("Your channel emote slots ").
			AddTextMessagePart(utils.Ternary(target.EmoteSlots > slots, "were reduced", "rose to").(string)).
			AddTextMessagePart(fmt.Sprintf(" from %d to %d", target.EmoteSlots, slots)).
			AddTextMessagePart(" by ").
			AddUserMentionPart(usr.ID),
		)
	}

	if req.CosmeticPaint != nil {
		paintID := primitive.NilObjectID
		if primitive.IsValidObjectID(*req.CosmeticPaint) {
			paintID, _ = primitive.ObjectIDFromHex(*req.CosmeticPaint)
		}

		// Set the user's paint
		if !paintID.IsZero() {
			res, err := mongo.Collection(mongo.CollectionNameEntitlements).UpdateOne(ctx, bson.M{
				"kind":     "PAINT",
				"data.ref": paintID,
				"user_id":  targetID,
			}, bson.M{"$set": bson.M{"data.selected": true}})
			if err == mongo.ErrNoDocuments || res.ModifiedCount == 0 {
				return nil, fmt.Errorf("you do not own this paint")
			} else if err != nil {
				logrus.WithError(err).Error("mongo, failed to select entitlement")
			}
		}

		// Disable other paints
		if _, err = mongo.Collection(mongo.CollectionNameEntitlements).UpdateMany(ctx, bson.M{
			"kind":     "PAINT",
			"data.ref": bson.M{"$not": bson.M{"$eq": paintID}},
			"user_id":  targetID,
		}, bson.M{"$set": bson.M{"data.selected": false}}); err != nil {
			logrus.WithError(err).Error("mongo, failed to update other entitlements")
			return nil, err
		}
	}

	if req.CosmeticBadge != nil {
		badgeID := primitive.NilObjectID
		if primitive.IsValidObjectID(*req.CosmeticBadge) {
			badgeID, _ = primitive.ObjectIDFromHex(*req.CosmeticBadge)
		}

		// Set the user's badge
		if !badgeID.IsZero() {
			res, err := mongo.Collection(mongo.CollectionNameEntitlements).UpdateOne(ctx, bson.M{
				"kind":     "BADGE",
				"data.ref": badgeID,
				"user_id":  targetID,
			}, bson.M{"$set": bson.M{"data.selected": true}})
			if err == mongo.ErrNoDocuments || res.ModifiedCount == 0 {
				return nil, fmt.Errorf("you do not own this badge")
			} else if err != nil {
				logrus.WithError(err).Error("mongo, failed to select entitlement")
			}
		}

		// Disable other badges
		if _, err = mongo.Collection(mongo.CollectionNameEntitlements).UpdateMany(ctx, bson.M{
			"kind":     "BADGE",
			"data.ref": bson.M{"$not": bson.M{"$eq": badgeID}},
			"user_id":  targetID,
		}, bson.M{"$set": bson.M{"data.selected": false}}); err != nil {
			logrus.WithError(err).Error("mongo, failed to update other entitlements")
			return nil, err
		}
	}

	field, failed := query_resolvers.GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	var user *datastructure.User
	if len(logChanges) > 0 {
		after := options.After
		doc := mongo.Collection(mongo.CollectionNameUsers).FindOneAndUpdate(ctx, bson.M{
			"_id": targetID,
		}, bson.M{
			"$set": update,
		}, &options.FindOneAndUpdateOptions{
			ReturnDocument: &after,
		})
		if err := doc.Decode(&user); err != nil {
			return nil, resolvers.ErrInternalServer
		}

		_, err := mongo.Collection(mongo.CollectionNameAudit).InsertOne(ctx, &datastructure.AuditLog{
			Type:      datastructure.AuditLogTypeUserEdit,
			CreatedBy: usr.ID,
			Target:    &datastructure.Target{ID: &targetID, Type: "users"},
			Changes:   logChanges,
			Reason:    args.Reason,
		})
		if err != nil {
			logrus.WithError(err).Error("mongo")
		}

		// Send notifications
		go func() {
			for _, n := range notifications {
				if err := n.Write(ctx); err != nil {
					logrus.WithError(err).Error("failed to create notification")
				}
			}
		}()

		return query_resolvers.GenerateUserResolver(ctx, user, &targetID, field.Children)
	}

	return query_resolvers.GenerateUserResolver(ctx, target, &targetID, field.Children)
}
