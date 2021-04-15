package redis

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	getCacheLuaScriptSHA1 string
)

func GetCache(collection, sha1 string) ([]interface{}, error) {
	res, err := Client.EvalSha(
		Ctx,
		getCacheLuaScriptSHA1, // scriptSHA1
		[]string{
			fmt.Sprintf("cached:queries:%s", collection),
			fmt.Sprintf("cached:objects:%s", collection),
		}, // KEYS
		sha1, // ARGV[1]
	).Result()
	if err != nil {
		return nil, err
	}
	resp, ok := res.([]interface{})
	if !ok {
		log.Errorf("redis resp, resp=%v", res)
		return nil, errInvalidResp
	}
	return resp, nil
}

var (
	setCacheLuaScriptSHA1 string
)

func SetCache(collection, sha1, commonIndex string, args ...string) (int64, error) {
	if len(args)%2 != 0 {
		return 0, fmt.Errorf("invalid args, must be even")
	}

	newArgs := make([]interface{}, len(args)+3)
	newArgs[0] = time.Now().Unix()
	newArgs[1] = sha1
	newArgs[2] = len(args)
	for i, v := range args {
		newArgs[i+3] = v
	}

	keys := []string{
		fmt.Sprintf("cached:queries:%s", collection),
		fmt.Sprintf("cached:objects:%s", collection),
	}
	if commonIndex != "" {
		keys = append(keys, fmt.Sprintf("cached:common-index:%s:%s", collection, commonIndex))
	}

	s, err := Client.EvalSha(
		Ctx,
		setCacheLuaScriptSHA1, // scriptSHA1
		keys,                  // KEYS
		newArgs...,            // ARGV
	).Result()
	if err != nil {
		return 0, err
	}
	resp, ok := s.(int64)
	if !ok {
		log.Errorf("redis resp, resp=%v", s)
		return 0, errInvalidResp
	}
	return resp, nil
}

var (
	invalidateCacheLuaScriptSHA1 string
)

func InvalidateCache(invalidateKey, collection, objectID, commonIndex string, objectJSON string) (int64, error) {
	keys := []string{
		invalidateKey,
		fmt.Sprintf("cached:queries:%s", collection),
		fmt.Sprintf("cached:objects:%s", collection),
	}
	if commonIndex != "" {
		keys = append(keys, fmt.Sprintf("cached:common-index:%s:%s", collection, commonIndex))
	}
	s, err := Client.EvalSha(
		Ctx,
		invalidateCacheLuaScriptSHA1, // scriptSHA1
		keys,                         // KEYS
		time.Now().Unix(),            // ARGV[1]
		objectID,                     // ARGV[2]
		objectJSON,                   // ARGV[3]
	).Result()
	if err != nil {
		return 0, err
	}
	resp, ok := s.(int64)
	if !ok {
		log.Errorf("redis resp, resp=%v", s)
		return 0, errInvalidResp
	}
	return resp, nil
}

var (
	invalidateCommonIndexCacheLuaScriptSHA1 string
)

func InvalidateCommonIndexCache(collection, commonIndex string) (int64, error) {
	keys := []string{
		fmt.Sprintf("cached:queries:%s", collection),
		fmt.Sprintf("cached:common-index:%s:%s", collection, commonIndex),
	}
	s, err := Client.EvalSha(
		Ctx,
		invalidateCommonIndexCacheLuaScriptSHA1, // scriptSHA1
		keys,                                    // KEYS
		time.Now().Unix(),                       // ARGV[1]
	).Result()
	if err != nil {
		return 0, err
	}
	resp, ok := s.(int64)
	if !ok {
		log.Errorf("redis resp, resp=%v", s)
		return 0, errInvalidResp
	}
	return resp, nil
}
