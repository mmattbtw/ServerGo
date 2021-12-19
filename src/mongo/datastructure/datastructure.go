package datastructure

import (
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/mongo/cache"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Emote struct {
	ID               primitive.ObjectID   `json:"id" bson:"_id,omitempty"`
	Name             string               `json:"name" bson:"name"`
	OwnerID          primitive.ObjectID   `json:"owner_id" bson:"owner"`
	Visibility       int32                `json:"visibility" bson:"visibility"`
	Mime             string               `json:"mime" bson:"mime"`
	Status           int32                `json:"status" bson:"status"`
	Tags             []string             `json:"tags" bson:"tags"`
	SharedWith       []primitive.ObjectID `json:"shared_with" bson:"shared_with"`
	LastModifiedDate time.Time            `json:"edited_at" bson:"edited_at"`
	Width            [4]int16             `json:"width" bson:"width"`   // The emote's width in pixels
	Height           [4]int16             `json:"height" bson:"height"` // The emote's height in pixels
	Animated         bool                 `json:"animated" bson:"animated"`

	// ChannelCount is used during the popularity sort check, generated by a pipeline.
	// It is not used anywhere else
	ChannelCount          *int32     `json:"channel_count,omitempty" bson:"channel_count"`
	LastChannelCountCheck *time.Time `json:"channel_count_checked_at,omitempty" bson:"channel_count_checked_at"`

	Owner        *User        `json:"owner,omitempty" bson:"-"`
	AuditEntries *[]*AuditLog `json:"audit_entries,omitempty" bson:"-"`
	Channels     *[]*User     `json:"channels,omitempty" bson:"-"`
	Reports      *[]*Report   `json:"reports,omitempty" bson:"-"`
	Provider     string       `json:"provider,omitempty" bson:"-"`    // The service provider for the emote
	ProviderID   *string      `json:"provider_id,omitempty" bson:"-"` // The emote ID as defined by the foreign provider. Nil if 7TV
	URLs         [][]string   `json:"urls,omitempty" bson:"-"`        // Synthesized URLs to CDN for the emote
}

func GetEmoteURLs(emote Emote) [][]string {
	result := make([][]string, 4)

	for i := 1; i <= 4; i++ {
		a := make([]string, 2)
		a[0] = fmt.Sprintf("%d", i)
		a[1] = utils.GetCdnURL(emote.ID.Hex(), int8(i))

		result[i-1] = a
	}

	return result
}

const (
	EmoteVisibilityPrivate int32 = 1 << iota
	EmoteVisibilityGlobal
	EmoteVisibilityUnlisted
	EmoteVisibilityOverrideBTTV
	EmoteVisibilityOverrideFFZ
	EmoteVisibilityOverrideTwitchGlobal
	EmoteVisibilityOverrideTwitchSubscriber
	EmoteVisibilityZeroWidth
	EmoteVisibilityPermanentlyUnlisted

	EmoteVisibilityAll int32 = (1 << iota) - 1
)

var EmoteVisibilitySimpleMap = map[int32]string{
	EmoteVisibilityPrivate:                  "PRIVATE",
	EmoteVisibilityGlobal:                   "GLOBAL",
	EmoteVisibilityUnlisted:                 "UNLISTED",
	EmoteVisibilityOverrideFFZ:              "OVERRIDE_FFZ",
	EmoteVisibilityOverrideBTTV:             "OVERRIDE_BTTV",
	EmoteVisibilityOverrideTwitchSubscriber: "OVERRIDE_TWITCH_SUBSCRIBER",
	EmoteVisibilityOverrideTwitchGlobal:     "OVERRIDE_TWITCH_GLOBAL",
	EmoteVisibilityZeroWidth:                "ZERO_WIDTH",
}

func (e *Emote) GetSimpleVisibility() []string {
	simpleVis := []string{}
	for vis, s := range EmoteVisibilitySimpleMap {
		if !utils.BitField.HasBits(int64(e.Visibility), int64(vis)) {
			continue
		}

		simpleVis = append(simpleVis, s)
	}

	return simpleVis
}

