package restutil

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ErrorResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

func (e *ErrorResponse) Send(c *fiber.Ctx, placeholders ...string) error {
	if len(placeholders) > 0 {
		e.Message = fmt.Sprintf(e.Message, strings.Join(placeholders, ", "))
		e.Reason = strings.Join(placeholders, ", ")
	}

	b, _ := json.Marshal(e)
	return c.Status(e.Status).Send(b)
}

func createErrorResponse(status int, message string) *ErrorResponse {
	return &ErrorResponse{
		status, message, "",
	}
}

var (
	ErrUnknownEmote       = func() *ErrorResponse { return createErrorResponse(404, "Unknown Emote") }
	ErrUnknownUser        = func() *ErrorResponse { return createErrorResponse(404, "Unknown User") }
	MalformedObjectId     = func() *ErrorResponse { return createErrorResponse(400, "Malformed Object ID") }
	ErrInternalServer     = func() *ErrorResponse { return createErrorResponse(500, "Internal Server Error (%s)") }
	ErrBadRequest         = func() *ErrorResponse { return createErrorResponse(400, "Bad Request (%s)") }
	ErrLoginRequired      = func() *ErrorResponse { return createErrorResponse(403, "Authentication Required") }
	ErrAccessDenied       = func() *ErrorResponse { return createErrorResponse(403, "Insufficient Privilege") }
	ErrMissingQueryParams = func() *ErrorResponse { return createErrorResponse(400, "Missing Query Params (%s)") }
)

func CreateEmoteResponse(emote *datastructure.Emote, owner *datastructure.User) EmoteResponse {
	// Generate URLs
	urls := make([][]string, 4)
	for i := 1; i <= 4; i++ {
		a := make([]string, 2)
		a[0] = fmt.Sprintf("%d", i)
		a[1] = utils.GetCdnURL(emote.ID.Hex(), int8(i))

		urls[i-1] = a
	}

	// Generate simple visibility value
	simpleVis := emote.GetSimpleVisibility()

	// Create the final response
	response := EmoteResponse{
		ID:               emote.ID.Hex(),
		Name:             emote.Name,
		Owner:            CreateUserResponse(datastructure.DeletedUser),
		Visibility:       emote.Visibility,
		VisibilitySimple: &simpleVis,
		Mime:             emote.Mime,
		Status:           emote.Status,
		Tags:             utils.Ternary(emote.Tags != nil, emote.Tags, []string{}).([]string),
		Width:            emote.Width,
		Height:           emote.Height,
		URLs:             urls,
	}
	if owner != nil {
		response.Owner = CreateUserResponse(owner)
	}

	return response
}

type EmoteResponse struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Owner            *UserResponse `json:"owner"`
	Visibility       int32         `json:"visibility"`
	VisibilitySimple *[]string     `json:"visibility_simple"`
	Mime             string        `json:"mime"`
	Status           int32         `json:"status"`
	Tags             []string      `json:"tags"`
	Width            [4]int16      `json:"width"`
	Height           [4]int16      `json:"height"`
	URLs             [][]string    `json:"urls"`
}

func CreateUserResponse(user *datastructure.User, opt ...UserResponseOptions) *UserResponse {
	var options UserResponseOptions
	if len(opt) > 0 {
		options = opt[0]
	}

	response := UserResponse{
		ID:               user.ID.Hex(),
		Login:            user.Login,
		DisplayName:      user.DisplayName,
		Role:             datastructure.GetRole(user.RoleID),
		EmoteAliases:     utils.Ternary(options.IncludeAliases, user.EmoteAlias, map[string]string{}).(map[string]string),
		ProfilePictureID: user.ProfilePictureID,
	}

	return &response
}

type UserResponseOptions struct {
	IncludeAliases bool
}

type UserResponse struct {
	ID               string             `json:"id"`
	TwitchID         string             `json:"twitch_id"`
	Login            string             `json:"login"`
	DisplayName      string             `json:"display_name"`
	Role             datastructure.Role `json:"role"`
	EmoteAliases     map[string]string  `json:"emote_aliases,omitempty"`
	ProfilePictureID string             `json:"profile_picture_id,omitempty"`
}

func CreateBadgeResponse(badge *datastructure.Cosmetic, users []*datastructure.User, idType string) *BadgeCosmeticResponse {
	// Get user list
	userIDs := selectUserIDType(users, idType)

	// Generate URLs
	urls := make([][]string, 3)
	for i := 1; i <= 3; i++ {
		a := make([]string, 2)
		a[0] = fmt.Sprintf("%d", i)
		a[1] = utils.GetBadgeCdnURL(badge.ID.Hex(), int8(i))

		urls[i-1] = a
	}

	data := badge.ReadBadge()
	if data == nil {
		return &BadgeCosmeticResponse{
			ID:      primitive.NilObjectID.Hex(),
			Name:    "Error",
			Tooltip: "Error",
		}
	}

	response := &BadgeCosmeticResponse{
		ID:      badge.ID.Hex(),
		Name:    badge.Name,
		Tooltip: data.Tooltip,
		Users:   userIDs,
		URLs:    urls,
		Misc:    data.Misc,
	}

	return response
}

func CreatePaintResponse(paint *datastructure.Cosmetic, users []*datastructure.User, idType string) *PaintCosmeticResponse {
	// Get user list
	userIDs := selectUserIDType(users, idType)

	data := paint.ReadPaint()
	if data == nil {
		return &PaintCosmeticResponse{
			ID:   paint.ID.Hex(),
			Name: "Error",
		}
	}

	return &PaintCosmeticResponse{
		ID:          paint.ID.Hex(),
		Name:        paint.Name,
		Users:       userIDs,
		Color:       data.Color,
		Function:    string(data.Function),
		Stops:       data.Stops,
		Repeat:      data.Repeat,
		Angle:       data.Angle,
		Shape:       data.Shape,
		ImageURL:    data.ImageURL,
		DropShadow:  data.DropShadow,
		DropShadows: data.DropShadows,
		Animation:   data.Animation,
	}
}

func selectUserIDType(users []*datastructure.User, t string) []string {
	userIDs := make([]string, len(users))
	for i, u := range users {
		switch t {
		case "object_id":
			userIDs[i] = u.ID.Hex()
		case "twitch_id":
			userIDs[i] = u.TwitchID
		case "login":
			userIDs[i] = u.Login
		}
	}

	return userIDs
}

type BadgeCosmeticResponse struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Tooltip string     `json:"tooltip"`
	URLs    [][]string `json:"urls"`
	Users   []string   `json:"users"`
	Misc    bool       `json:"misc,omitempty"`
}

type PaintCosmeticResponse struct {
	ID          string                                    `json:"id"`
	Name        string                                    `json:"name"`
	Users       []string                                  `json:"users"`
	Function    string                                    `json:"function"`
	Color       *int32                                    `json:"color"`
	Stops       []datastructure.CosmeticPaintGradientStop `json:"stops"`
	Repeat      bool                                      `json:"repeat"`
	Angle       int32                                     `json:"angle"`
	Shape       string                                    `json:"shape,omitempty"`
	ImageURL    string                                    `json:"image_url,omitempty"`
	DropShadow  datastructure.CosmeticPaintDropShadow     `json:"drop_shadow,omitempty"`
	DropShadows []datastructure.CosmeticPaintDropShadow   `json:"drop_shadows,omitempty"`
	Animation   datastructure.CosmeticPaintAnimation      `json:"animation,omitempty"`
}
