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
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

/*
* Query Params:
* user_identifier: "object_id", "twitch_id", "login"
 */
func GetCosmetics(router fiber.Router) {
	Avatar(router)

	mx := &sync.Mutex{}
	cosmeticsHandler := func(c *fiber.Ctx) error {
		mx.Lock()
		defer mx.Unlock()
		ctx := c.Context()
		c.Set("Cache-Control", "max-age=600, s-maxage=600")

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
		// Find entitlements
		cur, err := mongo.Collection(mongo.CollectionNameEntitlements).Aggregate(ctx, mongo.Pipeline{
			{{Key: "$sort", Value: bson.M{"priority": -1}}},
			{{Key: "$match", Value: bson.M{
				"disabled": bson.M{"$not": bson.M{"$eq": true}},
				"kind": bson.M{"$in": []datastructure.EntitlementKind{
					datastructure.EntitlementKindRole,
					datastructure.EntitlementKindBadge,
					datastructure.EntitlementKindPaint,
				}},
			}}},
		})
		if err != nil {
			logrus.WithError(err).Error("mongo, failed to spawn cosmetic entitlements aggregation")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}

		// Decode data
		entitlements := []*datastructure.Entitlement{}
		if err = cur.All(ctx, &entitlements); err != nil {
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
		ents := make(map[datastructure.EntitlementKind][]*datastructure.Entitlement)
		for _, ent := range entitlements {
			a := ents[ent.Kind]
			if a == nil {
				ents[ent.Kind] = []*datastructure.Entitlement{ent}
			} else {
				ents[ent.Kind] = append(a, ent)
			}
		}

		// Map user IDs by roles
		roleMap := make(map[primitive.ObjectID][]primitive.ObjectID)
		for _, ent := range ents[datastructure.EntitlementKindRole] {
			r := ent.GetData().ReadRole()
			if a := roleMap[ent.UserID]; a != nil {
				roleMap[ent.UserID] = append(roleMap[ent.UserID], r.ObjectReference)
			} else {
				roleMap[ent.UserID] = []primitive.ObjectID{r.ObjectReference}
			}
		}

		// Check entitled paints / badges for users we need to fetch
		entitledUserCount := 0
		entitledUserIDs := make([]primitive.ObjectID, len(ents[datastructure.EntitlementKindBadge])+len(ents[datastructure.EntitlementKindPaint]))
		userCosmetics := make(map[primitive.ObjectID][2]primitive.ObjectID) // [0] has badge, [1] has paint

		for _, ent := range ents[datastructure.EntitlementKindBadge] {
			if ok, d := readEntitled(roleMap, ent); ok {
				uc := userCosmetics[ent.UserID]
				cos := cosMap[d.ObjectReference]
				if !uc[0].IsZero() {
					oldCos := cosMap[uc[0]]
					if oldCos == nil || oldCos.Priority >= cos.Priority {
						continue // skip if priority is lower
					}
					// Find index of old
					for i, id := range oldCos.UserIDs {
						if id == ent.UserID {
							oldCos.UserIDs[i] = oldCos.UserIDs[len(oldCos.UserIDs)-1]
							oldCos.UserIDs = oldCos.UserIDs[:len(oldCos.UserIDs)-1]
							break
						}
					}
				}
				uc[0] = cos.ID
				cos.UserIDs = append(cos.UserIDs, ent.UserID)

				userCosmetics[ent.UserID] = uc
				entitledUserIDs[entitledUserCount] = ent.UserID
				entitledUserCount++
			}
		}
		for _, ent := range ents[datastructure.EntitlementKindPaint] {
			if ok, d := readEntitled(roleMap, ent); ok {
				uc := userCosmetics[ent.UserID]
				if uc[1].IsZero() {
					cos := cosMap[d.ObjectReference]
					cos.UserIDs = append(cos.UserIDs, ent.UserID)
					uc[1] = cos.ID
				}

				userCosmetics[ent.UserID] = uc
				entitledUserIDs[entitledUserCount] = ent.UserID
				entitledUserCount++
			}
		}

		// At this point we can fetch our users
		cur, err = mongo.Collection(mongo.CollectionNameUsers).Find(ctx, bson.M{
			"_id": bson.M{"$in": entitledUserIDs[:entitledUserCount]},
		}, options.Find().SetProjection(bson.M{
			"_id":   1,
			"id":    1,
			"login": 1,
		}))
		if err != nil {
			logrus.WithError(err).Error("mongo, failed to spawn cosmetic users cursor")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		// Decode data
		users := []*datastructure.User{}
		if err = cur.All(ctx, &users); err != nil {
			logrus.WithError(err).Error("mongo, failed to decode cosmetic users")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		userMap := make(map[primitive.ObjectID]*datastructure.User, len(users))
		for _, u := range users {
			userMap[u.ID] = u
		}

		// Find directly assigned users
		result := GetCosmeticsResult{
			Badges: []*restutil.BadgeCosmeticResponse{},
			Paints: []*restutil.PaintCosmeticResponse{},
		}
		for _, cos := range cosmetics {
			if len(cos.UserIDs) == 0 {
				continue // skip if cosmetic has no users
			}
			cos.Users = make([]*datastructure.User, len(cos.UserIDs))
			for i, uid := range cos.UserIDs {
				cos.Users[i] = userMap[uid]
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
	}

	router.Get("/cosmetics", func(c *fiber.Ctx) error {
		return cosmeticsHandler(c)
	})

	router.Get("/badges", func(c *fiber.Ctx) error {
		return cosmeticsHandler(c)
	})

	Avatar(router)
}

func readEntitled(rm map[primitive.ObjectID][]primitive.ObjectID, ent *datastructure.Entitlement) (bool, *datastructure.EntitledItem) {
	d := ent.GetData().ReadItem()

	if !d.Selected {
		return false, d
	}
	if d.RoleBinding != nil {
		rb := *d.RoleBinding
		roleList := rm[ent.UserID]
		if !utils.ContainsObjectID(roleList, rb) {
			return false, d // skip if user not
		}
	}
	return true, d
}

type GetCosmeticsResult struct {
	Badges []*restutil.BadgeCosmeticResponse `json:"badges"`
	Paints []*restutil.PaintCosmeticResponse `json:"paints"`
}
