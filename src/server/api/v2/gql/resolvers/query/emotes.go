package query_resolvers

import (
	"context"
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type EmoteResolver struct {
	ctx context.Context
	v   *datastructure.Emote

	fields map[string]*SelectedField
}

func GenerateEmoteResolver(ctx context.Context, emote *datastructure.Emote, emoteID *primitive.ObjectID, fields map[string]*SelectedField) (*EmoteResolver, error) {
	if emote == nil {
		emote = &datastructure.Emote{}
		res := mongo.Collection(mongo.CollectionNameEmotes).FindOne(ctx, bson.M{
			"_id": emoteID,
		})
		err := res.Err()
		if err == nil {
			err = res.Decode(emote)
		}
		if err != nil {
			if err != mongo.ErrNoDocuments {
				logrus.WithError(err).Error("mongo")
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
			cur, err := mongo.Collection(mongo.CollectionNameAudit).Find(ctx, bson.M{
				"target.id":   emote.ID,
				"target.type": "emotes",
			})
			if err == nil {
				err = cur.All(ctx, emote.AuditEntries)
			}
			if err != nil {
				logrus.WithError(err).Error("mongo")
				return nil, resolvers.ErrInternalServer
			}
		}
	}

	if emote.Channels == nil {
		if _, ok := fields["channels"]; ok {
			emote.Channels = &[]*datastructure.User{}
		}
	}

	if _, ok := fields["channel_count"]; ok {
		// Get count of notifications
		count, err := cache.GetCollectionSize(ctx, "users", bson.M{
			"emotes": bson.M{
				"$in": []primitive.ObjectID{emote.ID},
			},
		})
		if err != nil {
			return nil, err
		}

		emote.ChannelCount = utils.Int32Pointer(int32(count))
	}

	usr, usrValid := ctx.Value(utils.UserKey).(*datastructure.User)
	if v, ok := fields["reports"]; ok && usrValid && (usr.Rank != datastructure.UserRankAdmin && usr.Rank != datastructure.UserRankModerator) && emote.Reports == nil {
		emote.Reports = &[]*datastructure.Report{}
		cur, err := mongo.Collection(mongo.CollectionNameReports).Find(ctx, bson.M{
			"target.id":   emote.ID,
			"target.type": "emotes",
		})
		if err == nil {
			err = cur.All(ctx, emote.Reports)
		}
		if err != nil {
			logrus.WithError(err).Error("mongo")
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
			cur, err := mongo.Collection(mongo.CollectionNameUsers).Find(ctx, bson.M{
				"_id": bson.M{
					"$in": ids,
				},
			})
			if err == nil {
				err = cur.All(ctx, &reporters)
			}
			if err != nil {
				logrus.WithError(err).Error("mongo")
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

func (r *EmoteResolver) Owner() (*UserResolver, error) {

	if banned, _ := actions.Bans.IsUserBanned(r.v.OwnerID); banned {
		return GenerateUserResolver(r.ctx, datastructure.DeletedUser, nil, r.fields)
	}

	resolver, err := GenerateUserResolver(r.ctx, r.v.Owner, &r.v.OwnerID, r.fields["owner"].Children)
	if err != nil {
		return nil, err
	}
	if resolver == nil {
		return GenerateUserResolver(r.ctx, datastructure.DeletedUser, nil, nil)
	}

	return resolver, nil
}

func (r *EmoteResolver) AuditEntries() (*[]*auditResolver, error) {
	var logs []*datastructure.AuditLog
	if cur, err := mongo.Collection(mongo.CollectionNameAudit).Find(r.ctx, bson.M{
		"target.type": "emotes",
		"target.id":   r.v.ID,
	}, &options.FindOptions{
		Sort: bson.M{
			"_id": -1,
		},
		Limit: utils.Int64Pointer(20),
	}); err != nil {
		logrus.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	} else {
		err := cur.All(r.ctx, &logs)
		if err != nil && err != mongo.ErrNoDocuments {
			logrus.WithError(err).Error("mongo")
			return nil, err
		}
	}

	resolvers := make([]*auditResolver, len(logs))
	for i, l := range logs {
		if l == nil {
			continue
		}

		resolver, err := GenerateAuditResolver(r.ctx, l, r.fields)
		if err != nil {
			logrus.WithError(err).Error("GenerateAuditResolver")
			continue
		}

		resolvers[i] = resolver
	}

	return &resolvers, nil
}

func (r *EmoteResolver) Channels(ctx context.Context, args struct {
	Page  *int32
	Limit *int32
}) (*[]*UserResolver, error) {
	// Get queried page
	page := int32(1)
	if args.Page != nil {
		page = *args.Page
		if page < 1 {
			page = 1
		}
	}

	// Define limit
	limit := int32(20)
	if args.Limit != nil {
		limit = *args.Limit
		if limit < 1 || limit > 250 {
			limit = 250
		}
	}

	// Get the users with this emote
	users := []*datastructure.User{}
	pipeline := mongo.Pipeline{
		{{ // Step 1: Query for users with the emote enabled
			Key: "$match",
			Value: bson.M{
				"emotes": r.v.ID,
			},
		}},
		{{ // Step 2: Add users' role data
			Key: "$lookup",
			Value: bson.M{
				"from":         "roles",
				"localField":   "role",
				"foreignField": "_id",
				"as":           "_role",
			},
		}},
		// Step 3: Perform a sort by role position
		{{Key: "$sort", Value: bson.M{"_role.position": -1}}},

		// Paginate
		{{Key: "$skip", Value: utils.Int64Pointer(int64((page - 1) * limit))}},
		{{Key: "$limit", Value: utils.Int64Pointer(int64(limit))}},
	}

	if cur, err := mongo.Collection(mongo.CollectionNameUsers).Aggregate(ctx, pipeline); err != nil {
		logrus.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	} else {
		if err = cur.All(ctx, &users); err != nil {
			logrus.WithError(err).Error("mongo")
			return nil, err
		}
	}

	resolvers := []*UserResolver{}
	for _, usr := range users {
		resolver, err := GenerateUserResolver(r.ctx, usr, &usr.ID, nil)
		if err != nil {
			return nil, err
		}
		if resolver.ub.IsBanned() {
			continue
		}

		resolvers = append(resolvers, resolver)
	}

	return &resolvers, nil
}

func (r *EmoteResolver) ChannelCount() int32 {
	return *r.v.ChannelCount
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
