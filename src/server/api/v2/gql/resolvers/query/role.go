package query_resolvers

import (
	"context"
	"fmt"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RoleResolver struct {
	ctx context.Context
	v   *datastructure.Role

	fields map[string]*SelectedField
}

func GenerateRoleResolver(ctx context.Context, pRole *datastructure.Role, roleID *primitive.ObjectID, fields map[string]*SelectedField) (*RoleResolver, error) {
	if pRole != nil {
		return &RoleResolver{
			ctx:    ctx,
			v:      pRole,
			fields: fields,
		}, nil
	}
	if roleID == nil {
		return nil, nil
	}

	role := datastructure.GetRole(roleID)
	r := &RoleResolver{
		ctx:    ctx,
		v:      &role,
		fields: fields,
	}
	return r, nil
}

func (r *RoleResolver) ID() string {
	return r.v.ID.Hex()
}

func (r *RoleResolver) Name() string {
	return r.v.Name
}

func (r *RoleResolver) Position() int32 {
	return r.v.Position
}

func (r *RoleResolver) Color() int32 {
	return r.v.Color
}

func (r *RoleResolver) Allowed() string {
	return fmt.Sprint(r.v.Allowed)
}

func (r *RoleResolver) Denied() string {
	return fmt.Sprint(r.v.Denied)
}