const (
	EmoteStatusDeleted int32 = iota - 1
	EmoteStatusProcessing
	EmoteStatusPending
	EmoteStatusDisabled
	EmoteStatusLive
)

type User struct {
	ID           primitive.ObjectID   `json:"_id" bson:"_id,omitempty"`
	Email        string               `json:"email" bson:"email"`
	Rank         int32                `json:"rank" bson:"rank"`
	EmoteIDs     []primitive.ObjectID `json:"emote_ids" bson:"emotes"`
	EditorIDs    []primitive.ObjectID `json:"editor_ids" bson:"editors"`
	RoleID       *primitive.ObjectID  `json:"role_id" bson:"role"`
	TokenVersion string               `json:"token_version" bson:"token_version"`

	// Twitch Data
	TwitchID         string              `json:"twitch_id" bson:"id"`
	YouTubeID        string              `json:"yt_id,omitempty" bson:"yt_id,omitempty"`
	DisplayName      string              `json:"display_name" bson:"display_name"`
	Login            string              `json:"login" bson:"login"`
	BroadcasterType  string              `json:"broadcaster_type" bson:"broadcaster_type"`
	ProfileImageURL  string              `json:"profile_image_url" bson:"profile_image_url"`
	OfflineImageURL  string              `json:"offline_image_url" bson:"offline_image_url"`
	Description      string              `json:"description" bson:"description"`
	CreatedAt        time.Time           `json:"twitch_created_at" bson:"twitch_created_at"`
	ViewCount        int32               `json:"view_count" bson:"view_count"`
	ProfilePictureID string              `json:"profile_picture_id,omitempty" bson:"profile_picture_id,omitempty"`
	EmoteAlias       map[string]string   `json:"-" bson:"emote_alias"`           // Emote Alias - backend only
	Badge            *primitive.ObjectID `json:"badge" bson:"badge"`             // User's badge, if any
	EmoteSlots       int32               `json:"emote_slots" bson:"emote_slots"` // User's maximum channel emote slots

	// Relational Data
	Emotes            *[]*Emote       `json:"emotes" bson:"-"`
	OwnedEmotes       *[]*Emote       `json:"owned_emotes" bson:"-"`
	Editors           *[]*User        `json:"editors" bson:"-"`
	Role              *Role           `json:"role" bson:"-"`
	EditorIn          *[]*User        `json:"editor_in" bson:"-"`
	AuditEntries      *[]*AuditLog    `json:"audit_entries" bson:"-"`
	Reports           *[]*Report      `json:"reports" bson:"-"`
	Bans              *[]*Ban         `json:"bans" bson:"-"`
	Notifications     []*Notification `json:"-" bson:"-"`
	NotificationCount *int64          `json:"-" bson:"-"`
}

// Get the user's maximum emote slot count
func (u *User) GetEmoteSlots() int32 {
	if u.EmoteSlots == 0 {
		return configure.Config.GetInt32("limits.meta.channel_emote_slots")
	} else {
		return u.EmoteSlots
	}
}

// Test whether a User has a permission flag
func (u *User) HasPermission(flag int64) bool {
	// This function requires the users role to be queried. if it is not it will panic so we must ensure that the role is present.
	if u.Role == nil {
		role := GetRole(u.RoleID)
		u.Role = &role
	}
	var allowed int64 = 0
	var denied int64 = 0
	if u != nil {
		allowed = u.Role.Allowed
		denied = u.Role.Denied
	}

	if !utils.IsPowerOfTwo(flag) { // Don't evaluate if flag is invalid
		logrus.WithField("flag", flag).Error("flag is not power of two")
		return false
	}

	// Get the sum with denied permissions removed from the bitset
	sum := utils.BitField.RemoveBits(allowed, denied)
	return utils.BitField.HasBits(sum, flag) || utils.BitField.HasBits(sum, RolePermissionAdministrator)
}

