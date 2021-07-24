package mutation_resolvers

import (
	"context"
	"fmt"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	"github.com/SevenTV/ServerGo/src/utils"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

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
	builder := actions.Entitlements.Create().
		SetKind(args.Kind).
		SetUserID(userID)

	// Assign typed data based on kind
	var itemID primitive.ObjectID
	switch args.Kind {
	case datastructure.EntitlementKindSubscription:
		if args.Data.Subscription == nil {
			return nil, fmt.Errorf("Missing Subscription Data")
		}
		itemID, err = primitive.ObjectIDFromHex(args.Data.Subscription.ID)
		if err != nil {
			return nil, err
		}

		// Set Subscription Data to builder
		builder.SetSubscriptionData(datastructure.EntitledSubscription{
			ObjectReference: itemID,
		})
	}

	// Write to DB
	if err := builder.Write(ctx); err != nil {
		log.WithError(err).Error(err)
		return nil, resolvers.ErrInternalServer
	}

	return &response{
		OK:      true,
		Message: "Entitlement Created",
	}, nil
}
