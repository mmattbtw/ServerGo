package redis

import (
	log "github.com/sirupsen/logrus"
)

var (
	tokenConsumerLuaScriptSHA1 string
)

func AuthTokenValues(token string) (string, error) {
	res, err := Client.EvalSha(
		Ctx,
		tokenConsumerLuaScriptSHA1, // scriptSHA1
		[]string{},                 // KEYS
		token,                      // ARGV[1]
	).Result()
	if err != nil {
		return "", err
	}
	resp, ok := res.(string)
	if !ok {
		log.Errorf("redis resp, resp=%v", res)
		return "", errInvalidResp
	}
	return resp, nil
}