type Role struct {
	ID       primitive.ObjectID  `json:"id" bson:"_id"`
	Name     string              `json:"name" bson:"name"`
	Position int32               `json:"position" bson:"position"`
	Color    int32               `json:"color" bson:"color"`
	Allowed  int64               `json:"allowed" bson:"allowed"`
	Denied   int64               `json:"denied" bson:"denied"`
	Default  bool                `json:"default,omitempty" bson:"default"`
	Badge    *primitive.ObjectID `json:"badge,omitempty" bson:"badge,omitempty"`
}

// Get a cached role by ID
func GetRole(id *primitive.ObjectID) Role {
	if id == nil {
		return *DefaultRole
	}

	rid := *id
	roles := cache.CachedRoles.([]Role)

	for _, r := range roles {
		if r.ID != rid {
			continue
		}
		return r
	}
	return *DefaultRole
}

const (
	RolePermissionEmoteCreate          int64 = 1 << iota // 1 - Allows creating emotes
	RolePermissionEmoteEditOwned                         // 2 - Allows editing own emotes
	RolePermissionEmoteEditAll                           // 4 - (Elevated) Allows editing all emotes
	RolePermissionCreateReports                          // 8 - Allows creating reports
	RolePermissionManageReports                          // 16 - (Elevated) Allows managing reports
	RolePermissionBanUsers                               // 32 - (Elevated) Allows banning other users
	RolePermissionAdministrator                          // 64 - (Dangerous, Elevated) GRANTS ALL PERMISSIONS
	RolePermissionManageRoles                            // 128 - (Elevated) Allows managing roles
	RolePermissionManageUsers                            // 256 - (Elevated) Allows managing users
	RolePermissionManageEditors                          // 512 - Allows adding and removing editors from own channel
	RolePermissionEditEmoteGlobalState                   // 1024 - (Elevated) Allows editing the global state of an emote
	RolePermissionEditApplicationMeta                    // 2048 - (Elevated) Allows editing global app metadata, such as the active featured broadcast
	RolePermissionManageEntitlements                     // 4096 - (Elevated) Allows granting and revoking entitlements to and from users
	RolePermissionUseZeroWidthEmote                      // 8192 - Allows zero-width emotes to be enabled
	RolePermissionUseCustomAvatars                       // 16384 - Allows setting a custom avatar

	RolePermissionAll int64 = (1 << iota) - 1
)

const (
	UserRankDefault   int32 = 0
	UserRankModerator int32 = 1
	UserRankAdmin     int32 = 100
)

type Ban struct {
	ID         primitive.ObjectID  `json:"id" bson:"_id,omitempty"`
	UserID     *primitive.ObjectID `json:"user_id" bson:"user_id"`
	Reason     string              `json:"reason" bson:"reason"`
	IssuedByID *primitive.ObjectID `json:"issued_by_id" bson:"issued_by_id"`
	ExpireAt   time.Time           `json:"expire_at" bson:"expire_at"`
}

type AuditLog struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Type      int32              `json:"type" bson:"type"`
	Target    *Target            `json:"target" bson:"target"`
	Changes   []*AuditLogChange  `json:"changes" bson:"changes"`
	Reason    *string            `json:"reason" bson:"reason"`
	CreatedBy primitive.ObjectID `json:"action_user_id" bson:"action_user"`
}

type Target struct {
	ID   *primitive.ObjectID `json:"id" bson:"id"`
	Type string              `json:"type" bson:"type"`
}

type AuditLogChange struct {
	Key      string      `json:"key" bson:"key"`
	OldValue interface{} `json:"old_value" bson:"old_value"`
	NewValue interface{} `json:"new_value" bson:"new_value"`
}

type Report struct {
	ID         primitive.ObjectID  `json:"id" bson:"_id"`
	ReporterID *primitive.ObjectID `json:"reporter_id" bson:"reporter_id"`
	Reason     string              `json:"reason" bson:"target"`
	Target     *Target             `json:"target" bson:"target"`
	Cleared    bool                `json:"cleared" bson:"cleared"`

	ETarget      *Emote       `json:"e_target" bson:"-"`
	UTarget      *User        `json:"u_target" bson:"-"`
	Reporter     *User        `json:"reporter" bson:"-"`
	AuditEntries *[]*AuditLog `json:"audit_entries" bson:"-"`
}

