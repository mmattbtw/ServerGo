package actions

import (
	"context"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (b EntitlementBuilder) Write(ctx context.Context) error {
	upsert := true

	// Create new Object ID if this is a new notification
	if b.Entitlement.ID.IsZero() {
		b.Entitlement.ID = primitive.NewObjectID()
	}

	if _, err := mongo.Database.Collection("entitlements").UpdateByID(ctx, b.Entitlement.ID, bson.M{
		"$set": b.Entitlement,
	}, &options.UpdateOptions{
		Upsert: &upsert,
	}); err != nil {
		log.WithError(err).Error("mongo")
		return err
	}

	return nil
}

// SetKind: Change the entitlement's kind
func (b EntitlementBuilder) SetKind(kind datastructure.EntitlementKind) EntitlementBuilder {
	b.Entitlement.Kind = kind

	return b
}

// SetUserID: Change the entitlement's assigned user
func (b EntitlementBuilder) SetUserID(id primitive.ObjectID) EntitlementBuilder {
	b.Entitlement.UserID = id

	return b
}

// SetSubscriptionData: Add a subscription reference to the entitlement
func (b EntitlementBuilder) SetSubscriptionData(data datastructure.EntitledSubscription) EntitlementBuilder {
	v := datastructure.EntitlementWithSubscription{
		Entitlement: b.Entitlement,
		Data:        data,
	}

	b.Entitlement.Data = v
	return b
}

// SetBadgeData: Add a badge reference to the entitlement
func (b EntitlementBuilder) SetBadgeData(data datastructure.EntitledBadge) EntitlementBuilder {
	v := datastructure.EntitlementWithBadge{
		Entitlement: b.Entitlement,
		Data:        data,
	}

	b.Entitlement.Data = v
	return b
}

// SetRoleData: Add a role reference to the entitlement
func (b EntitlementBuilder) SetRoleData(data datastructure.EntitledRole) EntitlementBuilder {
	v := datastructure.EntitlementWithRole{
		Entitlement: b.Entitlement,
		Data:        data,
	}

	b.Entitlement.Data = v
	return b
}

// SetEmoteSetData: Add an emote set reference to the entitlement
func (b EntitlementBuilder) SetEmoteSetData(data datastructure.EntitledEmoteSet) EntitlementBuilder {
	v := datastructure.EntitlementWithEmoteSet{
		Entitlement: b.Entitlement,
		Data:        data,
	}

	b.Entitlement.Data = v
	return b
}

// Create: Get a new entitlement builder
func (entitlements) Create() EntitlementBuilder {
	return EntitlementBuilder{
		Entitlement: datastructure.Entitlement{},
	}
}

// With: Get an entitledment builder tied to an entitlement
func (entitlements) With(e datastructure.Entitlement) EntitlementBuilder {
	return EntitlementBuilder{
		Entitlement: e,
	}
}
