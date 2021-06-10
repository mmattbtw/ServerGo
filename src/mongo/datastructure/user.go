package datastructure

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
		} else {
		}

		result[i] = e
	}

	return result
}

var UserUtil = userUtil{}
