package session

import (
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/configure"
	"github.com/SevenTV/ServerGo/redis"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/google/uuid"
)

type Storage struct{}

// Get value by key
func (s *Storage) Get(key string) ([]byte, error) {
	if len(key) <= 0 {
		return nil, nil
	}
	val, err := redis.Client.Get(redis.Ctx, key).Bytes()
	if err == redis.ErrNil {
		return nil, nil
	}
	return val, err
}

// Set key with value
func (s *Storage) Set(key string, val []byte, exp time.Duration) error {
	// Ain't Nobody Got Time For That
	if len(key) <= 0 || len(val) <= 0 {
		return nil
	}
	return redis.Client.Set(redis.Ctx, key, val, exp).Err()
}

// Delete key by key
func (s *Storage) Delete(key string) error {
	// Ain't Nobody Got Time For That
	if len(key) <= 0 {
		return nil
	}
	return redis.Client.Del(redis.Ctx, key).Err()
}

// Reset all keys
func (s *Storage) Reset() error {
	// wtf
	return nil
}

// Close the database
func (s *Storage) Close() error {
	return nil
}

var Store = session.New(session.Config{
	Storage:        &Storage{},
	CookieDomain:   configure.Config.GetString("cookie_domain"),
	CookieHTTPOnly: true,
	CookieName:     "SESSION",
	CookieSameSite: "LAX",
	Expiration:     time.Hour * 24 * 14,
	CookieSecure:   configure.Config.GetBool("cookie_secure"),
	KeyGenerator: func() string {
		u, _ := uuid.NewRandom()
		return fmt.Sprintf("sessions:%s", u.String())
	},
})

func init() {
	Store.RegisterType("")
}
