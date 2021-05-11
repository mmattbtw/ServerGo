package query_resolvers

import (
	"context"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
)

type banResolver struct {
	ctx context.Context
	v   *datastructure.Ban

	fields map[string]*SelectedField
}

func GenerateBanResolver(ctx context.Context, ban *datastructure.Ban, fields map[string]*SelectedField) (*banResolver, error) {
	return &banResolver{
		ctx:    ctx,
		v:      ban,
		fields: fields,
	}, nil
}

func (r *banResolver) ID() string {
	return r.v.ID.Hex()
}

func (r *banResolver) UserID() *string {
	if r.v.UserID == nil {
		return nil
	}
	hex := r.v.UserID.Hex()
	return &hex
}

func (r *banResolver) Reason() string {
	return r.v.Reason
}

func (r *banResolver) Active() bool {
	return r.v.Active
}

func (r *banResolver) IssuedByID() *string {
	if r.v.IssuedByID == nil {
		return nil
	}
	hex := r.v.IssuedByID.Hex()
	return &hex
}

func (r *banResolver) User() (*UserResolver, error) {
	if r.v.UserID == nil {
		return nil, nil
	}
	return GenerateUserResolver(r.ctx, nil, r.v.UserID, r.fields["user"].Children)
}

func (r *banResolver) IssuedBy() (*UserResolver, error) {
	if r.v.IssuedByID == nil {
		return nil, nil
	}
	return GenerateUserResolver(r.ctx, nil, r.v.IssuedByID, r.fields["user"].Children)
}