const (
	// Emotes (1-19)
	AuditLogTypeEmoteCreate     = 1
	AuditLogTypeEmoteDelete     = 2
	AuditLogTypeEmoteDisable    = 3
	AuditLogTypeEmoteEdit       = 4
	AuditLogTypeEmoteUndoDelete = 4
	AuditLogTypeEmoteMerge      = 5

	// Auth (20-29)
	AuditLogTypeAuthIn  = 20
	AuditLogTypeAuthOut = 21

	// Users (30-69)
	AuditLogTypeUserCreate              = 30
	AuditLogTypeUserDelete              = 31
	AuditLogTypeUserBan                 = 32
	AuditLogTypeUserEdit                = 33
	AuditLogTypeUserChannelEmoteAdd     = 34
	AuditLogTypeUserChannelEmoteRemove  = 35
	AuditLogTypeUserUnban               = 36
	AuditLogTypeUserChannelEditorAdd    = 37
	AuditLogTypeUserChannelEditorRemove = 38
	AuditLogTypeUserChannelEmoteEdit    = 39

	// Admin (70-89)
	AuditLogTypeAppMaintenanceMode = 70
	AuditLogTypeAppRouteLock       = 71
	AuditLogTypeAppLogsView        = 72
	AuditLogTypeAppScale           = 73
	AuditLogTypeAppNodeCreate      = 74
	AuditLogTypeAppNodeDelete      = 75
	AuditLogTypeAppNodeJoin        = 75
	AuditLogTypeAppNodeUnref       = 76

	// Reports (90-99)
	AuditLogTypeReport      = 90
	AuditLogTypeReportClear = 91
)

type Cosmetic struct {
	ID       primitive.ObjectID   `json:"id" bson:"_id"`
	Kind     CosmeticKind         `json:"kind" bson:"kind"`
	Priority int                  `json:"priority" bson:"priority"`
	Name     string               `json:"name" bson:"name"`
	UserIDs  []primitive.ObjectID `json:"users" bson:"users"`
	Users    []*User              `json:"user_objects" bson:"user_objects,skip,omitempty"`
	Data     bson.Raw             `json:"data" bson:"data"`
}

func (c *Cosmetic) decode(i interface{}) interface{} {
	if err := bson.Unmarshal(c.Data, i); err != nil {
		logrus.WithError(err).Error("cosmetic, failed to decode data")
	}
	return i
}

func (c *Cosmetic) ReadBadge() *CosmeticDataBadge {
	b := c.decode(&CosmeticDataBadge{}).(*CosmeticDataBadge)
	b.ID = c.ID
	return b
}
func (c *Cosmetic) ReadPaint() *CosmeticDataPaint {
	b := c.decode(&CosmeticDataPaint{}).(*CosmeticDataPaint)
	b.ID = c.ID
	return b
}

type CosmeticKind string

const (
	CosmeticKindBadge        CosmeticKind = "BADGE"
	CosmeticKindNametagPaint CosmeticKind = "PAINT"
)

type CosmeticDataBadge struct {
	ID      primitive.ObjectID `json:"id" bson:"-"`
	Tooltip string             `json:"tooltip" bson:"tooltip"`
	Misc    bool               `json:"misc,omitempty" bson:"misc"`
}

type CosmeticDataPaint struct {
	ID primitive.ObjectID `json:"id" bson:"-"`
	// The function used to generate the paint (i.e gradients or an image)
	Function CosmeticPaintFunction `json:"function" bson:"function"`
	// The default color of the paint
	Color ColorRGBA `json:"color" bson:"color"`
	// Gradient stops, a list of positions and colors
	Stops []CosmeticPaintGradientStop `json:"stops" bson:"stops"`
	// Whether or not the gradient repeats outside its original area
	Repeat bool `json:"repeat" bson:"repeat"`
	// Gradient angle in degrees
	Angle int32 `json:"angle" bson:"angle"`
	// Shape of a radial gradient, when the paint is of RADIAL_GRADIENT type
	Shape string `json:"shape,omitempty" bson:"shape,omitempty"`
	// URL of an image, when the paint is of BACKGROUND_IMAGE type
	ImageURL string `json:"image_url,omitempty" bson:"image_url,omitempty"`
	// Properties for a drop shadow
	DropShadow CosmeticPaintDropShadow `json:"drop_shadow,omitempty" bson:"drop_shadow,omitempty"`
	// Animation instructions, if any
	Animation []CosmeticPaintAnimation `json:"animation,omitempty" bson:"animation,omitempty"`
}

