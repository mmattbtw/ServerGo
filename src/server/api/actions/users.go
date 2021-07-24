package actions

import (
	"context"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (users) GetByID(ctx context.Context, id primitive.ObjectID) (*datastructure.User, error) {
	res := mongo.Collection(mongo.CollectionNameUsers).FindOne(ctx, bson.M{"_id": id})
	err := res.Err()

	if err != nil {
		return nil, err
	}

	var user datastructure.User
	if err := res.Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (users) Get(ctx context.Context, q bson.M) (*datastructure.User, error) {
	res := mongo.Collection(mongo.CollectionNameUsers).FindOne(ctx, q)
	err := res.Err()

	if err != nil {
		return nil, err
	}

	var user datastructure.User
	if err := res.Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}
