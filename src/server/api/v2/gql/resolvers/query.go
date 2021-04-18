package resolvers

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
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
	maxDepth   = 4
	queryLimit = 150
)

var searchRegex = regexp.MustCompile(`[.*+?^${}()|[\\]\\\\]`)

var (
	errInternalServer   = fmt.Errorf("an internal server error occured")
	errDepth            = fmt.Errorf("exceeded max depth of %v", maxDepth)
	errQueryLimit       = fmt.Errorf("exeeded max query limit of %v", queryLimit)
	errInvalidSortOrder = fmt.Errorf("SortOrder is either 0 (descending) or 1 (ascending)")
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
	user := &mongo.User{}
	var id *primitive.ObjectID
	if isMe {
		// Get current user from context if @me
		if u, ok := ctx.Value(utils.UserKey).(*mongo.User); isMe && u != nil && ok {
			user = u
			id = &u.ID
		}
		if isMe && user.ID.IsZero() { // Handle error: current user requested but request was unauthenticated
			return nil, fmt.Errorf("Cannot request @me while unauthenticated")
		}
	} else if !primitive.IsValidObjectID(args.ID) {
		if err := cache.FindOne("users", "", bson.M{
			"login": strings.ToLower(args.ID),
		}, user); err != nil {
			if err == mongo.ErrNoDocuments {
				return nil, nil
			}
			log.Errorf("mongo, err=%v", err)
			return nil, errInternalServer
		}
	} else {
		if hexId, err := primitive.ObjectIDFromHex(args.ID); err == nil {
			id = &hexId
			user = nil
		} else {
			return nil, nil
		}
	}

	field, failed := GenerateSelectedFieldMap(ctx, maxDepth)
	if failed {
		return nil, errDepth
	}

	return GenerateUserResolver(ctx, user, id, field.children)
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
	Query       string
	Page        *int32
	PageSize    *int32
	Limit       *int32
	GlobalState *string
	SortBy      *string
	SortOrder   *int32
}) ([]*emoteResolver, error) {
	field, failed := GenerateSelectedFieldMap(ctx, maxDepth)
	if failed {
		return nil, errDepth
	}

	// Define limit
	// This is how many emotes can be searched in one request at most
	limit := int64(20)
	if args.Limit != nil {
		limit = int64(*args.Limit)
	}
	if limit > queryLimit {
		return nil, errQueryLimit
	}

	// Get the query parameter, used to search for specific emote names or tags
	query := strings.Trim(args.Query, " ")
	hasQuery := len(query) > 0
	lQuery := fmt.Sprintf("(?i)%s", strings.ToLower(searchRegex.ReplaceAllString(query, "\\\\$0")))

	// Pagination
	page := int64(1)
	if args.Page != nil && *args.Page > 1 {
		page = int64(*args.Page)
	}
	pageSize := limit
	if args.PageSize != nil {
		pageSize = int64(*args.PageSize)
	}

	// Create aggregation
	opts := options.Aggregate()
	emotes := []*mongo.Emote{}
	match := bson.M{
		"status": mongo.EmoteStatusLive,
	}

	// Define aggregation pipeline
	pipeline := mongo.Pipeline{}

	// Get sorting direction
	var order int32 = 1
	if args.SortOrder != nil {
		order = *args.SortOrder
	}
	if order > 1 {
		return nil, errInvalidSortOrder
	}
	if order == 1 {
		order = -1
	} else if order == 0 {
		order = 1
	}

	// Handle sorting
	if args.SortBy != nil {
		sortBy := *args.SortBy
		switch sortBy {
		// Popularity Sort - Channels Added
		case "popularity":
			// Create a pipeline for ranking emotes by channel count
			popCheck := mongo.Pipeline{
				{primitive.E{Key: "$match", Value: bson.M{
					"$or": bson.A{
						bson.M{ // Match emotes where kast check was over 6 hours ago
							"channel_count_checked_at": bson.M{
								"$lt": time.Now().Add(-6 * time.Hour),
							},
						},
						bson.M{"channel_count_checked_at": bson.M{"$exists": false}},
						bson.M{"channel_count_checked_at": nil},
					},
				}}},
				{primitive.E{Key: "$lookup", Value: bson.M{
					"from":         "users",
					"localField":   "_id",
					"foreignField": "emotes",
					"as":           "channels",
				}}},
				{primitive.E{Key: "$addFields", Value: bson.M{
					"channel_count": bson.M{"$size": "$channels"},
				}}},
				{primitive.E{Key: "$unset", Value: "channels"}},
			}
			cur, err := mongo.Database.Collection("emotes").Aggregate(mongo.Ctx, popCheck)
			if err != nil {
				fmt.Println(err)
			}

			countedEmotes := []*mongo.Emote{}
			if err := cur.All(mongo.Ctx, &countedEmotes); err == nil && len(countedEmotes) > 0 {
				if err == nil { // Get the unchecked emotes, add them to a slice
					ops := make([]mongo.WriteModel, len(countedEmotes))
					for i, e := range countedEmotes {
						now := time.Now()
						update := mongo.NewUpdateOneModel(). // Append update ops for bulk write
											SetFilter(bson.M{"_id": e.ID}).
											SetUpdate(bson.M{
								"$set": bson.M{
									"channel_count":            e.ChannelCount,
									"channel_count_checked_at": &now,
								},
							})

						ops[i] = update
					}

					// Update unchecked with channel count data
					_, err := mongo.Database.Collection("emotes").BulkWrite(mongo.Ctx, ops)
					if err != nil {
						log.Errorf("mongo, was unable to update channel count of "+fmt.Sprint(len(countedEmotes))+" emotes, err=%v", err)
					}
				}
			} else if err != nil {
				log.Errorf("mongo, was unable to aggregate channel count of emotes, err=%v", err)
			}

			// Sort by channel count
			pipeline = append(pipeline, []bson.D{
				{primitive.E{Key: "$sort", Value: bson.D{
					{Key: "channel_count", Value: -order},
				}}},
			}...)

		// Creation Date Sort
		case "age":
			pipeline = append(pipeline, bson.D{primitive.E{Key: "$sort", Value: bson.D{
				{Key: "_id", Value: order},
			}}})
		}
	}

	// Match & Limit
	pipeline = append(pipeline, []bson.D{
		{primitive.E{Key: "$match", Value: match}},                                                    // Match query
		{primitive.E{Key: "$skip", Value: (page - 1) * pageSize}},                                     // Paginate
		{primitive.E{Key: "$limit", Value: math.Max(0, math.Min(float64(pageSize), float64(limit)))}}, // Set limit
	}...)

	// If a query is specified, add sorting
	if hasQuery {
		match["$or"] = bson.A{
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
		}
	}

	// If global state is specified, filter global emotes
	if args.GlobalState != nil {
		globalState := *args.GlobalState
		switch globalState {
		case "only": // Only: query only global emotes
			match["visibility"] = bson.M{"$bitsAllSet": int32(mongo.EmoteVisibilityGlobal)}
		case "hide": // Hide: omit global emotes from query
			match["visibility"] = bson.M{"$bitsAllClear": int32(mongo.EmoteVisibilityGlobal)}
		}
	}

	// Determine the full collection size
	{
		f := ctx.Value(utils.RequestCtxKey).(*fiber.Ctx) // Fiber context

		// Count documents in the collection
		count, err := cache.GetCollectionSize("emotes", match)
		if err != nil {
			return nil, err
		}

		f.Response().Header.Add("X-Collection-Size", fmt.Sprint(count))
	}

	// Query the DB
	cur, err := mongo.Database.Collection("emotes").Aggregate(mongo.Ctx, pipeline, opts)

	if err == nil {
		err = cur.All(mongo.Ctx, &emotes)
	}
	if err != nil {
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	// Resolve emotes
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
	Page  *int32
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

	// Pagination
	page := int64(1)
	if args.Page != nil && *args.Page > 1 {
		page = int64(*args.Page)
	}

	query := strings.Trim(args.Query, " ")
	lQuery := fmt.Sprintf("(?i)%s", strings.ToLower(searchRegex.ReplaceAllString(query, "\\\\$0")))

	opts := options.Find().SetSort(bson.M{
		"login": 1,
	}).SetLimit(limit).SetSkip((page - 1) * limit)

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
