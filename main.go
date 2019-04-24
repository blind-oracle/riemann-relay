//go:generate protoc --go_out=. riemann.proto
package main

import (
	"flag"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	stats struct {
		processed uint64
	}
)

func main() {
	var (
		outputs []*output
		wg      sync.WaitGroup
	)

	chanClose, chanDrain := make(chan struct{}), make(chan struct{})

	var err error
	configFile := flag.String("config", "/etc/riemann-relay.toml", "Path to a config file")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	if err = configLoad(*configFile); err != nil {
		log.Fatalf("Unable to load config file: %s", err)
	}

	chanInput := make(chan []*Event, cfg.BufferSize)
	lis, err := newListener(cfg.Listen, cfg.Timeout.Duration, chanInput)
	if err != nil {
		log.Fatalf("Unable to set up listener: %s", err)
	}

	for _, ocfg := range cfg.Outputs {
		o, err := newOutput(ocfg)
		if err != nil {
			log.Fatalf("Unable to init output '%s': %s", ocfg.Name, err)
		}

		outputs = append(outputs, o)
	}

	go func() {
		sigchannel := make(chan os.Signal, 1)
		signal.Notify(sigchannel, syscall.SIGTERM, os.Interrupt)

		for sig := range sigchannel {
			switch sig {
			case os.Interrupt, syscall.SIGTERM:
				log.Warnf("Got SIGTERM/Ctrl+C, shutting down")
				close(chanClose)
				return
			}
		}
	}()

	wg.Add(1)
	if cfg.StatsInterval.Duration > 0 {
		go func() {
			defer wg.Done()
			t := time.NewTicker(cfg.StatsInterval.Duration)
			defer t.Stop()

			for {
				select {
				case <-chanClose:
					return

				case <-t.C:
					log.Infof("Events processed: %d", atomic.LoadUint64(&stats.processed))
				}
			}
		}()
	}

	pushToOutputs := func(batch []*Event) {
		for _, o := range outputs {
			o.chanIn <- batch
		}

		atomic.AddUint64(&stats.processed, uint64(len(batch)))
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		var (
			batch []*Event
			ok    bool
		)

		for {
			select {
			case <-chanDrain:
				log.Info("Draining input buffer...")

				c := 0
				for {
					batch, ok = <-chanInput
					if !ok {
						log.Infof("Input buffer drained (%d batches)", c)
						return
					}

					pushToOutputs(batch)
					c++
				}

			case batch, ok = <-chanInput:
				if !ok {
					continue
				}

				pushToOutputs(batch)
			}
		}
	}()

	<-chanClose
	lis.Close()
	close(chanInput)
	close(chanDrain)
	wg.Wait()

	for _, o := range outputs {
		o.Close()
	}

}
