package actions

import (
	"context"
	"time"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// FetchBans gets the current active bans stored in database and store them in memory
func (b *bans) FetchBans(ctx context.Context) error {
	b.Mtx.Lock()
	defer b.Mtx.Unlock()

	banList := []*datastructure.Ban{}
	cur, err := mongo.Collection(mongo.CollectionNameBans).Find(ctx, bson.M{
		"$or": bson.A{
			bson.M{"expire_at": nil},
			bson.M{"expire_at": bson.M{"$gt": time.Now()}},
		},
	})
	if err != nil {
		return err
	}

	if err := cur.All(ctx, &banList); err != nil {
		return err
	}

	for _, ban := range banList {
		userID := ban.UserID

		b.BannedUsers[*userID] = ban
	}

	return nil
}

func (b *bans) IsUserBanned(id primitive.ObjectID) (bool, string) {
	b.Mtx.Lock()
	defer b.Mtx.Unlock()

	ban, ok := b.BannedUsers[id]
	if !ok {
		return false, ""
	}
	if time.Now().After(ban.ExpireAt) {
		delete(b.BannedUsers, id)
		return false, ""
	}

	return true, ban.Reason
}
