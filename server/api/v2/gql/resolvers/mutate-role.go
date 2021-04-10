package resolvers

import "context"

type roleInput struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Color    *int32      `json:"color"`
	Position *int32      `json:"position"`
	Allowed  *int64      `json:"allowed"`
	Denied   *complex128 `json:"denied"`
}

func (r *RootResolver) CreateRole(ctx context.Context, args roleInput) (*roleResolver, error) {
	return nil, nil
}
