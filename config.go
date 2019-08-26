package main

import (
	"fmt"
	"net"
	"time"

	"github.com/BurntSushi/toml"
)

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type inputCfg struct {
	Name     string
	Listen   string
	ListenWS string

	TimeoutRead  duration `toml:"timeout_read"`
	TimeoutWrite duration `toml:"timeout_write"`

	Outputs []string
}

type outputCfg struct {
	Name string
	Type string

	Algo         string
	AlgoFailover bool     `toml:"algo_failover"`
	HashFields   []string `toml:"hash_fields"`
	CarbonFields []string `toml:"carbon_fields"`
	CarbonValue  string   `toml:"carbon_value"`

	Targets           []string
	ReconnectInterval duration `toml:"reconnect_interval"`
	TimeoutConnect    duration `toml:"timeout_connect"`
	TimeoutRead       duration `toml:"timeout_read"`
	TimeoutWrite      duration `toml:"timeout_write"`
	BufferSize        int      `toml:"buffer_size"`
	BatchSize         int      `toml:"batch_size"`
	BatchTimeout      duration `toml:"batch_timeout"`
}

type config struct {
	ListenHTTP string

	StatsInterval duration `toml:"stats_interval"`
	BufferSize    int      `toml:"buffer_size"`
	LogLevel      string   `toml:"log_level"`

	Inputs  map[string]*inputCfg  `toml:"input"`
	Outputs map[string]*outputCfg `toml:"output"`
}

func defaultConfig() config {
	return config{
		ListenHTTP:    "127.0.0.1:12347",
		StatsInterval: duration{60 * time.Second},
		BufferSize:    50000,

		Outputs: map[string]*outputCfg{},
	}
}

var cfg = defaultConfig()

func configLoad(file string) error {
	if _, err := toml.DecodeFile(file, &cfg); err != nil {
		return fmt.Errorf("Unable to load config: %s", err)
	}

	if len(cfg.Inputs) == 0 {
		return fmt.Errorf("No inputs defined")
	}

	if len(cfg.Outputs) == 0 {
		return fmt.Errorf("No outputs defined")
	}

	outputs := map[string]bool{}
	for n, o := range cfg.Outputs {
		outputs[n] = true

		o.Name = n

		if o.TimeoutConnect.Duration == 0 {
			o.TimeoutConnect.Duration = 5 * time.Second
		}

		if o.TimeoutRead.Duration == 0 {
			o.TimeoutRead.Duration = 5 * time.Second
		}

		if o.TimeoutWrite.Duration == 0 {
			o.TimeoutWrite.Duration = 5 * time.Second
		}

		if o.ReconnectInterval.Duration == 0 {
			o.ReconnectInterval.Duration = 1 * time.Second
		}

		if o.BufferSize == 0 {
			o.BufferSize = 50000
		}

		if o.BatchSize == 0 {
			o.BatchSize = 50
		}

		if o.BatchTimeout.Duration == 0 {
			o.BatchTimeout.Duration = 1 * time.Second
		}

		tgtMap := map[string]bool{}
		for _, t := range o.Targets {
			if tgtMap[t] {
				return fmt.Errorf("Output '%s': Target '%s' is specified more than once", n, t)
			}

			tgtMap[t] = true

			if _, err := net.ResolveTCPAddr("tcp", t); err != nil {
				return fmt.Errorf("Output %s: %s: Bad TCP address specified: %s", n, t, err)
			}
		}
	}

	for n, i := range cfg.Inputs {
		i.Name = n

		if i.TimeoutRead.Duration == 0 {
			i.TimeoutRead.Duration = 5 * time.Second
		}

		if i.TimeoutWrite.Duration == 0 {
			i.TimeoutWrite.Duration = 5 * time.Second
		}

		if len(i.Outputs) == 0 {
			return fmt.Errorf("Input %s: No outputs defined", n)
		}
	}

	return nil
}
