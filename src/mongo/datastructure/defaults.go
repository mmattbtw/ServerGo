package datastructure

import (
	"github.com/SevenTV/ServerGo/src/configure"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// The default role.
// It grants permissions for users without a defined role
var DefaultRole *Role = &Role{
	Allowed: configure.Config.GetInt64("default_permissions"),
	Denied:  0,
	Default: true,
}

var DeletedUser *User = &User{
	ID:          deletedUserID,
	Login:       "*deleteduser",
	DisplayName: "Deleted User",
}

var deletedUserID, _ = primitive.ObjectIDFromHex("000000000000000000000001")
