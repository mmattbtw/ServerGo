package restutil

import (
	"encoding/json"
	"fmt"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
)

type ErrorResponse struct {
	Status  int
	Message string
}

func (e *ErrorResponse) Send(c *fiber.Ctx, placeholders ...string) error {
	e.Message = fmt.Sprintf(e.Message, placeholders)
	fmt.Println("hello", e.Message, placeholders)

	b, _ := json.Marshal(e)
	return c.Status(e.Status).Send(b)
}

func createErrorResponse(status int, message string) ErrorResponse {
	return ErrorResponse{
		status, message,
	}
}

var (
	ErrUnknownEmote   = createErrorResponse(400, "Unknown Emote")
	MalformedObjectId = createErrorResponse(400, "Malformed Object ID")
	ErrInternalServer = createErrorResponse(500, "Internal Server Error (%s)")
)

func CreateEmoteResponse(emote datastructure.Emote, owner *datastructure.User) emoteResponse {
	// Generate URLs
	urls := make([][]string, 4)
	for i := 1; i <= 4; i++ {
		a := make([]string, 2)
		a[0] = fmt.Sprintf("%d", i)
		a[1] = utils.GetCdnURL(emote.ID.Hex(), int8(i))

		urls[i-1] = a
	}

	// Generate simple visibility value
	simpleVis := []string{}
	for vis, s := range emoteVisibilitySimpleMap {
		fmt.Println("vis:", emote.Visibility, s)
		if !utils.BitField.HasBits(int64(emote.Visibility), int64(vis)) {
			fmt.Println("does not have", s)
			continue
		}

		simpleVis = append(simpleVis, s)
	}

	// Create the final response
	response := emoteResponse{
		ID:               emote.ID.Hex(),
		Name:             emote.Name,
		Owner:            CreateUserResponse(owner),
		Visibility:       emote.Visibility,
		VisibilitySimple: &simpleVis,
		Mime:             emote.Mime,
		Status:           emote.Status,
		Tags:             emote.Tags,
		Width:            emote.Width,
		Height:           emote.Height,
		URLs:             urls,
	}

	return response
}

var emoteVisibilitySimpleMap = map[int32]string{
	datastructure.EmoteVisibilityPrivate:                  "PRIVATE",
	datastructure.EmoteVisibilityGlobal:                   "GLOBAL",
	datastructure.EmoteVisibilityHidden:                   "HIDDEN",
	datastructure.EmoteVisibilityOverrideFFZ:              "OVERRIDE_FFZ",
	datastructure.EmoteVisibilityOverrideBTTV:             "OVERRIDE_BTTV",
	datastructure.EmoteVisibilityOverrideTwitchSubscriber: "OVERRIDE_TWITCH_SUBSCRIBER",
	datastructure.EmoteVisibilityOverrideTwitchGlobal:     "OVERRIDE_TWITCH_GLOBAL",
}

type emoteResponse struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Owner            *userResponse `json:"owner"`
	Visibility       int32         `json:"visibility"`
	VisibilitySimple *[]string     `json:"visibility_simple"`
	Mime             string        `json:"mime"`
	Status           int32         `json:"status"`
	Tags             []string      `json:"tags"`
	Width            [4]int16      `json:"width"`
	Height           [4]int16      `json:"height"`
	URLs             [][]string    `json:"urls"`
}

func CreateUserResponse(user *datastructure.User) *userResponse {
	response := userResponse{
		ID:          user.ID.Hex(),
		Login:       user.Login,
		DisplayName: user.DisplayName,
	}
	if user.RoleID != nil {
		response.Role = datastructure.GetRole(user.RoleID)
	}

	return &response
}

type userResponse struct {
	ID          string             `json:"id"`
	TwitchID    string             `json:"twitch_id"`
	Login       string             `json:"login"`
	DisplayName string             `json:"display_name"`
	Role        datastructure.Role `json:"role"`
}
