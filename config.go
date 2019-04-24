package main

import (
	"fmt"
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

type outputCfg struct {
	Name           string
	Type           string
	Algo           string
	Targets        []string
	ConnectTimeout duration `toml:"connect_timeout"`
	Timeout        duration
	BufferSize     int      `toml:"buffer_size"`
	BatchSize      int      `toml:"batch_size"`
	BatchTimeout   duration `toml:"batch_timeout"`
}

type config struct {
	Listen        []string
	Timeout       duration
	StatsInterval duration              `toml:"stats_interval"`
	BufferSize    int                   `toml:"buffer_size"`
	Outputs       map[string]*outputCfg `toml:"output"`
}

func defaultConfig() config {
	return config{
		Listen:        []string{"127.0.0.1:32167"},
		StatsInterval: duration{60 * time.Second},
		Timeout:       duration{30 * time.Second},
		BufferSize:    50000,

		Outputs: map[string]*outputCfg{},
	}
}

var cfg = defaultConfig()

func configLoad(file string) error {
	if _, err := toml.DecodeFile(file, &cfg); err != nil {
		return fmt.Errorf("Unable to load config: %s", err)
	}

	if len(cfg.Outputs) == 0 {
		return fmt.Errorf("No outputs defined")
	}

	for n, o := range cfg.Outputs {
		o.Name = n

		if o.ConnectTimeout.Duration == 0 {
			o.ConnectTimeout.Duration = 5 * time.Second
		}

		if o.Timeout.Duration == 0 {
			o.Timeout.Duration = 5 * time.Second
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
	}

	return nil
}
