package datastructure

import (
	"fmt"

	"github.com/SevenTV/ServerGo/src/configure"
)

type userUtil struct{}

// Returns the user's enabled channel emotes with names changed to their defined alias
func (*userUtil) GetAliasedEmotes(user *User) []*Emote {

	if user.Emotes == nil {
		return []*Emote{}
	}
	result := make([]*Emote, len(*user.Emotes))
	for i, e := range *user.Emotes {
		if e == nil {
			continue
		}

		// Find alias
		alias := user.EmoteAlias[e.ID.Hex()]
		if alias != "" {
			e.Name = alias // Set new name as alias
		}

		result[i] = e
	}

	return result
}

func (*userUtil) GetProfilePictureURL(user *User) string {
	if user.ProfilePictureID == "" {
		return ""
	} else {
		return fmt.Sprintf("%s/pp/%s/%s", configure.Config.GetString("cdn_url"), user.ID.Hex(), user.ProfilePictureID)
	}
}

var UserUtil = userUtil{}
