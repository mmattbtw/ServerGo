package query_resolvers

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	api_proxy "github.com/SevenTV/ServerGo/src/server/api/v2/proxy"
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

type QueryResolver struct{}

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var searchRegex = regexp.MustCompile(`[.*+?^${}()|[\\]\\\\]`)

type SelectedField struct {
	Name     string
	Children map[string]*SelectedField
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
				Name:     f.Name,
				Children: children,
			}
			if d > maxD {
				maxD = d
			}
		}
		return m, maxD
	}
	children, depth := loop(graphql.SelectedFieldsFromContext(ctx), 0)
	return &SelectedField{
		Name:     "query",
		Children: children,
	}, depth > max
}

func (*QueryResolver) User(ctx context.Context, args struct{ ID string }) (*UserResolver, error) {
	isMe := args.ID == "@me" // Handle @me (current authenticated user)
	user := &datastructure.User{}
	var id *primitive.ObjectID
	if isMe {
		// Get current user from context if @me
		if u, ok := ctx.Value(utils.UserKey).(*datastructure.User); isMe && u != nil && ok {
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
			return nil, resolvers.ErrInternalServer
		}
	} else {
		if hexId, err := primitive.ObjectIDFromHex(args.ID); err == nil {
			id = &hexId
			user = nil
		} else {
			return nil, nil
		}
	}

	field, failed := GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	return GenerateUserResolver(ctx, user, id, field.Children)
}

func (*QueryResolver) Role(ctx context.Context, args struct{ ID string }) (*RoleResolver, error) {
	id, err := primitive.ObjectIDFromHex(args.ID)
	if err != nil {
		return nil, nil
	}

	field, failed := GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	return GenerateRoleResolver(ctx, nil, &id, field.Children)
}

func (*QueryResolver) Emote(ctx context.Context, args struct{ ID string }) (*EmoteResolver, error) {
	id, err := primitive.ObjectIDFromHex(args.ID)
	if err != nil {
		return nil, nil
	}

	field, failed := GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	resolver, err := GenerateEmoteResolver(ctx, nil, &id, field.Children)
	if err != nil {
		return nil, err
	}
	// Get actor user
	usr, _ := ctx.Value(utils.UserKey).(*datastructure.User)
	// Verify actor permissions
	if utils.HasBits(int64(resolver.v.Visibility), int64(datastructure.EmoteVisibilityPrivate)) && usr.ID != resolver.v.OwnerID {
		if usr == nil || !datastructure.UserHasPermission(usr, datastructure.RolePermissionEmoteEditAll) {
			return nil, resolvers.ErrUnknownEmote
		}
	}

	return resolver, nil
}

func (*QueryResolver) Emotes(ctx context.Context, args struct{ List []string }) (*[]*EmoteResolver, error) {
	field, failed := GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	ids := mongo.HexIDSliceToObjectID(args.List)
	emotes := []*datastructure.Emote{}
	if len(args.List) > 0 {
		if err := cache.Find("emotes", "", bson.M{
			"_id": bson.M{
				"$in": ids,
			},
		}, &emotes); err != nil {
			log.Errorf("mongo, err=%v", err)
			return nil, resolvers.ErrInternalServer
		}
	}

	resolvers := make([]*EmoteResolver, len(emotes))
	if len(emotes) > 0 {
		for i, e := range emotes {
			resolvers[i], _ = GenerateEmoteResolver(ctx, e, nil, field.Children)
		}
	}

	return &resolvers, nil
}

