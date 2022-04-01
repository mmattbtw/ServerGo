package cosmetics

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/restutil"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

/*
* Query Params:
* user_identifier: "object_id", "twitch_id", "login"
 */
func GetBadges(router fiber.Router) {
	Avatar(router)

	mx := sync.Mutex{}
	router.Get("/", func(c *fiber.Ctx) error {
		mx.Lock()
		defer mx.Unlock()
		ctx := c.Context()
		c.Set("Cache-Control", "max-age=150 s-maxage=300")

		idType := c.Query("user_identifier")

		if !utils.Contains([]string{"object_id", "twitch_id", "login"}, idType) {
			return restutil.ErrMissingQueryParams().Send(c, `user_identifier: must be 'object_id', 'twitch_id' or 'login'`)
		}

		// Compose Redis Key
		cacheKey := fmt.Sprintf("cache:cosmetics:%s", idType)

		// Return existing cache?
		d, err := redis.Client.Get(ctx, cacheKey).Result()
		if err == nil && d != "" {
			return c.SendString(d)
		}

		// Retrieve all users of badges
		pipeline := mongo.Pipeline{
			{{Key: "$sort", Value: bson.M{"priority": -1}}},
			{{Key: "$match", Value: bson.M{
				"disabled": bson.M{"$not": bson.M{"$eq": true}},
				"kind": bson.M{"$in": []datastructure.EntitlementKind{
					datastructure.EntitlementKindRole,
					datastructure.EntitlementKindBadge,
					datastructure.EntitlementKindPaint,
				}},
			}}},
			// Lookup cosmetics
			{{
				Key: "$group",
				Value: bson.M{
					"_id": nil,
					"entitlements": bson.M{
						"$push": "$$ROOT",
					},
				},
			}},
			// Lookup: Users
			{{
				Key: "$lookup",
				Value: bson.M{
					"from":         mongo.CollectionNameUsers,
					"localField":   "entitlements.user_id",
					"foreignField": "_id",
					"as":           "users",
				},
			}},
			{{Key: "$project", Value: bson.M{
				"cosmetics":                  1,
				"entitlements._id":           1,
				"entitlements.kind":          1,
				"entitlements.data":          1,
				"entitlements.user_id":       1,
				"users.connections.id":       1,
				"users.connections.platform": 1,
				"users.username":             1,
				"users._id":                  1,
				"users.id":                   1,
				"users.login":                1,
			}}},
		}

		// Run the aggregation
		cur, err := mongo.Collection(mongo.CollectionNameEntitlements).Aggregate(ctx, pipeline)
		if err != nil {
			logrus.WithError(err).Error("mongo, failed to spawn cosmetic entitlements aggregation")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}

		// Decode data
		data := &aggregatedCosmeticsResult{}
		cur.Next(ctx)
		if err = multierror.Append(cur.Decode(data), cur.Close(ctx)).ErrorOrNil(); err != nil {
			logrus.WithError(err).Error("mongo, failed to decode aggregated cosmetic entitlements")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}

		// Map cosmetics
		cosmetics := []*datastructure.Cosmetic{}
		cur, err = mongo.Collection(mongo.CollectionNameCosmetics).Find(
			ctx,
			bson.M{},
			options.Find().SetSort(bson.M{"priority": -1}),
		)
		if err != nil {
			logrus.WithError(err).Error("mongo, failed to fetch cosmetics data")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		if err = cur.All(ctx, &cosmetics); err != nil {
			logrus.WithError(err).Error("mongo, failed to decode cosmetics data")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		cosMap := make(map[primitive.ObjectID]*datastructure.Cosmetic)
		for _, cos := range cosmetics {
			cosMap[cos.ID] = cos
		}

		// Structure entitlements by kind
		// kind:ent_id:[]ent
		ents := make(map[datastructure.EntitlementKind]map[primitive.ObjectID]*datastructure.Entitlement)
		for _, ent := range data.Entitlements {
			m := ents[ent.Kind]
			if m == nil {
				ents[ent.Kind] = map[primitive.ObjectID]*datastructure.Entitlement{}
				m = ents[ent.Kind]
			}
			m[ent.ID] = ent
		}

		// Map users with their roles
		userMap := make(map[primitive.ObjectID]*datastructure.User)
		userCosmetics := make(map[primitive.ObjectID][2]bool) // [0]: badge, [1] paint
		for _, u := range data.Users {
			userMap[u.ID] = u
			userCosmetics[u.ID] = [2]bool{false, false}
		}
		for _, ent := range ents[datastructure.EntitlementKindRole] {
			u := userMap[ent.UserID]
			rol := ent.GetData().ReadRole()
			u.RoleIDs = append(u.RoleIDs, rol.ObjectReference)
		}

		// Find directly assigned users
		result := GetCosmeticsResult{
			Badges: []*restutil.BadgeCosmeticResponse{},
			Paints: []*restutil.PaintCosmeticResponse{},
		}

		for _, ent := range ents[datastructure.EntitlementKindBadge] {
			entd := ent.GetData().ReadItem()
			cos := cosMap[entd.ObjectReference]
			u := userMap[ent.UserID]
			uc := userCosmetics[u.ID]
			if uc[0] || !entd.Selected {
				continue // user already has a badge
			}

			if entd.RoleBinding == nil || utils.ContainsObjectID(u.RoleIDs, *entd.RoleBinding) {
				cos.Users = append(cos.Users, u)
				uc[0] = true
				userCosmetics[u.ID] = uc
			}
		}
		for _, ent := range ents[datastructure.EntitlementKindPaint] {
			entd := ent.GetData().ReadItem()
			cos := cosMap[entd.ObjectReference]
			u := userMap[ent.UserID]
			uc := userCosmetics[u.ID]
			if uc[1] || !entd.Selected {
				continue // user already has a paint
			}

			if entd.RoleBinding == nil || utils.ContainsObjectID(u.RoleIDs, *entd.RoleBinding) {
				cos.Users = append(cos.Users, u)
				uc[1] = true
				userCosmetics[u.ID] = uc
			}
		}

		for _, cos := range cosmetics {
			if len(cos.Users) == 0 {
				continue // skip if cosmetic has no users
			}
			switch cos.Kind {
			case datastructure.CosmeticKindBadge:
				badge := cos.ReadBadge()
				urls := make([][2]string, 3)
				for i := 1; i <= 3; i++ {
					a := [2]string{}
					a[0] = strconv.Itoa(i)
					a[1] = fmt.Sprintf("https://%s/badge/%s/%dx", configure.Config.GetString("cdn_urk"), badge.ID.Hex(), i)
					urls[i-1] = a
				}
				b := restutil.CreateBadgeResponse(cos, cos.Users, idType)
				result.Badges = append(result.Badges, b)
			case datastructure.CosmeticKindNametagPaint:
				paint := cos.ReadPaint()
				stops := make([]datastructure.CosmeticPaintGradientStop, len(paint.Stops))
				dropShadows := make([]datastructure.CosmeticPaintDropShadow, len(paint.DropShadows))
				for i, stop := range paint.Stops {
					stops[i] = datastructure.CosmeticPaintGradientStop{
						At:    stop.At,
						Color: stop.Color,
					}
				}
				for i, shadow := range paint.DropShadows {
					dropShadows[i] = datastructure.CosmeticPaintDropShadow{
						OffsetX: shadow.OffsetX,
						OffsetY: shadow.OffsetY,
						Radius:  shadow.Radius,
						Color:   shadow.Color,
					}
				}
				b := restutil.CreatePaintResponse(cos, cos.Users, idType)
				result.Paints = append(result.Paints, b)
			}
		}

		b, err := json.Marshal(&result)
		if err != nil {
			return restutil.ErrInternalServer().Send(c, err.Error())
		}

		if redis.Client.Set(ctx, cacheKey, utils.B2S(b), 10*time.Minute).Err() != nil {
			logrus.WithField("id_type", idType).WithError(err).Error("couldn't save cosmetics response to redis cache")
		}
		return c.Status(200).Send(b)
	})
}

type aggregatedCosmeticsResult struct {
	Entitlements []*datastructure.Entitlement `bson:"entitlements"`
	Users        []*datastructure.User        `bson:"users"`
}

type GetCosmeticsResult struct {
	Badges []*restutil.BadgeCosmeticResponse `json:"badges"`
	Paints []*restutil.PaintCosmeticResponse `json:"paints"`
}
