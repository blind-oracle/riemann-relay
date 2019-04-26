//go:generate protoc --go_out=. riemann.proto
package main

import (
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"
)

var (
	lis     *listener
	outputs []*output
)

func main() {
	var (
		wg  sync.WaitGroup
		err error
	)

	l := &logger{"Main"}
	chanClose, chanDrain := make(chan struct{}), make(chan struct{})

	configFile := flag.String("config", "/etc/riemann-relay.toml", "Path to a config file")
	debug := flag.Bool("debug", false, "Enable debug logging (use with care - a LOT of output)")
	flag.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
		l.Infof("Debugging enabled")
	}

	if err = configLoad(*configFile); err != nil {
		l.Fatalf("Unable to load config file: %s", err)
	}
	l.Infof("Configuration loaded")

	// Buffer for event batches coming from listener
	chanInput := make(chan []*Event, cfg.BufferSize)

	// Fire up listener
	if lis, err = newListener(cfg.Listen, cfg.Timeout.Duration, chanInput); err != nil {
		l.Fatalf("Unable to set up listener: %s", err)
	}

	// Fire up outputs
	for _, ocfg := range cfg.Outputs {
		o, err := newOutput(ocfg)
		if err != nil {
			l.Fatalf("Unable to init output '%s': %s", ocfg.Name, err)
		}

		outputs = append(outputs, o)
	}

	pushBatch := func(batch []*Event) {
		for _, o := range outputs {
			o.chanIn <- batch
		}
	}

	wg.Add(1)
	// Fire up event dispatcher
	go func() {
		defer wg.Done()
		var (
			batch []*Event
			ok    bool
		)

		for {
			select {
			case <-chanDrain:
				l.Infof("Draining input buffer...")

				c := 0
				for {
					batch, ok = <-chanInput
					if !ok {
						l.Infof("Input buffer drained (%d batches)", c)
						return
					}

					pushBatch(batch)
					c++
				}

			case batch, ok = <-chanInput:
				if !ok {
					continue
				}

				pushBatch(batch)
			}
		}
	}()

	l.Infof("HTTP listening to %s", cfg.ListenHTTP)
	go initHTTP()

	// Set up signal handling
	go func() {
		sigchannel := make(chan os.Signal, 1)
		signal.Notify(sigchannel, syscall.SIGTERM, os.Interrupt)

		for sig := range sigchannel {
			switch sig {
			case os.Interrupt, syscall.SIGTERM:
				l.Warnf("Got SIGTERM/Ctrl+C, shutting down")
				close(chanClose)
				return
			}
		}
	}()

	// Wait for shutdown
	<-chanClose

	// Close listener first and wait for all events to drain to outputs
	lis.Close()
	close(chanInput)
	close(chanDrain)
	wg.Wait()

	// Drain & close outputs
	for _, o := range outputs {
		o.Close()
	}

	l.Infof("Shutdown complete")
}
