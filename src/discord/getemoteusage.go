package discord

// func getEmotesUsage(ctx context.Context) ([]*datastructure.Emote, error) {
// 	cur, err := mongo.Database.Collection("emotes").Find(ctx, bson.M{
// 		"channel_count": bson.M{
// 			"$gte": 1,
// 		},
// 	}, &options.FindOptions{
// 		Limit: utils.Int64Pointer(50),
// 		Sort: bson.D{
// 			{Key: "channel_count", Value: -1},
// 		},
// 	})

// 	if err != nil {
// 		return nil, err
// 	}

// 	var emotes []*datastructure.Emote
// 	_ = cur.All(ctx, &emotes)

// 	return emotes, nil
// }