func (*QueryResolver) SearchEmotes(ctx context.Context, args struct {
	Query       string
	Page        *int32
	PageSize    *int32
	Limit       *int32
	GlobalState *string
	SortBy      *string
	SortOrder   *int32
	Channel     *string
	SubmittedBy *string
}) ([]*EmoteResolver, error) {
	field, failed := GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	// Define limit
	// This is how many emotes can be searched in one request at most
	limit := int64(20)
	if args.Limit != nil {
		limit = int64(*args.Limit)
	}
	if limit > resolvers.QueryLimit {
		return nil, resolvers.ErrQueryLimit
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

	// Get actor user
	usr, _ := ctx.Value(utils.UserKey).(*datastructure.User)

	// Create aggregation
	opts := options.Aggregate()
	emotes := []*datastructure.Emote{}
	match := bson.M{
		"status": datastructure.EmoteStatusLive,
	}
	if args.Channel != nil {
		var targetChannel *datastructure.User
		// Find user and get their emotes
		if err := cache.FindOne("users", "", bson.M{"login": args.Channel}, &targetChannel); err == nil {
			match["_id"] = bson.M{"$in": targetChannel.EmoteIDs}
		}
	}

	if usr == nil || !datastructure.UserHasPermission(usr, datastructure.RolePermissionEmoteEditAll) {
		var usrID primitive.ObjectID
		if usr != nil {
			usrID = usr.ID
		}

		match["$and"] = bson.A{
			bson.M{"$or": bson.A{
				bson.M{"visibility": bson.M{"$bitsAllClear": int32(datastructure.EmoteVisibilityPrivate)}},
				bson.M{"owner": usrID},
			}},
		}
	}

	// Define aggregation pipeline
	pipeline := mongo.Pipeline{}

	// Get sorting direction
	var order int32 = 1
	if args.SortOrder != nil {
		order = *args.SortOrder
	}
	if order > 1 {
		return nil, resolvers.ErrInvalidSortOrder
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

			countedEmotes := []*datastructure.Emote{}
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
			match["visibility"] = bson.M{"$bitsAllSet": int32(datastructure.EmoteVisibilityGlobal)}
		case "hide": // Hide: omit global emotes from query
			match["visibility"] = bson.M{"$bitsAllClear": int32(datastructure.EmoteVisibilityGlobal)}
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
		return nil, resolvers.ErrInternalServer
	}

	// Resolve emotes
	resolvers := make([]*EmoteResolver, len(emotes))
	for i, e := range emotes {
		resolvers[i], err = GenerateEmoteResolver(ctx, e, nil, field.Children)
		if err != nil {
			return nil, err
		}
	}
	return resolvers, nil
}

func (*QueryResolver) ThirdPartyEmotes(ctx context.Context, args struct {
	Providers []string
	Channel   string
	Global    *bool
}) (*[]*EmoteResolver, error) {
	field, failed := GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	// Query foreign APIs for requested third party emotes
	var emotes []*datastructure.Emote
	var globalEmotes []*datastructure.Emote
	for _, p := range args.Providers {
		switch p {
		case "BTTV": // Handle BTTV Provider
			if args.Channel != "" {
				if bttv, err := api_proxy.GetChannelEmotesBTTV(args.Channel); err == nil { // Find channel bttv emotes
					emotes = append(emotes, bttv...)
				}
			}
			if args.Global != nil && *args.Global {
				if bttvG, err := api_proxy.GetGlobalEmotesBTTV(); err == nil { // Find global bttv emotes
					globalEmotes = append(globalEmotes, bttvG...)
				}
			}
		case "FFZ": // Handle FFZ Provider
			if args.Channel != "" {
				if ffz, err := api_proxy.GetChannelEmotesFFZ(args.Channel); err == nil { // Find channel FFZ emotes
					emotes = append(emotes, ffz...)
				}
			}
			if args.Global != nil && *args.Global {
				if ffzG, err := api_proxy.GetGlobalEmotesFFZ(); err == nil { // Find global ffz emotes
					globalEmotes = append(globalEmotes, ffzG...)
				}
			}
		}
	}
	for _, e := range globalEmotes {
		e.Visibility = datastructure.EmoteVisibilityGlobal
	}
	emotes = append(emotes, globalEmotes...)

	// Create emote resolvers to return
	result := make([]*EmoteResolver, len(emotes))
	for i, emote := range emotes {
		resolver, _ := GenerateEmoteResolver(ctx, emote, nil, field.Children)
		if resolver == nil {
			continue
		}

		result[i] = resolver
	}

	return &result, nil
}

func (*QueryResolver) SearchUsers(ctx context.Context, args struct {
	Query string
	Page  *int32
	Limit *int32
}) ([]*UserResolver, error) {
	field, failed := GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	limit := int64(20)
	if args.Limit != nil {
		limit = int64(*args.Limit)
	}
	if limit > resolvers.QueryLimit {
		return nil, resolvers.ErrQueryLimit
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

	users := []*datastructure.User{}

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
		return nil, resolvers.ErrInternalServer
	}

	resolvers := make([]*UserResolver, len(users))
	for i, e := range users {
		resolvers[i], err = GenerateUserResolver(ctx, e, nil, field.Children)
		if err != nil {
			return nil, err
		}
	}
	return resolvers, nil
}