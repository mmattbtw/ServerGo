package utils

import (
	"fmt"

	"github.com/SevenTV/ServerGo/src/configure"
)

func GetEmoteImageURL(emoteID string) string {
	return configure.Config.GetString("cdn_url") + fmt.Sprintf("/emote/%s/%dx", emoteID, 4)
}

func GetEmotePageURL(emoteID string) string {
	return configure.Config.GetString("website_url") + fmt.Sprintf("/emotes/%s", emoteID)
}

func GetUserPageURL(userID string) string {
	return configure.Config.GetString("website_url") + fmt.Sprintf("/users/%s", userID)
}
