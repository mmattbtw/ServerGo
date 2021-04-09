package mongo

import (
	"time"

	"github.com/SevenTV/ServerGo/utils"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Emote struct {
	ID               primitive.ObjectID   `json:"_id" bson:"_id,omitempty"`
	Name             string               `json:"name" bson:"name"`
	OwnerID          primitive.ObjectID   `json:"owner_id" bson:"owner"`
	Visibility       int32                `json:"visibility" bson:"visibility"`
	Mime             string               `json:"mime" bson:"mime"`
	Status           int32                `json:"status" bson:"status"`
	Tags             []string             `json:"tags" bson:"tags"`
	SharedWith       []primitive.ObjectID `json:"shared_with" bson:"shared_with"`
	LastModifiedDate time.Time            `json:"edited_at" bson:"edited_at"`

	Owner        *User        `json:"owner" bson:"-"`
	AuditEntries *[]*AuditLog `json:"audit_entries" bson:"-"`
	Channels     *[]*User     `json:"channels" bson:"-"`
	Reports      *[]*Report   `json:"reports" bson:"-"`
}

const (
	EmoteVisibilityNormal int32 = iota
	EmoteVisibilityPrivate
	EmoteVisibilityGlobal
)

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
	TwitchID        string `json:"twitch_id" bson:"id"`
	DisplayName     string `json:"display_name" bson:"display_name"`
	Login           string `json:"login" bson:"login"`
	BroadcasterType string `json:"broadcaster_type" bson:"broadcaster_type"`
	ProfileImageURL string `json:"profile_image_url" bson:"profile_image_url"`

	// Relational Data
	Emotes       *[]*Emote    `json:"emotes" bson:"-"`
	OwnedEmotes  *[]*Emote    `json:"owned_emotes" bson:"-"`
	Editors      *[]*User     `json:"editors" bson:"-"`
	Role         *Role        `json:"role" bson:"-"`
	EditorIn     *[]*User     `json:"editor_in" bson:"-"`
	AuditEntries *[]*AuditLog `json:"audit_entries" bson:"-"`
	Reports      *[]*Report   `json:"reports" bson:"-"`
	Bans         *[]*Ban      `json:"bans" bson:"-"`
}

// Test whether a User has a permission flag
func UserHasPermission(user *User, flag int64) bool {
	allowed := utils.Ternary(&user.Role.Allowed != nil, user.Role.Allowed, 0).(int64)
	denied := utils.Ternary(&user.Role.Denied != nil, user.Role.Denied, 0).(int64)
	if user == nil {
		return false
	}
	if !utils.IsPowerOfTwo(flag) { // Don't evaluate if flag is invalid
		log.Errorf("HasPermission, err=flag is not power of two (%s)", flag)
		return false
	}

	// Get the sum with denied permissions removed from the bitset
	sum := utils.RemoveBits(allowed, denied)
	return utils.HasBits(sum, flag) || utils.HasBits(sum, RolePermissionAdministrator)
}

type Role struct {
	ID       primitive.ObjectID `json:"id" bson:"_id"`
	Name     string             `json:"name" bson:"name"`
	Position int32              `json:"position" bson:"position"`
	Color    int32              `json:"color" bson:"color"`
	Allowed  int64              `json:"allowed" bson:"allowed"`
	Denied   int64              `json:"denied" bson:"denied"`
}

// The default role.
// It grants permissions for users without a defined role
var DefaultRole *Role = &Role{
	ID:      primitive.NewObjectID(),
	Name:    "Default",
	Allowed: RolePermissionDefault,
	Denied:  0,
}

// Get cached roles
func GetRoles() []Role {
	return Ctx.Value(utils.AllRolesKey).([]Role)
}

// Get a cached role by ID
func GetRole(id primitive.ObjectID) Role {
	var role Role
	roles := GetRoles()

	for _, r := range roles {
		if r.ID != id {
			continue
		}

		role = r
		break
	}

	return role
}

const (
	RolePermissionEmoteCreate    int64 = 1 << iota // 1 - Allows creating emotes
	RolePermissionEmoteEditOwned int64 = 1 << iota // 2 - Allows editing own emotes
	RolePermissionEditAll        int64 = 1 << iota // 4 - (Elevated) Allows editing all emotes

	RolePermissionCreateReports int64 = 1 << iota // 8 - Allows creating reports
	RolePermissionManageReports int64 = 1 << iota // 16 - (Elevated) Allows managing reports

	RolePermissionBanUsers      int64 = 1 << iota // 32 - (Elevated) Allows banning other users
	RolePermissionAdministrator int64 = 1 << iota // 64 - (Dangerous, Elevated) GRANTS ALL PERMISSIONS
	RolePermissionManageRoles   int64 = 1 << iota // 128 - (Elevated) Allows managing roles
	RolePermissionManageUsers   int64 = 1 << iota // 256 - (Elevated) Allows managing users

	RolePermissionManageEditors int64 = 1 << iota // 512 - Allows adding and removing editors from own channel

	RolePermissionAll     int64 = (1 << iota) - 1                                                                                                        // Sum of all permissions combined
	RolePermissionDefault int64 = (RolePermissionEmoteCreate | RolePermissionEmoteEditOwned | RolePermissionCreateReports | RolePermissionManageEditors) // Default permissions for users without a role
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
	Active     bool                `json:"active" bson:"active"`
	IssuedByID *primitive.ObjectID `json:"issued_by_id" bson:"issued_by_id"`
}

type AuditLog struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Type      int32              `json:"type" bson:"type"`
	Target    *Target            `json:"target" bson:"target"`
	Changes   []*AuditLogChange  `json:"changes" bson:"changes"`
	Reason    *string            `json:"reason" bson:"reason"`
	CreatedBy primitive.ObjectID `json:"action_user" bson:"action_user"`
}

type Target struct {
	ID   *primitive.ObjectID `json:"id" bson:"id"`
	Type string              `json:"type" bson:"type"`
}

type AuditLogChange struct {
	Key      string      `json:"type" bson:"type"`
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
	AuditLogTypeEmoteCreate int32 = 1
	AuditLogTypeEmoteDelete int32 = iota
	AuditLogTypeEmoteDisable
	AuditLogTypeEmoteEdit
	AuditLogTypeEmoteUndoDelete

	AuditLogTypeAuthIn  int32 = 21
	AuditLogTypeAuthOut int32 = iota

	AuditLogTypeUserCreate int32 = 31
	AuditLogTypeUserDelete int32 = iota
	AuditLogTypeUserBan
	AuditLogTypeUserEdit
	AuditLogTypeUserChannelEmoteAdd
	AuditLogTypeUserChannelEmoteRemove
	AuditLogTypeUserUnban
	AuditLogTypeUserChannelEditorAdd
	AuditLogTypeUserChannelEditorRemove

	AuditLogTypeAppMaintenanceMode int32 = 51
	AuditLogTypeAppRouteLock       int32 = iota
	AuditLogTypeAppLogsView
	AuditLogTypeAppScale
	AuditLogTypeAppNodeCreate
	AuditLogTypeAppNodeDelete
	AuditLogTypeAppNodeJoin
	AuditLogTypeAppNodeUnref

	AuditLogTypeReport      int32 = 71
	AuditLogTypeReportClear int32 = iota
)
