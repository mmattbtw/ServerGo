package resolvers

import (
	"context"
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/cache"
	"github.com/SevenTV/ServerGo/mongo"
	"github.com/SevenTV/ServerGo/utils"
	"go.mongodb.org/mongo-driver/bson"

	"go.mongodb.org/mongo-driver/bson/primitive"

	log "github.com/sirupsen/logrus"
)

type userResolver struct {
	ctx context.Context
	v   *mongo.User

	fields map[string]*SelectedField
}

func GenerateUserResolver(ctx context.Context, user *mongo.User, userID *primitive.ObjectID, fields map[string]*SelectedField) (*userResolver, error) {
	if user == nil {
		user = &mongo.User{
			Role: mongo.DefaultRole,
		}
		if err := cache.FindOne("users", "", bson.M{
			"_id": userID,
		}, user); err != nil {
			if err != mongo.ErrNoDocuments {
				log.Errorf("mongo, err=%v", err)
				return nil, errInternalServer
			}
			return nil, nil
		}
	}

	if user == nil {
		return nil, nil
	}

	if _, ok := fields["role"]; ok && user.Role == nil {
		role := &[]*mongo.Role{}
		if err := cache.Find("users", fmt.Sprintf("user:%s:role", userID.Hex()), bson.M{
			"_id": user.RoleID,
		}, role); err != nil {
			log.Errorf("mongo, err=%v", err)
			return nil, errInternalServer
		}

		if len(*role) > 0 {
			user.Role = (*role)[0]
		}
		if user.Role == nil { // Resolve default role if no role assigned
			user.Role = mongo.DefaultRole
		}
	}

	if v, ok := fields["owned_emotes"]; ok && user.OwnedEmotes == nil {
		user.OwnedEmotes = &[]*mongo.Emote{}
		if err := cache.Find("emotes", fmt.Sprintf("owner:%s", userID.Hex()), bson.M{
			"owner": userID,
		}, user.OwnedEmotes); err != nil {
			log.Errorf("mongo, err=%v", err)
			return nil, errInternalServer
		}
		ems := *user.OwnedEmotes
		ids := make([]primitive.ObjectID, len(ems))
		emotes := make(map[primitive.ObjectID]*mongo.Emote, len(ems))
		for i, e := range ems {
			e.Owner = user
			ids[i] = e.ID
			emotes[e.ID] = e
		}
		if _, ok := v.children["audit_entries"]; ok {
			logs := []*mongo.AuditLog{}
			if err := cache.Find("logs", fmt.Sprintf("user:%s:owned_emotes", userID.Hex()), bson.M{
				"target.id": bson.M{
					"$in": ids,
				},
				"target.type": "emotes",
			}, &logs); err != nil {
				log.Errorf("mongo, err=%v", err)
				return nil, errInternalServer
			}

			for _, l := range logs {
				e := emotes[*l.Target.ID]
				var d []*mongo.AuditLog
				if e.AuditEntries != nil {
					d = *e.AuditEntries
				}
				d = append(d, l)
				e.AuditEntries = &d
			}
		}
	}

	if v, ok := fields["emotes"]; ok && user.Emotes == nil {
		if len(user.EmoteIDs) == 0 {
			user.Emotes = &[]*mongo.Emote{}
		} else {
			user.Emotes = &[]*mongo.Emote{}
			if err := cache.Find("emotes", fmt.Sprintf("user:%s:emotes", userID.Hex()), bson.M{
				"_id": bson.M{
					"$in": user.EmoteIDs,
				},
			}, user.Emotes); err != nil {
				log.Errorf("mongo, err=%v", err)
				return nil, errInternalServer
			}
			ems := *user.Emotes
			ids := make([]primitive.ObjectID, len(ems))
			emotes := make(map[primitive.ObjectID]*mongo.Emote, len(ems))
			for i, e := range ems {
				e.Owner = user
				ids[i] = e.ID
				emotes[e.ID] = e
			}
			if _, ok := v.children["audit_entries"]; ok {
				logs := []*mongo.AuditLog{}
				if err := cache.Find("logs", "", bson.M{
					"target.id": bson.M{
						"$in": ids,
					},
					"target.type": "emotes",
				}, &logs); err != nil {
					log.Errorf("mongo, err=%v", err)
					return nil, errInternalServer
				}
				for _, l := range logs {
					e := emotes[*l.Target.ID]
					var d []*mongo.AuditLog
					if e.AuditEntries != nil {
						d = *e.AuditEntries
					}
					d = append(d, l)
					e.AuditEntries = &d
				}
			}
		}
	}

	if _, ok := fields["editors"]; ok && user.Editors == nil {
		user.Editors = &[]*mongo.User{}
		if err := cache.Find("users", fmt.Sprintf("user:%s:editors", userID.Hex()), bson.M{
			"_id": bson.M{
				"$in": utils.Ternary(len(user.EditorIDs) > 0, user.EditorIDs, []primitive.ObjectID{}).([]primitive.ObjectID),
			},
		}, user.Editors); err != nil {
			log.Errorf("mongo, err=%v", err)
			return nil, errInternalServer
		}
	}

	if _, ok := fields["editor_in"]; ok && user.EditorIn == nil {
		user.EditorIn = &[]*mongo.User{}
		if err := cache.Find("users", fmt.Sprintf("user:%s:editor_in", userID.Hex()), bson.M{
			"editors": user.ID,
		}, user.EditorIn); err != nil {
			log.Errorf("mongo, err=%v", err)
			return nil, errInternalServer
		}
	}

	usr, usrValid := ctx.Value(utils.UserKey).(*mongo.User)
	if v, ok := fields["reports"]; ok && usrValid && (usr.Rank != mongo.UserRankAdmin && usr.Rank != mongo.UserRankModerator) && user.Reports == nil {
		user.Reports = &[]*mongo.Report{}
		if err := cache.Find("users", fmt.Sprintf("user:%s:reports", userID.Hex()), bson.M{
			"target.id":   user.ID,
			"target.type": "users",
		}, user.EditorIn); err != nil {
			log.Errorf("mongo, err=%v", err)
			return nil, errInternalServer
		}

		_, query := v.children["reporter"]

		reports := *user.Reports
		reportMap := map[primitive.ObjectID][]*mongo.Report{}
		for _, r := range reports {
			r.UTarget = user
			if r.ReporterID != nil && query {
				reportMap[*r.ReporterID] = append(reportMap[*r.ReporterID], r)
			}
		}
		if query {
			ids := []primitive.ObjectID{}
			for k := range reportMap {
				ids = append(ids, k)
			}
			reporters := []*mongo.User{}

			if err := cache.Find("users", "", bson.M{
				"_id": bson.M{
					"$in": ids,
				},
			}, &reporters); err != nil {
				log.Errorf("mongo, err=%v", err)
				return nil, errInternalServer
			}
			for _, u := range reporters {
				for _, r := range reportMap[u.ID] {
					r.Reporter = u
				}
			}
		}
	}

	r := &userResolver{
		ctx:    ctx,
		v:      user,
		fields: fields,
	}
	return r, nil
}

