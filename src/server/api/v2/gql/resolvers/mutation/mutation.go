package mutation_resolvers

import "github.com/SevenTV/ServerGo/src/mongo/datastructure"

type MutationResolver struct{}

type response struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type emoteInput struct {
	ID         string    `json:"id"`
	Name       *string   `json:"name"`
	OwnerID    *string   `json:"owner_id"`
	Visibility *int32    `json:"visibility"`
	Tags       *[]string `json:"tags"`
}

type userInput struct {
	ID         string  `json:"id"`
	RoleID     *string `json:"role_id"`
	EmoteSlots *int32  `json:"emote_slots"`
}

type entitlementCreateInput struct {
	Subscription datastructure.EntitledSubscription
	Badge        datastructure.EntitledBadge
	Role         datastructure.EntitledRole
	EmoteSet     datastructure.EntitledEmoteSet
}
