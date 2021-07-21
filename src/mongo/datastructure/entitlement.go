package datastructure

import "go.mongodb.org/mongo-driver/bson/primitive"

type Entitlement struct {
	ID primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	// Kind represents what item this entitlement grants
	Kind EntitlementKind `json:"kind" bson:"kind"`
	// Data referencing the entitled item
	Data interface{} `json:"data" bson:"data"`
	// The user who is entitled to the item
	UserID primitive.ObjectID `json:"user_id" bson:"user_id"`
	// Wether this entitlement is currently inactive
	Disabled bool `json:"disabled,omitempty" bson:"disabled,omitempty"`
}

// Entitlement Kind: Subscription
type EntitlementWithSubscription struct {
	*Entitlement
	Data EntitledSubscription `json:"data" bson:"data"`
}

// Entitlement Kind: Badge
type EntitlementWithBadge struct {
	*Entitlement
	Data EntitledBadge `json:"data" bson:"data"`
}

// Entitlement Kind: Role
type EntitlementWithRole struct {
	*Entitlement
	Data entitledrole `json:"data" bson:"data"`
}

// Entitlement Kind: EmoteSet
type EntitlementWithEmoteSet struct {
	*Entitlement
	Data EntitlementEmoteSet `json:"data" bson:"data"`
}

// A string representing an Entitlement Kind
type EntitlementKind string

var (
	EntitlementKindSubscription EntitlementKind // Subscription Entitlement
	EntitlementKindBadge        EntitlementKind // Badge Entitlement
	EntitlementKindRole         EntitlementKind // Role Entitlement
	EntitlementKindEmoteSet     EntitlementKind // Emote Set Entitlement
)

type EntitledSubscription struct {
	// The ID of the subscription
	SubscriptionID primitive.ObjectID `json:"subscription_id" bson:"subscription_id"`
}

type EntitledBadge struct {
	BadgeID  primitive.ObjectID `json:"badge_id" bson:"badge_id"`
	Selected bool               `json:"selected" bson:"selected"`
}

type EntitledRole struct {
	RoleID primitive.ObjectID `json:"role_id" bson:"role_id"`
	// Whether or not the entitlemet will cause the user's role to be overriden,
	// even if their current role has a higher position
	Override bool `json:"override" bson:"override"`
}

type EntitlementEmoteSet struct {
	SetID      primitive.ObjectID   `json:"set_id" bson:"set_id"`
	UnicodeTag string               `json:"unicode_tag" bson:"unicode_tag"`
	EmoteIDs   []primitive.ObjectID `json:"emote_ids" bson:"emotes"`

	// Relational

	// A list of emotes for this emote set entitlement
	Emotes []*Emote `json:"emotes" bson:"-"`
}
