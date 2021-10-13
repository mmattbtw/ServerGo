package redis

import (
	"context"

	"github.com/sirupsen/logrus"
)

var (
	tokenConsumerLuaScriptSHA1 string
)

func AuthTokenValues(ctx context.Context, token string) (string, error) {
	res, err := Client.EvalSha(
		ctx,
		tokenConsumerLuaScriptSHA1, // scriptSHA1
		[]string{},                 // KEYS
		token,                      // ARGV[1]
	).Result()
	if err != nil {
		return "", err
	}
	resp, ok := res.(string)
	if !ok {
		logrus.WithField("resp", res).Error("invalid redis resp expected string")
		return "", errInvalidResp
	}
	return resp, nil
}
