package query_resolvers

import (
	"context"
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	"github.com/SevenTV/ServerGo/src/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	log "github.com/sirupsen/logrus"
)

type EmoteResolver struct {
	ctx context.Context
	v   *datastructure.Emote

	fields map[string]*SelectedField
}

func GenerateEmoteResolver(ctx context.Context, emote *datastructure.Emote, emoteID *primitive.ObjectID, fields map[string]*SelectedField) (*EmoteResolver, error) {
	if emote == nil {
		emote = &datastructure.Emote{}
		if err := cache.FindOne("emotes", "", bson.M{
			"_id": emoteID,
		}, emote); err != nil {
			if err != mongo.ErrNoDocuments {
				log.Errorf("mongo, err=%v", err)
				return nil, resolvers.ErrInternalServer
			}
			return nil, nil
		}
	}

	if emote == nil {
		return nil, nil
	}

	if emote.AuditEntries == nil {
		if _, ok := fields["audit_entries"]; ok {
			emote.AuditEntries = &[]*datastructure.AuditLog{}
			if err := cache.Find("audit", fmt.Sprintf("logs:%s", emote.ID.Hex()), bson.M{
				"target.id":   emote.ID,
				"target.type": "emotes",
			}, emote.AuditEntries); err != nil {
				log.Errorf("mongo, err=%v", err)
				return nil, resolvers.ErrInternalServer
			}
		}
	}

	if emote.Channels == nil {
		if _, ok := fields["channels"]; ok {
			emote.Channels = &[]*datastructure.User{}

			if err := cache.Find("users", fmt.Sprintf("emotes:%s", emote.ID.Hex()), bson.M{
				"emotes": bson.M{
					"$in": []primitive.ObjectID{emote.ID},
				},
			}, emote.Channels); err != nil {
				return nil, resolvers.ErrInternalServer
			}
		}
	}

	usr, usrValid := ctx.Value(utils.UserKey).(*datastructure.User)
	if v, ok := fields["reports"]; ok && usrValid && (usr.Rank != datastructure.UserRankAdmin && usr.Rank != datastructure.UserRankModerator) && emote.Reports == nil {
		emote.Reports = &[]*datastructure.Report{}
		if err := cache.Find("reports", fmt.Sprintf("reports:%s", emote.ID.Hex()), bson.M{
			"target.id":   emote.ID,
			"target.type": "emotes",
		}, emote.Reports); err != nil {
			log.Errorf("mongo, err=%v", err)
			return nil, resolvers.ErrInternalServer
		}

		_, query := v.Children["reporter"]

		reports := *emote.Reports
		reportMap := map[primitive.ObjectID][]*datastructure.Report{}
		for _, r := range reports {
			r.ETarget = emote
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

	if _, ok := fields["provider"]; ok && emote.Provider == "" {
		emote.Provider = "7TV"
	}

	r := &EmoteResolver{
		ctx:    ctx,
		v:      emote,
		fields: fields,
	}
	return r, nil
}

func (r *EmoteResolver) ID() string {
	if r.v.ID.IsZero() {
		return utils.Ternary(r.v.ProviderID != nil, *r.v.ProviderID, "").(string)
	}

	return r.v.ID.Hex()
}
func (r *EmoteResolver) Name() string {
	return r.v.Name
}
func (r *EmoteResolver) OwnerID() string {
	return r.v.OwnerID.Hex()
}
func (r *EmoteResolver) Visibility() int32 {
	return r.v.Visibility
}
func (r *EmoteResolver) Mime() string {
	return r.v.Mime
}
func (r *EmoteResolver) Status() int32 {
	return r.v.Status
}

func (r *EmoteResolver) Tags() []string {
	return r.v.Tags
}

func (r *EmoteResolver) CreatedAt() string {
	return r.v.ID.Timestamp().Format(time.RFC3339)
}

func (r *EmoteResolver) ChannelCount() int32 {
	return *r.v.ChannelCount
}

func (r *EmoteResolver) Owner() (*UserResolver, error) {
	resolver, err := GenerateUserResolver(r.ctx, r.v.Owner, &r.v.OwnerID, r.fields["owner"].Children)
	if err != nil {
		return nil, err
	}
	if resolver == nil {
		return GenerateUserResolver(r.ctx, datastructure.DeletedUser, nil, nil)
	}

	return resolver, nil
}

func (r *EmoteResolver) AuditEntries() ([]string, error) {
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
	return logs, nil
}

func (r *EmoteResolver) Channels() (*[]*UserResolver, error) {
	if r.v.Channels == nil {
		return nil, nil
	}

	u := *r.v.Channels
	users := make([]*UserResolver, len(u))
	for i, usr := range u {
		resolver, err := GenerateUserResolver(r.ctx, usr, &usr.ID, nil)
		if err != nil {
			return nil, err
		}

		users[i] = resolver
	}

	return &users, nil
}

func (r *EmoteResolver) Reports() (*[]*reportResolver, error) {
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

func (r *EmoteResolver) Provider() string {
	return r.v.Provider
}

func (r *EmoteResolver) ProviderID() *string {
	return r.v.ProviderID
}

func (r *EmoteResolver) URLs() [][]string {
	result := make([][]string, 4) // 4 length because there are 4 CDN sizes supported (1x, 2x, 3x, 4x)

	if r.v.Provider == "7TV" { // Provider is 7TV: append URLs
		for i := 1; i <= 4; i++ {
			a := make([]string, 2)
			a[0] = fmt.Sprintf("%d", i)
			a[1] = utils.GetCdnURL(r.v.ID.Hex(), int8(i))

			result[i-1] = a
		}

		r.v.URLs = result
	} else if r.v.URLs == nil { // Provider is null: send empty array
		return [][]string{}
	}

	return r.v.URLs
}

func (r *EmoteResolver) Width() []int32 {
	result := make([]int32, 4)
	for i, v := range r.v.Width {
		result[i] = int32(v)
	}

	return result
}

func (r *EmoteResolver) Height() []int32 {
	result := make([]int32, 4)
	for i, v := range r.v.Height {
		result[i] = int32(v)
	}

	return result
}