type CosmeticPaintFunction string

const (
	CosmeticPaintFunctionLinearGradient CosmeticPaintFunction = "linear-gradient"
	CosmeticPaintFunctionRadialGradient CosmeticPaintFunction = "radial-gradient"
	CosmeticPaintFunctionImageURL       CosmeticPaintFunction = "url"
)

type CosmeticPaintGradientStop struct {
	At    float64   `json:"at" bson:"at"`
	Color ColorRGBA `json:"color" bson:"color"`
}

type CosmeticPaintDropShadow struct {
	OffsetX int32     `json:"x_offset" bson:"x_offset"`
	OffsetY int32     `json:"y_offset" bson:"y_offset"`
	Radius  int32     `json:"radius" bson:"radius"`
	Color   ColorRGBA `json:"color" bson:"color"`
}

type CosmeticPaintAnimation struct {
	Speed     uint32                           `json:"speed" bson:"speed"`
	Keyframes []CosmeticPaintAnimationKeyframe `json:"keyframes" bson:"keyframes"`
}

type CosmeticPaintAnimationKeyframe struct {
	At float32 `json:"at" bson:"at"`
	X  float32 `json:"x" bson:"x"`
	Y  float32 `json:"y" bson:"y"`
}

type ColorRGBA [4]uint8

type Meta struct {
	Announcement      string   `json:"announcement"`
	FeaturedBroadcast string   `json:"featured_broadcast"`
	Roles             []string `json:"roles"`
}

type Broadcast struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	ThumbnailURL string   `json:"thumbnail_url"`
	ViewerCount  int32    `json:"viewer_count"`
	Type         string   `json:"type"`
	GameName     string   `json:"game_name"`
	GameID       string   `json:"game_id"`
	Language     string   `json:"language"`
	Tags         []string `json:"tags"`
	Mature       bool     `json:"mature"`
	StartedAt    string   `json:"started_at"`
	UserID       string   `json:"user_id"`
}

type Notification struct {
	ID           primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Announcement bool               `json:"announcement" bson:"announcement"` // If true, the notification is global and visible to all users regardless of targets

	Title        string                    `json:"title" bson:"title"`                 // The notification's heading / title
	MessageParts []NotificationMessagePart `json:"message_parts" bson:"message_parts"` // The parts making up the notification's formatted message

	Read   bool      `json:"read" bson:"read,omitempty"`
	ReadAt time.Time `json:"read_at" bson:"read_at,omitempty"`
	Users  []*User   `json:"users" bson:"-"`  // The users mentioned in this notification
	Emotes []*Emote  `json:"emotes" bson:"-"` // The emotesm entioned in this notification
}

type NotificationMessagePart struct {
	Type NotificationContentMessagePartType `json:"part_type" bson:"part_type"` // The type of this part

	Text    *string             `json:"text" bson:"text"`
	Mention *primitive.ObjectID `json:"mention" bson:"mention"`
}

type NotificationReadState struct {
	TargetUser   primitive.ObjectID `json:"target" bson:"target"`                // The user targeted to see the notification
	Notification primitive.ObjectID `json:"notification_id" bson:"notification"` // The notification that can be read
	Read         bool               `json:"read" bson:"read"`                    // Whether the user read the notification
	ReadAt       *time.Time         `json:"read_at" bson:"read_at"`              // When the notification was read
}

const (
	NotificationMessagePartTypeText NotificationContentMessagePartType = 1 + iota
	NotificationMessagePartTypeUserMention
	NotificationMessagePartTypeEmoteMention
	NotificationMessagePartTypeRoleMention
)

type NotificationContentMessagePartType int8
