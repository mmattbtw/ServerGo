package query_resolvers

import (
	"context"
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	api_proxy "github.com/SevenTV/ServerGo/src/server/api/v2/proxy"
	"github.com/SevenTV/ServerGo/src/utils"
	"go.mongodb.org/mongo-driver/bson"

	"go.mongodb.org/mongo-driver/bson/primitive"

	log "github.com/sirupsen/logrus"
)

type UserResolver struct {
	ctx context.Context
	v   *datastructure.User

	fields map[string]*SelectedField
}

func GenerateUserResolver(ctx context.Context, user *datastructure.User, userID *primitive.ObjectID, fields map[string]*SelectedField) (*UserResolver, error) {
	if user == nil || user.Login == "" {
		user = &datastructure.User{
			Role: datastructure.DefaultRole,
		}
		if err := cache.FindOne("users", "", bson.M{
			"_id": userID,
		}, user); err != nil {
			if err != mongo.ErrNoDocuments {
				log.Errorf("mongo, err=%v", err)
				return nil, resolvers.ErrInternalServer
			}
			return nil, nil
		}
	}

	if user == nil {
		return nil, nil
	}

	if v, ok := fields["owned_emotes"]; ok && user.OwnedEmotes == nil {
		user.OwnedEmotes = &[]*datastructure.Emote{}
		if err := cache.Find("emotes", fmt.Sprintf("owner:%s", user.ID.Hex()), bson.M{
			"owner": user.ID,
		}, user.OwnedEmotes); err != nil {
			log.Errorf("mongo, err=%v", err)
			return nil, resolvers.ErrInternalServer
		}
		ems := *user.OwnedEmotes
		ids := make([]primitive.ObjectID, len(ems))
		emotes := make(map[primitive.ObjectID]*datastructure.Emote, len(ems))
		for i, e := range ems {
			e.Owner = user
			ids[i] = e.ID
			emotes[e.ID] = e
		}
		if _, ok := v.Children["audit_entries"]; ok {
			logs := []*datastructure.AuditLog{}
			if err := cache.Find("audit", fmt.Sprintf("user:%s:owned_emotes", user.ID.Hex()), bson.M{
				"target.id": bson.M{
					"$in": ids,
				},
				"target.type": "emotes",
			}, &logs); err != nil {
				log.Errorf("mongo, err=%v", err)
				return nil, resolvers.ErrInternalServer
			}

			for _, l := range logs {
				e := emotes[*l.Target.ID]
				var d []*datastructure.AuditLog
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
			user.Emotes = &[]*datastructure.Emote{}
		} else {
			user.Emotes = &[]*datastructure.Emote{}
			if err := cache.Find("emotes", fmt.Sprintf("user:%s:emotes", user.ID.Hex()), bson.M{
				"_id": bson.M{
					"$in": user.EmoteIDs,
				},
			}, user.Emotes); err != nil {
				log.Errorf("mongo, err=%v", err)
				return nil, resolvers.ErrInternalServer
			}
			ems := *user.Emotes
			ids := make([]primitive.ObjectID, len(ems))
			emotes := make(map[primitive.ObjectID]*datastructure.Emote, len(ems))
			for i, e := range ems {
				e.Owner = user
				ids[i] = e.ID
				emotes[e.ID] = e
			}
			if _, ok := v.Children["audit_entries"]; ok {
				logs := []*datastructure.AuditLog{}
				if err := cache.Find("audit", "", bson.M{
					"target.id": bson.M{
						"$in": ids,
					},
					"target.type": "emotes",
				}, &logs); err != nil {
					log.Errorf("mongo, err=%v", err)
					return nil, resolvers.ErrInternalServer
				}
				for _, l := range logs {
					e := emotes[*l.Target.ID]
					var d []*datastructure.AuditLog
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
		user.Editors = &[]*datastructure.User{}
		if err := cache.Find("users", fmt.Sprintf("user:%s:editors", user.ID.Hex()), bson.M{
			"_id": bson.M{
				"$in": utils.Ternary(len(user.EditorIDs) > 0, user.EditorIDs, []primitive.ObjectID{}).([]primitive.ObjectID),
			},
		}, user.Editors); err != nil {
			log.Errorf("mongo, err=%v", err)
			return nil, resolvers.ErrInternalServer
		}
	}

	if _, ok := fields["editor_in"]; ok && user.EditorIn == nil {
		user.EditorIn = &[]*datastructure.User{}
		if err := cache.Find("users", fmt.Sprintf("user:%s:editor_in", user.ID.Hex()), bson.M{
			"editors": user.ID,
		}, user.EditorIn); err != nil {
			log.Errorf("mongo, err=%v", err)
			return nil, resolvers.ErrInternalServer
		}
	}

	usr, usrValid := ctx.Value(utils.UserKey).(*datastructure.User)
	if v, ok := fields["reports"]; ok && usrValid && (usr.Rank != datastructure.UserRankAdmin && usr.Rank != datastructure.UserRankModerator) && user.Reports == nil {
		user.Reports = &[]*datastructure.Report{}
		if err := cache.Find("users", fmt.Sprintf("user:%s:reports", user.ID.Hex()), bson.M{
			"target.id":   user.ID,
			"target.type": "users",
		}, user.EditorIn); err != nil {
			log.Errorf("mongo, err=%v", err)
			return nil, resolvers.ErrInternalServer
		}

		_, query := v.Children["reporter"]

		reports := *user.Reports
		reportMap := map[primitive.ObjectID][]*datastructure.Report{}
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
			reporters := []*datastructure.User{}

			if err := cache.Find("users", "", bson.M{
				"_id": bson.M{
					"$in": ids,
				},
			}, &reporters); err != nil {
				log.Errorf("mongo, err=%v", err)
				return nil, resolvers.ErrInternalServer
			}
			for _, u := range reporters {
				for _, r := range reportMap[u.ID] {
					r.Reporter = u
				}
			}
		}
	}

	r := &UserResolver{
		ctx:    ctx,
		v:      user,
		fields: fields,
	}
	return r, nil
}

func (r *UserResolver) ID() string {
	if r.v.ID.IsZero() {
		return ""
	}

	return r.v.ID.Hex()
}

func (r *UserResolver) Email() *string {
	if u, ok := r.ctx.Value(utils.UserKey).(*datastructure.User); ok && (r.v.ID == u.ID || u.HasPermission(datastructure.RolePermissionManageUsers)) {
		return &r.v.Email
	} else { // Hide the email address if
		s := "<hidden>"
		return &s
	}
}

func (r *UserResolver) Rank() int32 {
	return r.v.Rank
}

func (r *UserResolver) Role() (*RoleResolver, error) {
	roleID := r.v.RoleID
	role := datastructure.GetRole(mongo.Ctx, roleID)

	res, err := GenerateRoleResolver(r.ctx, &role, roleID, nil)
	if err != nil {
		log.Errorf("generation, err=%v", err)
		return nil, resolvers.ErrInternalServer
	}

	if res == nil {
		return GenerateRoleResolver(r.ctx, datastructure.DefaultRole, nil, r.fields["role"].Children)
	}

	return res, nil
}

func (r *UserResolver) EmoteIDs() []string {
	ids := make([]string, len(r.v.EmoteIDs))
	for i, id := range r.v.EmoteIDs {
		ids[i] = id.Hex()
	}
	return ids
}

func (r *UserResolver) EditorIDs() []string {
	ids := make([]string, len(r.v.EditorIDs))
	for i, id := range r.v.EditorIDs {
		ids[i] = id.Hex()
	}
	return ids
}

func (r *UserResolver) CreatedAt() string {
	return r.v.ID.Timestamp().Format(time.RFC3339)
}

func (r *UserResolver) Editors() ([]*UserResolver, error) {
	editors := *r.v.Editors
	result := []*UserResolver{}
	for _, e := range editors {
		r, err := GenerateUserResolver(r.ctx, e, nil, r.fields["editors"].Children)
		if err != nil {
			log.Errorf("generation, err=%v", err)
			return nil, resolvers.ErrInternalServer
		}
		if r != nil {
			result = append(result, r)
		}
	}
	return result, nil
}

func (r *UserResolver) EditorIn() ([]*UserResolver, error) {
	editors := *r.v.EditorIn
	result := []*UserResolver{}
	for _, e := range editors {
		r, err := GenerateUserResolver(r.ctx, e, nil, r.fields["editor_in"].Children)
		if err != nil {
			log.Errorf("generation, err=%v", err)
			return nil, resolvers.ErrInternalServer
		}
		if r != nil {
			result = append(result, r)
		}
	}
	return result, nil
}

func (r *UserResolver) Emotes() ([]*EmoteResolver, error) {
	if r.v.Emotes == nil {
		return nil, nil
	}
	emotes := *r.v.Emotes
	result := []*EmoteResolver{}
	for _, e := range emotes {
		r, err := GenerateEmoteResolver(r.ctx, e, nil, r.fields["emotes"].Children)
		if err != nil {
			log.Errorf("generation, err=%v", err)
			return nil, resolvers.ErrInternalServer
		}
		if r != nil {
			result = append(result, r)
		}
	}
	return result, nil
}

func (r *UserResolver) OwnedEmotes() ([]*EmoteResolver, error) {
	emotes := *r.v.OwnedEmotes
	result := []*EmoteResolver{}
	for _, e := range emotes {
		r, err := GenerateEmoteResolver(r.ctx, e, nil, r.fields["owned_emotes"].Children)
		if err != nil {
			log.Errorf("generation, err=%v", err)
			return nil, resolvers.ErrInternalServer
		}
		if r != nil {
			result = append(result, r)
		}
	}
	return result, nil
}

func (r *UserResolver) ThirdPartyEmotes() ([]*EmoteResolver, error) {
	var emotes []*datastructure.Emote
	if bttv, err := api_proxy.GetChannelEmotesBTTV(r.v.Login); err == nil { // Find channel bttv emotes
		emotes = append(emotes, bttv...)
	}
	if ffz, err := api_proxy.GetChannelEmotesFFZ(r.v.Login); err == nil { // Find channel FFZ emotes
		emotes = append(emotes, ffz...)
	}

	result := make([]*EmoteResolver, len(emotes))
	for i, emote := range emotes {
		resolver, _ := GenerateEmoteResolver(r.ctx, emote, nil, r.fields["third_party_emotes"].Children)
		if resolver == nil {
			continue
		}

		result[i] = resolver
	}

	return result, nil
}

func (r *UserResolver) TwitchID() string {
	return r.v.TwitchID
}

func (r *UserResolver) DisplayName() string {
	return r.v.DisplayName
}

func (r *UserResolver) Login() string {
	return r.v.Login
}

func (r *UserResolver) BroadcasterType() string {
	return r.v.BroadcasterType
}

func (r *UserResolver) ProfileImageURL() string {
	return r.v.ProfileImageURL
}

func (r *UserResolver) Reports() (*[]*reportResolver, error) {
	u, ok := r.ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok || (u.Rank != datastructure.UserRankAdmin && u.Rank != datastructure.UserRankModerator) {
		return nil, resolvers.ErrAccessDenied
	}

	if r.v.Reports == nil {
		return nil, nil
	}

	e := *r.v.Reports
	reports := make([]*reportResolver, len(e))
	var err error
	for i, l := range e {
		reports[i], err = GenerateReportResolver(r.ctx, l, r.fields["reports"].Children)
		if err != nil {
			return nil, err
		}
	}
	return &reports, nil
}

func (r *UserResolver) Bans() (*[]*banResolver, error) {
	u, ok := r.ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok || (u.Rank != datastructure.UserRankAdmin && u.Rank != datastructure.UserRankModerator) {
		return nil, resolvers.ErrAccessDenied
	}

	if r.v.Bans == nil {
		return nil, nil
	}

	e := *r.v.Bans
	bans := make([]*banResolver, len(e))
	var err error
	for i, l := range e {
		bans[i], err = GenerateBanResolver(r.ctx, l, r.fields["bans"].Children)
		if err != nil {
			return nil, err
		}
	}
	return &bans, nil
}

func (r *UserResolver) AuditEntries() (*[]string, error) {
	u, ok := r.ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok || (u.Rank != datastructure.UserRankAdmin && u.Rank != datastructure.UserRankModerator) {
		return nil, resolvers.ErrAccessDenied
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
