package datastructure

import "go.mongodb.org/mongo-driver/bson/primitive"

// The default role.
// It grants permissions for users without a defined role
var DefaultRole *Role = &Role{
	ID:      primitive.NewObjectID(),
	Name:    "Default",
	Allowed: RolePermissionDefault,
	Denied:  0,
}

var DeletedUser *User = &User{
	Login:       "*deleteduser",
	DisplayName: "Deleted User",
}
