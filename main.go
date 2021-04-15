package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/mongo"
	_ "github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/server"
	"github.com/SevenTV/ServerGo/src/utils"
)

func init() {
	log.Infoln("Application Starting...")
}

func main() {
	configCode := configure.Config.GetInt("exit_code")
	if configCode > 125 || configCode < 0 {
		log.Warnf("Invalid exit code specified in config (%v), using 0 as new exit code.", configCode)
		configCode = 0
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	s := server.New()

	go func() {
		sig := <-c
		log.Infof("sig=%v, gracefully shutting down...", sig)
		start := time.Now().UnixNano()

		wg := sync.WaitGroup{}

		wg.Wait()

		wg.Add(1)

		go func() {
			defer wg.Done()
			if err := s.Shutdown(); err != nil {
				log.Errorf("failed to shutdown server, err=%v", err)
			}
		}()

		wg.Wait()

		log.Infof("Shutdown took, %.2fms", float64(time.Now().UnixNano()-start)/10e5)
		os.Exit(configCode)
	}()

	go func() {
		for {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			log.Debugf("Alloc = %vM\tTotalAlloc = %vM\tSys = %vM\tNumGC = %v", m.Alloc/1024/1024, m.TotalAlloc/1024/1024, m.Sys/1024/1024, m.NumGC)
			time.Sleep(5 * time.Second)
		}
	}()

	log.Infoln("Application Started.")

	// Get and cache roles
	roles, err := GetAllRoles()
	if err != nil {
		log.Errorf("could not get roles, %s", err)
	}
	log.Infof("Retrieved %s roles", fmt.Sprint(len(roles)))

	select {}
}

// Get all roles available and cache into the mongo context
func GetAllRoles() ([]mongo.Role, error) {
	roles := []mongo.Role{}
	cur, err := mongo.Database.Collection("roles").Find(mongo.Ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	roles = append(roles, *mongo.DefaultRole)          // Add default role
	if err := cur.All(mongo.Ctx, &roles); err != nil { // Fetch roles
		if err == mongo.ErrNoDocuments {
			return roles, nil
		}

		return nil, err
	}

	// Set "AllRoles" value to mongo context
	mongo.Ctx = context.WithValue(mongo.Ctx, utils.AllRolesKey, roles)
	return roles, nil
}
