package mutation_resolvers

import (
	"context"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
)

func (*MutationResolver) CreateEntitlement(ctx context.Context, args struct {
	Kind     datastructure.EntitlementKind
	Data     entitlementCreateInput
	UserID   string
	Disabled *bool
}) (*response, error) {
	builder := actions.Entitlements.Create()

	builder.SetKind(args.Kind)

	return nil, nil
}
