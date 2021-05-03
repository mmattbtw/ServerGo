package resolvers

import (
	"context"
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	log "github.com/sirupsen/logrus"
)

type emoteResolver struct {
	ctx context.Context
	v   *mongo.Emote

	fields map[string]*SelectedField
}

func GenerateEmoteResolver(ctx context.Context, emote *mongo.Emote, emoteID *primitive.ObjectID, fields map[string]*SelectedField) (*emoteResolver, error) {
	if emote == nil {
		emote = &mongo.Emote{}
		if err := cache.FindOne("emotes", "", bson.M{
			"_id": emoteID,
		}, emote); err != nil {
			if err != mongo.ErrNoDocuments {
				log.Errorf("mongo, err=%v", err)
				return nil, errInternalServer
			}
			return nil, nil
		}
	}

	if emote == nil {
		return nil, nil
	}

	if emote.AuditEntries == nil {
		if _, ok := fields["audit_entries"]; ok {
			emote.AuditEntries = &[]*mongo.AuditLog{}
			if err := cache.Find("audit", fmt.Sprintf("logs:%s", emote.ID.Hex()), bson.M{
				"target.id":   emote.ID,
				"target.type": "emotes",
			}, emote.AuditEntries); err != nil {
				log.Errorf("mongo, err=%v", err)
				return nil, errInternalServer
			}
		}
	}

	if emote.Channels == nil {
		if _, ok := fields["channels"]; ok {
			emote.Channels = &[]*mongo.User{}

			if err := cache.Find("users", fmt.Sprintf("emotes:%s", emote.ID.Hex()), bson.M{
				"emotes": bson.M{
					"$in": []primitive.ObjectID{emote.ID},
				},
			}, emote.Channels); err != nil {
				return nil, errInternalServer
			}
		}
	}

	usr, usrValid := ctx.Value(utils.UserKey).(*mongo.User)
	if v, ok := fields["reports"]; ok && usrValid && (usr.Rank != mongo.UserRankAdmin && usr.Rank != mongo.UserRankModerator) && emote.Reports == nil {
		emote.Reports = &[]*mongo.Report{}
		if err := cache.Find("reports", fmt.Sprintf("reports:%s", emote.ID.Hex()), bson.M{
			"target.id":   emote.ID,
			"target.type": "emotes",
		}, emote.Reports); err != nil {
			log.Errorf("mongo, err=%v", err)
			return nil, errInternalServer
		}

		_, query := v.children["reporter"]

		reports := *emote.Reports
		reportMap := map[primitive.ObjectID][]*mongo.Report{}
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

	if _, ok := fields["provider"]; ok && emote.Provider == "" {
		emote.Provider = "7TV"
	}

	r := &emoteResolver{
		ctx:    ctx,
		v:      emote,
		fields: fields,
	}
	return r, nil
}

func (r *emoteResolver) ID() string {
	if r.v.ID.IsZero() {
		return utils.Ternary(r.v.ProviderID != nil, *r.v.ProviderID, "").(string)
	}

	return r.v.ID.Hex()
}
func (r *emoteResolver) Name() string {
	return r.v.Name
}
func (r *emoteResolver) OwnerID() string {
	return r.v.OwnerID.Hex()
}
func (r *emoteResolver) Visibility() int32 {
	return r.v.Visibility
}
func (r *emoteResolver) Mime() string {
	return r.v.Mime
}
func (r *emoteResolver) Status() int32 {
	return r.v.Status
}

func (r *emoteResolver) Tags() []string {
	return r.v.Tags
}

func (r *emoteResolver) CreatedAt() string {
	return r.v.ID.Timestamp().Format(time.RFC3339)
}

func (r *emoteResolver) Owner() (*userResolver, error) {
	if r.v.Owner != nil {
		resolver, err := GenerateUserResolver(r.ctx, r.v.Owner, nil, r.fields["owner"].children)
		if err != nil {
			return nil, err
		}
		return resolver, nil
	}
	resolver, err := GenerateUserResolver(r.ctx, nil, &r.v.OwnerID, r.fields["owner"].children)
	if err != nil {
		return nil, err
	}
	if resolver == nil {
		return GenerateUserResolver(r.ctx, mongo.DeletedUser, nil, nil)
	}

	return resolver, nil
}

func (r *emoteResolver) AuditEntries() ([]string, error) {
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

func (r *emoteResolver) Channels() (*[]*userResolver, error) {
	if r.v.Channels == nil {
		return nil, nil
	}

	u := *r.v.Channels
	users := make([]*userResolver, len(u))
	for i, usr := range u {
		resolver, err := GenerateUserResolver(r.ctx, usr, &usr.ID, nil)
		if err != nil {
			return nil, err
		}

		users[i] = resolver
	}

	return &users, nil
}

func (r *emoteResolver) Reports() (*[]*reportResolver, error) {
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

func (r *emoteResolver) Provider() string {
	return r.v.Provider
}

func (r *emoteResolver) ProviderID() *string {
	return r.v.ProviderID
}

func (r *emoteResolver) URLs() *[]*[]*string {
	result := make([]*[]*string, 4) // 4 length because there are 4 CDN sizes supported (1x, 2x, 3x, 4x)

	if r.v.Provider == "7TV" { // Provider is 7TV: append URLs
		for i := 1; i <= 4; i++ {
			a := make([]*string, 2)
			a[0] = utils.StringPointer(fmt.Sprintf("%d", i))
			a[1] = utils.StringPointer(utils.GetCdnURL(r.v.ID.Hex(), int8(i)))

			result[i-1] = &a
		}

		r.v.URLs = &result
	} else if r.v.URLs == nil { // Provider is null: send empty array
		return &[]*[]*string{}
	}

	return r.v.URLs
}
