package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/bsm/redislock"
	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var json = jsoniter.Config{
	TagKey:                 "bson",
	EscapeHTML:             true,
	SortMapKeys:            true,
	ValidateJsonRawMessage: true,
}.Froze()

func genSha(prefix, collection string, q interface{}, opts interface{}) (string, error) {
	h := sha256.New()
	bytes, err := json.Marshal(q)
	if err != nil {
		return "", err
	}
	h.Write(utils.S2B(prefix))
	h.Write(utils.S2B(collection))
	h.Write(bytes)
	if opts != nil {
		bytes, err = json.Marshal(opts)
		if err != nil {
			return "", err
		}
		h.Write(bytes)
	}
	sha1 := hex.EncodeToString(h.Sum(nil))
	return sha1, nil
}

// Gets the collection size then caches it in redis for some time
func GetCollectionSize(ctx context.Context, collection string, q interface{}, opts ...*options.CountOptions) (int64, error) {
	sha1, err := genSha("collection-size", collection, q, opts)
	if err != nil {
		return 0, err
	}
	key := "cached:collection-size:" + collection + ":" + sha1

	if count, err := redis.Client.Get(ctx, key).Int64(); err == nil { // Try to find the cached value in redis
		return count, nil
	} else { // Otherwise, query mongo
		count, err := mongo.Database.Collection(collection).CountDocuments(ctx, q, opts...)
		if err != nil {
			return 0, err
		}

		redis.Client.Set(ctx, key, count, 1*time.Minute)
		return count, nil
	}

}

// Send a GET request to an endpoint and cache the result
func CacheGetRequest(ctx context.Context, uri string, cacheDuration time.Duration, errorCacheDuration time.Duration, headers ...struct {
	Key   string
	Value string
}) (*cachedGetRequest, error) {
	//
	h := sha256.New()
	h.Write(utils.S2B(url.QueryEscape(uri)))
	checkSum := hex.EncodeToString(h.Sum(nil))
	key := "cached:http-get:" + checkSum

	// Establish distributed lock
	// This prevents the same request from being executed multiple times simultaneously
	lock, err := redis.GetLocker().Obtain(ctx, "lock:http-get:"+checkSum, 10*time.Second, &redislock.Options{
		RetryStrategy: redislock.ExponentialBackoff(4, 750),
	})
	if err != nil {
		logrus.WithError(err).Error("CacheGetRequest")
		return nil, err
	}
	defer func() {
		_ = lock.Release(context.Background())
	}()

	// Try to find the cached result of this request
	cachedBody := redis.Client.Get(ctx, key).Val()
	if cachedBody != "" {
		return &cachedGetRequest{
			Status:     "OK",
			StatusCode: 200,
			Body:       utils.S2B(cachedBody),
			FromCache:  true,
		}, nil
	}

	// If not cached let's make the request
	req, _ := http.NewRequest("GET", uri, nil)
	for _, header := range headers { // Add custom headers
		req.Header.Add(header.Key, header.Value)
	}

	startedAt := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	logrus.WithFields(logrus.Fields{
		"status":         resp.StatusCode,
		"response_in_ms": time.Since(startedAt).Milliseconds(),
		"completed_at":   time.Now(),
	}).Info("CacheGetRequest")

	// Read the body as byte slice
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// Cache the request body
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		redis.Client.Set(ctx, key, body, cacheDuration)
	} else if errorCacheDuration > 0 { // Cache as errored for specified amount of time?
		redis.Client.Set(ctx, key, fmt.Sprintf("err=%v", resp.StatusCode), errorCacheDuration)
	}

	// Return request
	return &cachedGetRequest{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       body,
		FromCache:  false,
	}, nil
}

type cachedGetRequest struct {
	Status     string
	StatusCode int
	Header     map[string][]string
	Body       []byte
	FromCache  bool
}
