package users

import (
	"encoding/json"
	"strings"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/restutil"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetUser(router fiber.Router) {
	router.Get("/:user", func(c *fiber.Ctx) error {
		id, err := primitive.ObjectIDFromHex(c.Params("user"))
		if err != nil {
			id = primitive.NilObjectID
		}

		var user datastructure.User
		res := mongo.Collection(mongo.CollectionNameUsers).FindOne(c.Context(), bson.M{
			"$or": bson.A{
				bson.M{"_id": id},
				bson.M{"login": strings.ToLower(c.Params("user"))},
				bson.M{"id": strings.ToLower(c.Params("user"))},
			},
		})
		err = res.Err()
		if err == nil {
			err = res.Decode(&user)
		}
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return restutil.ErrUnknownUser().Send(c)
			}
		}

		response := restutil.CreateUserResponse(&user)
		b, err := json.Marshal(&response)
		if err != nil {
			return restutil.ErrInternalServer().Send(c, err.Error())
		}

		return c.Send(b)
	})
}
