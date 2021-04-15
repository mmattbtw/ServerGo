package mongo

import (
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"

	"context"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/redis"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// var json = jsoniter.Config{
// 	TagKey:                 "bson",
// 	EscapeHTML:             true,
// 	SortMapKeys:            true,
// 	ValidateJsonRawMessage: true,
// }.Froze()

var Database *mongo.Database
var Ctx = context.TODO()

var ErrNoDocuments = mongo.ErrNoDocuments

type Pipeline = mongo.Pipeline
type WriteModel = mongo.WriteModel

func NewUpdateOneModel() *mongo.UpdateOneModel {
	return mongo.NewUpdateOneModel()
}

func init() {
	clientOptions := options.Client().ApplyURI(configure.Config.GetString("mongo_uri"))
	if configure.Config.GetBool("mongo_direct") {
		clientOptions.SetDirect(true)
	}
	client, err := mongo.Connect(Ctx, clientOptions)
	if err != nil {
		panic(err)
	}

	err = client.Ping(Ctx, nil)
	if err != nil {
		panic(err)
	}

	Database = client.Database(configure.Config.GetString("mongo_db"))

	_, err = Database.Collection("emotes").Indexes().CreateMany(Ctx, []mongo.IndexModel{
		{Keys: bson.M{"name": 1}},
		{Keys: bson.M{"owner_id": 1}},
		{Keys: bson.M{"tags": 1}},
		{Keys: bson.M{"status": 1}},
		{Keys: bson.M{"last_modified_date": 1}, Options: options.Index().SetExpireAfterSeconds(int32(time.Hour * 24 * 21 / time.Second)).SetPartialFilterExpression(bson.M{
			"status": EmoteStatusDeleted,
		})},
		{Keys: bson.M{"channel_count_checked_at": 1}},
	})
	if err != nil {
		log.Errorf("mongodb, err=%v", err)
		return
	}

	_, err = Database.Collection("users").Indexes().CreateMany(Ctx, []mongo.IndexModel{
		{Keys: bson.M{"id": 1}, Options: options.Index().SetUnique(true)},
		{Keys: bson.M{"login": 1}, Options: options.Index().SetUnique(true)},
		{Keys: bson.M{"rank": 1}},
		{Keys: bson.M{"editors": 1}},
	})
	if err != nil {
		log.Errorf("mongodb, err=%v", err)
		return
	}

	_, err = Database.Collection("bans").Indexes().CreateMany(Ctx, []mongo.IndexModel{
		{Keys: bson.M{"user_id": 1}},
		{Keys: bson.M{"issued_by": 1}},
		{Keys: bson.M{"active": 1}},
	})
	if err != nil {
		log.Errorf("mongodb, err=%v", err)
		return
	}

	_, err = Database.Collection("audit").Indexes().CreateMany(Ctx, []mongo.IndexModel{
		{Keys: bson.M{"type": 1}},
		{Keys: bson.M{"target.type": 1}},
		{Keys: bson.M{"target.id": 1}},
	})
	if err != nil {
		log.Errorf("mongodb, err=%v", err)
		return
	}

	_, err = Database.Collection("reports").Indexes().CreateMany(Ctx, []mongo.IndexModel{
		{Keys: bson.M{"reporter_id": 1}},
		{Keys: bson.M{"target.type": 1}},
		{Keys: bson.M{"target.id": 1}},
	})
	if err != nil {
		log.Errorf("mongodb, err=%v", err)
		return
	}

	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)

	for _, v := range []string{"users", "emotes", "bans", "reports", "audit"} {
		go func(col string) {
			userChangeStream, err := Database.Collection(col).Watch(Ctx, mongo.Pipeline{}, opts)
			if err != nil {
				panic(err)
			}
			go func() {
				for userChangeStream.Next(Ctx) {
					data := bson.M{}
					if err := userChangeStream.Decode(&data); err != nil {
						log.Errorf("mongo change stream, err=%v, col=%s", err, col)
						continue
					}
					changeStream(col, data)
				}
			}()
		}(v)
	}

}

func changeStream(collection string, data bson.M) {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("recovered, err=%v", err)
		}
	}()
	// spew.Dump(data)
	var commonIndex string
	var ojson string
	eventType := data["operationType"].(string)
	eventID := (data["_id"].(bson.M))["_data"].(string)
	oid := ((data["documentKey"].(bson.M))["_id"].(primitive.ObjectID)).Hex()
	if eventType != "delete" {
		document := data["fullDocument"].(bson.M)
		dataString, err := jsoniter.MarshalToString(document)
		if err != nil {
			log.Errorf("json, err=%v", err)
			return
		}
		ojson = dataString
	}
	if eventType == "create" {
		switch collection {
		case "emote":
		}
	}
	_, err := redis.InvalidateCache(fmt.Sprintf("cached:events:%s", eventID), collection, oid, commonIndex, ojson)
	if err != nil {
		log.Errorf("redis, err=%s", err)
	}
}
