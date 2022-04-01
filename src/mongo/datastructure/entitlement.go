package datastructure

import (
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Entitlement is a binding between a resource and a user
// It grants the user access to the bound resource
// and may define some additional properties on top.
type Entitlement struct {
	ID primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	// Kind represents what item this entitlement grants
	Kind EntitlementKind `json:"kind" bson:"kind"`
	// Data referencing the entitled item
	Data bson.Raw `json:"data" bson:"data"`
	// The user who is entitled to the item
	UserID primitive.ObjectID `json:"user_id" bson:"user_id"`
	// Wether this entitlement is currently inactive
	Disabled bool `json:"disabled,omitempty" bson:"disabled,omitempty"`
}

func (e *Entitlement) GetData() EntitlementData {
	return EntitlementData(e.Data)
}

type EntitlementData bson.Raw

func (d EntitlementData) ReadItem() *EntitledItem {
	return d.unmarshal(&EntitledItem{}).(*EntitledItem)
}

func (d EntitlementData) ReadRole() *EntitledRole {
	return d.unmarshal(&EntitledRole{}).(*EntitledRole)
}

func (d EntitlementData) unmarshal(i interface{}) interface{} {
	if err := bson.Unmarshal(d, i); err != nil {
		logrus.WithError(err).Error("message, decoding message data failed")
	}
	return i
}

// A string representing an Entitlement Kind
type EntitlementKind string

var (
	EntitlementKindSubscription = EntitlementKind("SUBSCRIPTION") // Subscription Entitlement
	EntitlementKindBadge        = EntitlementKind("BADGE")        // Badge Entitlement
	EntitlementKindPaint        = EntitlementKind("PAINT")        // Nametag Paint Entitlement
	EntitlementKindRole         = EntitlementKind("ROLE")         // Role Entitlement
	EntitlementKindEmoteSet     = EntitlementKind("EMOTE_SET")    // Emote Set Entitlement
)

type EntitledItem struct {
	ObjectReference primitive.ObjectID  `json:"-" bson:"ref"`
	RoleBinding     *primitive.ObjectID `json:"role_binding,omitempty" bson:"role_binding,omitempty"`
	Selected        bool                `json:"selected,omitempty" bson:"selected,omitempty"`
}

// (Data) Subscription binding in an Entitlement
type EntitledSubscription struct {
	ID string `json:"id" bson:"-"`
	// The ID of the subscription
	ObjectReference primitive.ObjectID `json:"-" bson:"ref"`
}

// (Data) Badge binding in an Entitlement
type EntitledBadge struct {
	ID              string             `json:"id" bson:"-"`
	ObjectReference primitive.ObjectID `json:"-" bson:"ref"`
	Selected        bool               `json:"selected" bson:"selected"`
	// The role required for the badge to show up
	RoleBinding   *primitive.ObjectID `json:"role_binding" bson:"role_binding"`
	RoleBindingID *string             `json:"role_binding_id" bson:"-"`
}

type EntitledPaint struct {
	ID              string             `json:"id" bson:"-"`
	ObjectReference primitive.ObjectID `json:"-" bson:"ref"`
	Selected        bool               `json:"selected" bson:"selected"`
	// The role required for the paint to show up
	RoleBinding   *primitive.ObjectID `json:"role_binding" bson:"role_binding"`
	RoleBindingID *string             `json:"role_binding_id" bson:"-"`
}

// (Data) Role binding in an Entitlement
type EntitledRole struct {
	ID              string             `json:"id" bson:"-"`
	ObjectReference primitive.ObjectID `json:"-" bson:"ref"`
}

// (Data) Emote Set binding in an Entitlement
type EntitledEmoteSet struct {
	ID              string               `json:"id" bson:"-"`
	ObjectReference primitive.ObjectID   `json:"-" bson:"ref"`
	UnicodeTag      string               `json:"unicode_tag" bson:"unicode_tag"`
	EmoteIDs        []primitive.ObjectID `json:"emote_ids" bson:"emotes"`

	// Relational

	// A list of emotes for this emote set entitlement
	Emotes []*Emote `json:"emotes" bson:"-"`
}
