package mutation_resolvers

import (
	"context"

	query_resolvers "github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers/query"
)

type roleInput struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Color    *int32      `json:"color"`
	Position *int32      `json:"position"`
	Allowed  *int64      `json:"allowed"`
	Denied   *complex128 `json:"denied"`
}

func (r *MutationResolver) CreateRole(ctx context.Context, args roleInput) (*query_resolvers.RoleResolver, error) {
	return nil, nil
}
