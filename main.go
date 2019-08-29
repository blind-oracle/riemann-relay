//go:generate protoc --go_out=. riemann.proto
package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	version   string
	startTime = time.Now()

	hashKey = []byte{
		0xf7, 0x74, 0x6b, 0xd7, 0xc2, 0x19, 0xe4, 0xa8,
		0xc4, 0x8d, 0xc3, 0xd5, 0x0f, 0x7b, 0x1f, 0x54,
		0x46, 0xa5, 0xdf, 0x7c, 0x64, 0x55, 0x1c, 0x8d,
		0x77, 0x94, 0xbb, 0x5d, 0x9f, 0x63, 0x54, 0x63,
	}

	inputs      = map[string]*input{}
	inputNames  []string
	outputs     = map[string]*output{}
	outputNames []string
)

func main() {
	var err error

	l := &logger{"Main"}
	l.Warnf("riemann-relay v%s starting", version)

	chanClose := make(chan struct{})

	configFile := flag.String("config", "/etc/riemann-relay/riemann-relay.conf", "Path to a config file")
	flag.Parse()

	if err = configLoad(*configFile); err != nil {
		l.Fatalf("Unable to load config file: %s", err)
	}
	l.Infof("Configuration loaded")

	if cfg.LogLevel != "" {
		lvl, err := log.ParseLevel(cfg.LogLevel)
		if err != nil {
			log.Fatalf("Unable to parse '%s' as log level: %s", cfg.LogLevel, err)
		}

		log.SetLevel(lvl)
	} else {
		log.SetLevel(log.WarnLevel)
	}

	// Fire up outputs
	for _, c := range cfg.Outputs {
		o, err := newOutput(c)
		if err != nil {
			l.Fatalf("Unable to init output '%s': %s", c.Name, err)
		}

		outputs[c.Name] = o
		outputNames = append(outputNames, c.Name)
	}
	l.Warnf("Outputs started: %d", len(outputs))

	// Fire up inputs
	unusedOutputs := map[string]bool{}
	for on := range outputs {
		unusedOutputs[on] = true
	}

	for _, c := range cfg.Inputs {
		i, err := newInput(c)
		if err != nil {
			l.Fatalf("Unable to init input '%s': %s", c.Name, err)
		}

		for _, on := range c.Outputs {
			if o, ok := outputs[on]; !ok {
				l.Fatalf("Input %s: output '%s' not defined", c.Name, on)
			} else {
				i.addHandler(on, o.pushBatch)
				delete(unusedOutputs, on)
			}
		}

		inputs[c.Name] = i
		inputNames = append(inputNames, c.Name)
	}
	l.Warnf("Inputs started: %d", len(inputs))

	if len(unusedOutputs) > 0 {
		l.Fatalf("Unused outputs in a config file: %+v", unusedOutputs)
	}

	l.Warnf("HTTP listening to %s", cfg.ListenHTTP)
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

	// Close inputs
	for _, i := range inputs {
		i.close()
	}

	// Drain & close outputs
	for _, o := range outputs {
		o.close()
	}

	l.Warnf("Shutdown complete")
}
