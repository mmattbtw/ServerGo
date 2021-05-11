package redis

import (
	"context"
	"fmt"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/bsm/redislock"
	"github.com/go-redis/redis/v8"
	"github.com/gobuffalo/packr/v2"
)

var Ctx = context.Background()

var (
	errInvalidResp = fmt.Errorf("invalid resp from redis")
)

func init() {
	options, err := redis.ParseURL(configure.Config.GetString("redis_uri"))
	if err != nil {
		panic(err)
	}

	Client = redis.NewClient(options)
	Publisher = redis.NewClient(options)
	Subscriber = redis.NewClient(options)

	box := packr.New("lua", "./lua")

	tokenConsumerLuaScript, err := box.FindString("token-consumer.lua")
	if err != nil {
		panic(err)
	}
	v, err := Client.ScriptLoad(Ctx, tokenConsumerLuaScript).Result()
	if err != nil {
		panic(err)
	}
	tokenConsumerLuaScriptSHA1 = v

	getCacheLuaScript, err := box.FindString("get-cache.lua")
	if err != nil {
		panic(err)
	}
	v, err = Client.ScriptLoad(Ctx, getCacheLuaScript).Result()
	if err != nil {
		panic(err)
	}
	getCacheLuaScriptSHA1 = v

	setCacheLuaScript, err := box.FindString("set-cache.lua")
	if err != nil {
		panic(err)
	}
	v, err = Client.ScriptLoad(Ctx, setCacheLuaScript).Result()
	if err != nil {
		panic(err)
	}
	setCacheLuaScriptSHA1 = v

	invalidateCacheLuaScript, err := box.FindString("invalidate-cache.lua")
	if err != nil {
		panic(err)
	}
	v, err = Client.ScriptLoad(Ctx, invalidateCacheLuaScript).Result()
	if err != nil {
		panic(err)
	}
	invalidateCacheLuaScriptSHA1 = v

	invalidateCommonIndexCacheLuaScript, err := box.FindString("invalidate-common-index-cache.lua")
	if err != nil {
		panic(err)
	}
	v, err = Client.ScriptLoad(Ctx, invalidateCommonIndexCacheLuaScript).Result()
	if err != nil {
		panic(err)
	}
	invalidateCommonIndexCacheLuaScriptSHA1 = v
}

var Client *redis.Client

var lockerClient *redislock.Client

func GetLocker() *redislock.Client {
	if lockerClient == nil {
		lockerClient = redislock.New(Client)
	}

	return lockerClient
}

type Message = redis.Message

type StringCmd = redis.StringCmd

type StringStringMapCmd = redis.StringStringMapCmd

type Z = redis.Z

const ErrNil = redis.Nil
