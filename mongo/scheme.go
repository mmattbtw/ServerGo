package mongo

import (
	"time"

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
	LastModifiedDate time.Time                `json:"last_modified_date" bson:"last_modified_date"`

	Owner        *User        `json:"owner" bson:"-"`
	AuditEntries *[]*AuditLog `json:"audit_entries" bson:"-"`
	Reports      *[]*Report   `json:"reports" bson:"-"`
}

const (
	EmoteVisibilityNormal int32 = iota
	EmoteVisibilityPrivate
	EmoteVisibilityGlobal
)

const (
	EmoteStatusDeleted    int32 = -1
	EmoteStatusProcessing int32 = iota
	EmoteStatusPending
	EmoteStatusDisabled
	EmoteStatusLive
)

type User struct {
	ID        primitive.ObjectID   `json:"id" bson:"_id,omitempty"`
	Email     string               `json:"email" bson:"email"`
	Rank      int32                `json:"rank" bson:"rank"`
	EmoteIDs  []primitive.ObjectID `json:"emote_ids" bson:"emotes"`
	EditorIDs []primitive.ObjectID `json:"editor_ids" bson:"editors"`
	Role      *primitive.ObjectID  `json:"role" bson:"role"`

	TwitchID        string `json:"twitch_id" bson:"id"`
	DisplayName     string `json:"display_name" bson:"display_name"`
	Login           string `json:"login" bson:"login"`
	ProfileImageURL string `json:"profile_image_url" bson:"profile_image_url"`

	// Relational Data
	Emotes       *[]*Emote    `json:"emotes" bson:"-"`
	OwnedEmotes  *[]*Emote    `json:"owned_emotes" bson:"-"`
	Editors      *[]*User     `json:"editors" bson:"-"`
	EditorIn     *[]*User     `json:"editor_in" bson:"-"`
	AuditEntries *[]*AuditLog `json:"audit_entries" bson:"-"`
	Reports      *[]*Report   `json:"reports" bson:"-"`
	Bans         *[]*Ban      `json:"bans" bson:"-"`
}

type Role struct {
	ID      primitive.ObjectID `json:"id" bson:"_id"`
	Name    string             `json:"name" bson:"name"`
	Color   int32              `json:"color" bson:"color"`
	Allowed int64              `json:"allowed" bson:"allowed"`
	Denied  int64              `json:"denied" bson:"denied"`
}

const (
	RolePermissionEmoteUpload  int64 = 2 << iota // 1
	RolePermissionEmoteDelete  int64 = 2 << iota // 4
	RolePermissionEmoteRestore int64 = 2 << iota // 8
	RolePermissionEmoteEdit    int64 = 2 << iota // 16
	UserPermissionEmoteGlobal  int64 = 2 << iota // 64
	UserPermissionReport       int64 = 2 << iota // 128
	UserPermissionBan          int64 = 2 << iota // 256
	UserPermissionManageOthers int64 = 2 << iota // 1024
	UserPermissionEditorChange int64 = 2 << iota // 2048
	RolePermissionRolesEdit    int64 = 2 << iota // 8192
	UserPermissionAll          int64 = (2 << iota) - 1
	UserPermissionsDefault     int64 = RolePermissionEmoteUpload & RolePermissionEmoteEdit & RolePermissionEmoteDelete & UserPermissionReport & RolePermissionEmoteRestore
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
	Changes   []*AutitLogChange  `json:"changes" bson:"changes"`
	Reason    *string            `json:"reason" bson:"reason"`
	CreatedBy primitive.ObjectID `json:"created_by" bson:"created_by"`
}

type Target struct {
	ID   *primitive.ObjectID `json:"id" bson:"id"`
	Type string              `json:"type" bson:"type"`
}

type AutitLogChange struct {
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
