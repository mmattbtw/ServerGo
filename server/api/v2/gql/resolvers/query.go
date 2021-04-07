package resolvers

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/SevenTV/ServerGo/cache"
	"github.com/SevenTV/ServerGo/mongo"
	"github.com/SevenTV/ServerGo/utils"
	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/selection"
	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const (
	maxDepth   = 3
	queryLimit = 150
)

var searchRegex = regexp.MustCompile(`[.*+?^${}()|[\\]\\\\]`)

var (
	errInternalServer = fmt.Errorf("an internal server error occured")
	errDepth          = fmt.Errorf("exceeded max depth of %v", maxDepth)
	errQueryLimit     = fmt.Errorf("exeeded max query limit of %v", queryLimit)
)

type SelectedField struct {
	name     string
	children map[string]*SelectedField
}

func GenerateSelectedFieldMap(ctx context.Context, max int) (*SelectedField, bool) {
	var loop func(fields []*selection.SelectedField, localDepth int) (map[string]*SelectedField, int)
	loop = func(fields []*selection.SelectedField, localDepth int) (map[string]*SelectedField, int) {
		if len(fields) == 0 {
			return nil, localDepth
		}
		m := map[string]*SelectedField{}
		localDepth++
		maxD := localDepth
		for _, f := range fields {
			if maxD > max {
				return nil, maxD
			}
			children, d := loop(f.SelectedFields, localDepth)
			m[f.Name] = &SelectedField{
				name:     f.Name,
				children: children,
			}
			if d > maxD {
				maxD = d
			}
		}
		return m, maxD
	}
	children, depth := loop(graphql.SelectedFieldsFromContext(ctx), 0)
	return &SelectedField{
		name:     "query",
		children: children,
	}, depth > max
}

func (*RootResolver) User(ctx context.Context, args struct{ ID string }) (*userResolver, error) {
	isMe := args.ID == "@me" // Handle @me (current authenticated user)
	var currentUser *mongo.User

	id, err := primitive.ObjectIDFromHex(args.ID)
	if err != nil && !isMe {
		return nil, nil
	}

	// Get current user from context if @me
	if u, ok := ctx.Value(utils.UserKey).(*mongo.User); isMe && u != nil && ok {
		currentUser = u
	}
	if isMe && currentUser == nil { // Handle error: current user requested but request was unauthenticated
		return nil, nil
	}

	field, failed := GenerateSelectedFieldMap(ctx, maxDepth)
	if failed {
		return nil, errDepth
	}

	return GenerateUserResolver(ctx, currentUser, &id, field.children)
}

func (*RootResolver) Role(ctx context.Context, args struct{ ID string }) (*roleResolver, error) {
	id, err := primitive.ObjectIDFromHex(args.ID)
	if err != nil {
		return nil, nil
	}

	field, failed := GenerateSelectedFieldMap(ctx, maxDepth)
	if failed {
		return nil, errDepth
	}

	return GenerateRoleResolver(ctx, nil, &id, field.children)
}

func (*RootResolver) Emote(ctx context.Context, args struct{ ID string }) (*emoteResolver, error) {
	id, err := primitive.ObjectIDFromHex(args.ID)
	if err != nil {
		return nil, nil
	}

	field, failed := GenerateSelectedFieldMap(ctx, maxDepth)
	if failed {
		return nil, errDepth
	}

	return GenerateEmoteResolver(ctx, nil, &id, field.children)
}

func (*RootResolver) Emotes(ctx context.Context, args struct{ UserID string }) (*[]*emoteResolver, error) {
	objID, err := primitive.ObjectIDFromHex(args.UserID)
	if err != nil {
		return nil, nil
	}

	field, failed := GenerateSelectedFieldMap(ctx, maxDepth)
	if failed {
		return nil, errDepth
	}

	user := &mongo.User{}

	if err = cache.FindOne("users", "", bson.M{
		"_id": objID,
	}, user); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	emotes := []*mongo.Emote{}

	if len(user.EmoteIDs) > 0 {
		if err = cache.Find("emotes", "", bson.M{
			"_id": bson.M{
				"$in": user.EmoteIDs,
			},
		}, &emotes); err != nil {
			log.Errorf("mongo, err=%v", err)
			return nil, errInternalServer
		}
	}

	resolvers := make([]*emoteResolver, len(emotes))

	if len(emotes) > 0 {
		for i, e := range emotes {
			e.Owner = user
			resolvers[i], _ = GenerateEmoteResolver(ctx, e, nil, field.children)
		}
	}

	return &resolvers, nil
}

