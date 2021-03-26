package main

import (
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/SevenTV/ServerGo/configure"
	_ "github.com/SevenTV/ServerGo/redis"
	"github.com/SevenTV/ServerGo/server"
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

	select {}
}
