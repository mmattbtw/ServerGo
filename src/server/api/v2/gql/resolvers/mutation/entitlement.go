package mutation_resolvers

import (
	"context"
	"fmt"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (*MutationResolver) DeleteEntitlement(ctx context.Context, args struct {
	ID string
}) (*response, error) {
	// Get actor reference
	actor, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	// Verify actor's permission
	if !actor.HasPermission(datastructure.RolePermissionManageEntitlements) {
		return nil, resolvers.ErrAccessDenied
	}

	// Parse ID to objectid
	eID, err := primitive.ObjectIDFromHex(args.ID)
	if err != nil {
		return nil, err
	}

	// Delete the entitlement
	if _, err = mongo.Collection(mongo.CollectionNameEntitlements).DeleteOne(ctx, bson.M{
		"_id": eID,
	}); err != nil {
		log.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}

	return &response{
		OK:      true,
		Status:  200,
		Message: "Entitlement Deleted",
	}, nil
}

func (*MutationResolver) CreateEntitlement(ctx context.Context, args struct {
	Kind     datastructure.EntitlementKind
	Data     entitlementCreateInput
	UserID   string
	Disabled *bool
}) (*response, error) {
	// Get actor reference
	actor, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	// Verify actor's permission
	if !actor.HasPermission(datastructure.RolePermissionManageEntitlements) {
		return nil, resolvers.ErrAccessDenied
	}

	// Parse entitled user ID
	userID, err := primitive.ObjectIDFromHex(args.UserID)
	if err != nil {
		return nil, err
	}

	// Create an entitlement builder and assign kind+user ID
	builder := actions.Entitlements.Create(ctx).
		SetKind(args.Kind).
		SetUserID(userID)

	// Initiate a new notification to be sent to the entitled user
	notify := actions.Notifications.Create().
		AddTargetUsers(userID)

	// Assign typed data based on kind
	var itemID primitive.ObjectID
	switch args.Kind {
	case datastructure.EntitlementKindBadge:
		if args.Data.Badge == nil {
			return nil, fmt.Errorf("missing badge data")
		}

		itemID, err = primitive.ObjectIDFromHex(args.Data.Badge.ID)
		if err != nil {
			return nil, err
		}
		// Check the item already being entitled to the user
		var badge datastructure.Badge
		exists := true
		if err = mongo.Collection(mongo.CollectionNameEntitlements).FindOne(ctx, bson.M{
			"kind":     "BADGE",
			"user_id":  userID,
			"data.ref": itemID,
		}).Decode(&badge); err == mongo.ErrNoDocuments {
			exists = false
		}

		if !exists {
			if err := mongo.Collection(mongo.CollectionNameBadges).FindOne(ctx, bson.M{"_id": itemID}).Decode(&badge); err != nil {
				log.WithError(err).Error("mongo")
				if err == mongo.ErrNoDocuments {
					return nil, fmt.Errorf("unknown badge")
				}

				return nil, err
			}

			var roleBindingID primitive.ObjectID
			if args.Data.Badge.RoleBindingID != nil && primitive.IsValidObjectID(*args.Data.Badge.RoleBindingID) {
				roleBindingID, _ = primitive.ObjectIDFromHex(*args.Data.Badge.RoleBindingID)
			}

			builder = builder.SetBadgeData(datastructure.EntitledBadge{
				ObjectReference: badge.ID,
				Selected:        args.Data.Badge.Selected,
				RoleBinding:     &roleBindingID,
			})

			notify = notify.SetTitle("Chat Badge Acquired").
				AddTextMessagePart(fmt.Sprintf("The badge \"%v\" has been added to your account", badge.Name))
		}
	case datastructure.EntitlementKindRole:
		if args.Data.Role == nil {
			return nil, fmt.Errorf("missing role data")
		}
		itemID, err = primitive.ObjectIDFromHex(args.Data.Role.ID)
		if err != nil {
			return nil, err
		}

		// Set Role Data to builder
		builder = builder.SetRoleData(datastructure.EntitledRole{
			ObjectReference: itemID,
		})

		role := datastructure.GetRole(&itemID)
		notify = notify.SetTitle("Global Role Granted").
			AddTextMessagePart("You've been granted the role").
			AddRoleMentionPart(role.ID)
	}

	if builder.Entitlement.Data != nil {
		// Write to DB
		if builder, err = builder.Write(); err != nil {
			log.WithError(err).Error(err)
			return nil, resolvers.ErrInternalServer
		}

		// Add the X-Created-ID header specifying the ID of the entitlement created
		f, ok := ctx.Value(utils.RequestCtxKey).(*fiber.Ctx) // Fiber context
		if ok {
			f.Set("X-Created-ID", builder.Entitlement.ID.Hex())
		}

		// Send the notification
		if len(notify.Notification.MessageParts) > 0 {
			go func() {
				if err := notify.Write(ctx); err != nil {
					log.WithError(err).Error("notifications")
				}
			}()
		}
	}

	return &response{
		OK:      true,
		Status:  200,
		Message: "Entitlement Created",
	}, nil
}
