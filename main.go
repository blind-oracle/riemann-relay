//go:generate protoc --go_out=. riemann.proto
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	processed uint64
)

func main() {
	//log.SetLevel(log.DebugLevel)
	ctxShutdown, ctxCancel := context.WithCancel(context.Background())

	var err error
	configFile := flag.String("config", "/etc/riemann-relay.toml", "Path to a config file")
	flag.Parse()

	if err = configLoad(*configFile); err != nil {
		log.Fatalf("Unable to load config file: %s", err)
	}

	chanInput := make(chan *Event, cfg.BufferSize)
	lis, err := newListener(cfg.Listen, cfg.Timeout.Duration, chanInput)
	if err != nil {
		log.Fatalf("Unable to set up listener: %s", err)
	}

	go func() {
		sigchannel := make(chan os.Signal, 1)
		signal.Notify(sigchannel, syscall.SIGTERM, os.Interrupt)

		for sig := range sigchannel {
			switch sig {
			case os.Interrupt, syscall.SIGTERM:
				log.Warnf("Got SIGTERM/Ctrl+C, shutting down")
				ctxCancel()
			}
		}
	}()

	if cfg.StatsInterval.Duration > 0 {
		go func() {
			t := time.NewTicker(cfg.StatsInterval.Duration)
			defer t.Stop()

			for {
				select {
				case <-ctxShutdown.Done():
					return
				case <-t.C:
					log.Infof("Events processed: %d", atomic.LoadUint64(&processed))
				}
			}
		}()
	}

	go func() {
		var e *Event
		for {
			select {
			case <-ctxShutdown.Done():
				return
			case e = <-chanInput:
				log.Infof("%+v", e)
				log.Info(eventToCarbon(e))
				atomic.AddUint64(&processed, 1)
			}
		}
	}()

	<-ctxShutdown.Done()
	lis.Close()
}