func (r *userResolver) ID() string {
	return r.v.ID.Hex()
}

func (r *userResolver) Email() *string {
	if u, ok := r.ctx.Value(utils.UserKey).(*mongo.User); ok && (r.v.ID == u.ID || HasPermission(u, mongo.RolePermissionManageUsers)) {
		return &r.v.Email
	} else { // Hide the email address if
		s := "<hidden>"
		return &s
	}
}

func (r *userResolver) Rank() int32 {
	return r.v.Rank
}

func (r *userResolver) Role() (*roleResolver, error) {
	role := r.v.RoleID
	res, err := GenerateRoleResolver(r.ctx, nil, role, r.fields["role"].children)
	if err != nil {
		log.Errorf("generation, err=%v", err)
		return nil, errInternalServer
	}

	if res == nil {
		return GenerateRoleResolver(r.ctx, mongo.DefaultRole, nil, r.fields["role"].children)
	}

	return res, nil
}

func (r *userResolver) EmoteIDs() []string {
	ids := make([]string, len(r.v.EmoteIDs))
	for i, id := range r.v.EmoteIDs {
		ids[i] = id.Hex()
	}
	return ids
}

func (r *userResolver) EditorIDs() []string {
	ids := make([]string, len(r.v.EditorIDs))
	for i, id := range r.v.EditorIDs {
		ids[i] = id.Hex()
	}
	return ids
}

func (r *userResolver) CreatedAt() string {
	return r.v.ID.Timestamp().Format(time.RFC3339)
}