func (*RootResolver) TwitchUser(ctx context.Context, args struct{ ChannelName string }) (*userResolver, error) {
	field, failed := GenerateSelectedFieldMap(ctx, maxDepth)
	if failed {
		return nil, errDepth
	}

	user := &mongo.User{}

	if err := cache.FindOne("users", "", bson.M{
		"login": strings.ToLower(args.ChannelName),
	}, user); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		log.Errorf("mongo, err=%v", err)
		return nil, errDepth
	}

	return GenerateUserResolver(ctx, user, nil, field.children)
}

func (*RootResolver) SearchEmotes(ctx context.Context, args struct {
	Query    string
	Page     *int32
	PageSize *int32
	Limit    *int32
}) ([]*emoteResolver, error) {
	field, failed := GenerateSelectedFieldMap(ctx, maxDepth)
	if failed {
		return nil, errDepth
	}

	limit := int64(20)
	if args.Limit != nil {
		limit = int64(*args.Limit)
	}
	if limit > queryLimit {
		return nil, errQueryLimit
	}

	query := strings.Trim(args.Query, " ")
	hasQuery := len(query) > 0
	lQuery := fmt.Sprintf("(?i)%s", strings.ToLower(searchRegex.ReplaceAllString(query, "\\\\$0")))

	// Pagination
	page := int64(*args.Page)
	if page < 1 {
		page = int64(1)
	}
	pageSize := int64(*args.PageSize)
	log.Info("Pagination", page, pageSize)

	opts := options.Aggregate()

	emotes := []*mongo.Emote{}
	match := bson.M{
		"status": mongo.EmoteStatusLive,
		"$or": bson.A{
			bson.M{
				"name": bson.M{
					"$regex": lQuery,
				},
			},
			bson.M{
				"tags": bson.M{
					"$regex": lQuery,
				},
			},
		},
	}

	// Create mongo pipeline
	pipeline := mongo.Pipeline{
		bson.D{primitive.E{Key: "$match", Value: match}},                // Match query
		bson.D{primitive.E{Key: "$skip", Value: (page - 1) * pageSize}}, // Paginate
		bson.D{primitive.E{Key: "$limit", Value: pageSize}},             // Set limit
	}
	if hasQuery { // If a query is specified, add sorting
		pipeline = append(pipeline, bson.D{primitive.E{Key: "$sort", Value: bson.D{
			{Key: "name", Value: 1},
			{Key: "tags", Value: 1},
		}}})
	}
	cur, err := mongo.Database.Collection("emotes").Aggregate(mongo.Ctx, pipeline, opts)
	if err == nil {
		err = cur.All(mongo.Ctx, &emotes)
	}
	if err != nil {
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	resolvers := make([]*emoteResolver, len(emotes))
	for i, e := range emotes {
		resolvers[i], err = GenerateEmoteResolver(ctx, e, nil, field.children)
		if err != nil {
			return nil, err
		}
	}
	return resolvers, nil
}

func (*RootResolver) SearchUsers(ctx context.Context, args struct {
	Query string
	Limit *int32
}) ([]*userResolver, error) {
	field, failed := GenerateSelectedFieldMap(ctx, maxDepth)
	if failed {
		return nil, errDepth
	}

	limit := int64(20)
	if args.Limit != nil {
		limit = int64(*args.Limit)
	}
	if limit > queryLimit {
		return nil, errQueryLimit
	}

	query := strings.Trim(args.Query, " ")
	lQuery := fmt.Sprintf("(?i)%s", strings.ToLower(searchRegex.ReplaceAllString(query, "\\\\$0")))

	opts := options.Find().SetSort(bson.M{
		"login": 1,
	}).SetLimit(limit)

	users := []*mongo.User{}

	cur, err := mongo.Database.Collection("users").Find(mongo.Ctx, bson.M{
		"login": bson.M{
			"$regex": lQuery,
		},
	}, opts)
	if err == nil {
		err = cur.All(mongo.Ctx, &users)
	}
	if err != nil {
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	resolvers := make([]*userResolver, len(users))
	for i, e := range users {
		resolvers[i], err = GenerateUserResolver(ctx, e, nil, field.children)
		if err != nil {
			return nil, err
		}
	}
	return resolvers, nil
}
