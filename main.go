package main

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/mitchellh/panicwrap"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/discord"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/cache"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	_ "github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/server"

	"github.com/SevenTV/ServerGo/src/server/api/actions"
	"github.com/SevenTV/ServerGo/src/server/api/tasks"
)

func init() {
	logrus.Infoln("starting")
}

func main() {
	// Catch panics - send alert to discord channel optionally
	exitStatus, err := panicwrap.BasicWrap(panicHandler)
	if err != nil {
		// PANIC-CEPTION. BRRRRRRRRRRRRRRRRRRRRRRRR
		panic(err)
	}
	if exitStatus >= 0 {
		os.Exit(exitStatus)
	}

	configCode := configure.Config.GetInt("exit_code")
	if configCode > 125 || configCode < 0 {
		logrus.WithField("requested_exit_code", configCode).Warn("invalid exit code specified in config using 0 as new exit code")
		configCode = 0
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	s := server.New()

	go func() {
		sig := <-c
		logrus.WithField("sig", sig).Info("stop issued")

		start := time.Now().UnixNano()

		// Run pre-shutdown cleanup
		Cleanup()

		wg := sync.WaitGroup{}

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Shutdown(); err != nil {
				logrus.WithError(err).Error("failed to shutdown server")
			}
		}()
		wg.Wait()

		logrus.WithField("duration", float64(time.Now().UnixNano()-start)/10e5).Infof("shutdown")
		os.Exit(configCode)
	}()

	go func() {
		for {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			logrus.WithField("alloc", m.Alloc).WithField("total_alloc", m.TotalAlloc).WithField("sys", m.Sys).WithField("numgc", m.NumGC).Debug("stats")
			time.Sleep(5 * time.Second)
		}
	}()

	logrus.Infoln("started")

	// Get and cache roles*
	ctx := context.Background()
	roles, err := GetAllRoles(ctx)
	if err != nil {
		logrus.WithError(err).Error("could not get roles")
	}
	logrus.WithField("count", len(roles)).Infof("retrieved roles")

	// Sync bans
	err = actions.Bans.FetchBans(ctx)
	if err != nil {
		logrus.WithError(err).Error("could not sync bans")
	}
	logrus.WithField("count", len(actions.Bans.BannedUsers)).Info("retrieved bans")

	go tasks.Start()

	select {}
}

func Cleanup() {
	// Cleanup ongoing tasks
	tasks.Cleanup()

	// Logout from discord
	_ = discord.Discord.CloseWithCode(1000)
}

// Get all roles available and cache into the mongo context
func GetAllRoles(ctx context.Context) ([]datastructure.Role, error) {
	roles := []datastructure.Role{}
	cur, err := mongo.Collection(mongo.CollectionNameRoles).Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	roles = append(roles, *datastructure.DefaultRole) // Add default role
	if err := cur.All(ctx, &roles); err != nil {      // Fetch roles
		return nil, err
	}

	// Set "AllRoles" value to mongo context
	cache.CachedRoles = roles
	return roles, nil
}

func panicHandler(output string) {
	logrus.Errorf("PANIC OCCURED: %s", output)
	// Try to send a message to discord
	discord.SendPanic(output)

	os.Exit(1)
}