func (r *userResolver) Editors() ([]*userResolver, error) {
	editors := *r.v.Editors
	resolvers := []*userResolver{}
	for _, e := range editors {
		r, err := GenerateUserResolver(r.ctx, e, nil, r.fields["editors"].children)
		if err != nil {
			log.Errorf("generation, err=%v", err)
			return nil, errInternalServer
		}
		if r != nil {
			resolvers = append(resolvers, r)
		}
	}
	return resolvers, nil
}

func (r *userResolver) EditorIn() ([]*userResolver, error) {
	editors := *r.v.EditorIn
	resolvers := []*userResolver{}
	for _, e := range editors {
		r, err := GenerateUserResolver(r.ctx, e, nil, r.fields["editor_in"].children)
		if err != nil {
			log.Errorf("generation, err=%v", err)
			return nil, errInternalServer
		}
		if r != nil {
			resolvers = append(resolvers, r)
		}
	}
	return resolvers, nil
}

func (r *userResolver) Emotes() ([]*emoteResolver, error) {
	emotes := *r.v.Emotes
	resolvers := []*emoteResolver{}
	for _, e := range emotes {
		r, err := GenerateEmoteResolver(r.ctx, e, nil, r.fields["emotes"].children)
		if err != nil {
			log.Errorf("generation, err=%v", err)
			return nil, errInternalServer
		}
		if r != nil {
			resolvers = append(resolvers, r)
		}
	}
	return resolvers, nil
}

func (r *userResolver) OwnedEmotes() ([]*emoteResolver, error) {
	emotes := *r.v.OwnedEmotes
	resolvers := []*emoteResolver{}
	for _, e := range emotes {
		r, err := GenerateEmoteResolver(r.ctx, e, nil, r.fields["owned_emotes"].children)
		if err != nil {
			log.Errorf("generation, err=%v", err)
			return nil, errInternalServer
		}
		if r != nil {
			resolvers = append(resolvers, r)
		}
	}
	return resolvers, nil
}

func (r *userResolver) TwitchID() string {
	return r.v.TwitchID
}

func (r *userResolver) DisplayName() string {
	return r.v.DisplayName
}

func (r *userResolver) Login() string {
	return r.v.Login
}

func (r *userResolver) BroadcasterType() string {
	return r.v.BroadcasterType
}

func (r *userResolver) ProfileImageURL() string {
	return r.v.ProfileImageURL
}

func (r *userResolver) PairedAt() string {
	return r.v.ID.Timestamp().Format(time.RFC3339)
}

func (r *userResolver) Reports() (*[]*reportResolver, error) {
	u, ok := r.ctx.Value(utils.UserKey).(*mongo.User)
	if !ok || (u.Rank != mongo.UserRankAdmin && u.Rank != mongo.UserRankModerator) {
		return nil, errAccessDenied
	}

	if r.v.Reports == nil {
		return nil, nil
	}

	e := *r.v.Reports
	reports := make([]*reportResolver, len(e))
	var err error
	for i, l := range e {
		reports[i], err = GenerateReportResolver(r.ctx, l, r.fields["reports"].children)
		if err != nil {
			return nil, err
		}
	}
	return &reports, nil
}

func (r *userResolver) Bans() (*[]*banResolver, error) {
	u, ok := r.ctx.Value(utils.UserKey).(*mongo.User)
	if !ok || (u.Rank != mongo.UserRankAdmin && u.Rank != mongo.UserRankModerator) {
		return nil, errAccessDenied
	}

	if r.v.Bans == nil {
		return nil, nil
	}

	e := *r.v.Bans
	bans := make([]*banResolver, len(e))
	var err error
	for i, l := range e {
		bans[i], err = GenerateBanResolver(r.ctx, l, r.fields["bans"].children)
		if err != nil {
			return nil, err
		}
	}
	return &bans, nil
}

func (r *userResolver) AuditEntries() (*[]string, error) {
	u, ok := r.ctx.Value(utils.UserKey).(*mongo.User)
	if !ok || (u.Rank != mongo.UserRankAdmin && u.Rank != mongo.UserRankModerator) {
		return nil, errAccessDenied
	}

	if r.v.AuditEntries == nil {
		return nil, nil
	}
	e := *r.v.AuditEntries
	logs := make([]string, len(e))
	var err error
	for i, l := range e {
		logs[i], err = json.MarshalToString(l)
		if err != nil {
			return nil, err
		}
	}
	return &logs, nil
}

// Test whether a User has a permission flag
func HasPermission(user *mongo.User, flag int64) bool {
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
	return utils.HasBits(sum, flag) || utils.HasBits(sum, mongo.RolePermissionAdministrator)
}
