package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash"
)

type outputAlgo int
type outputType int
type writeBatchFunc func([]*Event) error

func (a outputAlgo) String() string {
	return outputAlgoMapRev[a]
}

func (t outputType) String() string {
	return outputTypeMapRev[t]
}

const (
	outputTypeCarbon outputType = iota
	outputTypeRiemann
)

const (
	outputAlgoHash outputAlgo = iota
	outputAlgoFailover
	outputAlgoRoundRobin
	outputAlgoBroadcast
)

var (
	outputTypeMap = map[string]outputType{
		"carbon":  outputTypeCarbon,
		"riemann": outputTypeRiemann,
	}

	outputAlgoMap = map[string]outputAlgo{
		"hash":       outputAlgoHash,
		"failover":   outputAlgoFailover,
		"roundrobin": outputAlgoRoundRobin,
		"broadcast":  outputAlgoBroadcast,
	}

	outputTypeMapRev = map[outputType]string{}
	outputAlgoMapRev = map[outputAlgo]string{}
)

func init() {
	for k, v := range outputTypeMap {
		outputTypeMapRev[v] = k
	}

	for k, v := range outputAlgoMap {
		outputAlgoMapRev[v] = k
	}
}

type output struct {
	name string

	typ  outputType
	algo outputAlgo

	tgts    []*target
	tgtCnt  uint64
	tgtNext int

	wg sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc

	getTargets func(string) []*target
	getKey     func(*Event) (string, error)

	stats struct {
		processed        uint64
		droppedNoTargets uint64
	}

	chanIn chan []*Event
	*logger
}

func newOutput(c *outputCfg) (*output, error) {
	typ, ok := outputTypeMap[c.Type]
	if !ok {
		return nil, fmt.Errorf("Unknown output type '%s'", c.Type)
	}

	algo, ok := outputAlgoMap[c.Algo]
	if !ok {
		return nil, fmt.Errorf("Unknown algorithm '%s'", c.Algo)
	}

	o := &output{
		name:   c.Name,
		typ:    typ,
		algo:   algo,
		tgtCnt: uint64(len(c.Targets)),
		chanIn: make(chan []*Event),
		logger: &logger{c.Name},
	}

	o.Infof("Starting output (type '%s', algo '%s')", typ, algo)

	switch algo {
	case outputAlgoFailover:
		o.getTargets = o.getTargetsFailover
	case outputAlgoRoundRobin:
		o.getTargets = o.getTargetsRoundRobin
	case outputAlgoHash:
		o.getTargets = o.getTargetsHash
	case outputAlgoBroadcast:
		o.getTargets = o.getTargetsBroadcast
	}

	switch typ {
	case outputTypeCarbon:
		o.getKey = o.getKeyCarbon
	case outputTypeRiemann:
		o.getKey = o.getKeyRiemann
	}

	o.ctx, o.ctxCancel = context.WithCancel(context.Background())

	for _, h := range c.Targets {
		t := &target{
			host:              h,
			typ:               typ,
			reconnectInterval: c.ReconnectInterval.Duration,
			connTimeout:       c.ConnectTimeout.Duration,
			timeout:           c.Timeout.Duration,
			o:                 o,

			chanDispatch: make(chan struct{}),
			chanIn:       make(chan *Event, cfg.BufferSize),
			logger:       &logger{fmt.Sprintf("%s: %s", c.Name, h)},
		}

		t.ctx, t.ctxCancel = context.WithCancel(context.Background())

		switch typ {
		case outputTypeCarbon:
			t.writeBatch = t.writeBatchCarbon
		case outputTypeRiemann:
			t.writeBatch = t.writeBatchRiemann
		}

		t.batch.buf = make([]*Event, c.BatchSize)
		t.batch.size = c.BatchSize
		t.batch.timeout = c.BatchTimeout.Duration

		o.tgts = append(o.tgts, t)
		t.wg.Add(1)
		go t.run()
	}

	o.wg.Add(1)
	go o.dispatch()

	if cfg.StatsInterval.Duration > 0 {
		o.wg.Add(1)
		go o.statsTicker()
	}

	return o, nil
}

func (o *output) dispatch() {
	defer o.wg.Done()

	var batch []*Event
	for {
		select {
		case <-o.ctx.Done():
			return

		case batch = <-o.chanIn:
			o.pushBatch(batch)
		}
	}
}

func (o *output) pushBatch(batch []*Event) {
	var (
		key  string
		err  error
		tgts []*target
	)

	for _, e := range batch {
		if o.algo == outputAlgoHash {
			if key, err = o.getKey(e); err != nil {
				o.Warnf("Unable to get key, dropping event: %s", err)
				continue
			}
		}

		if tgts = o.getTargets(key); len(tgts) == 0 {
			o.Debugf("No targets, dropping event")
			atomic.AddUint64(&o.stats.droppedNoTargets, 1)
			promOutNoTarget.WithLabelValues(o.name).Add(1)
			continue
		}

		for _, t := range tgts {
			select {
			case t.chanIn <- e:
				atomic.AddUint64(&o.stats.processed, 1)
				promOutProcessed.WithLabelValues(o.name).Add(1)
			default:
				atomic.AddUint64(&t.stats.dropped, 1)
				promTgtDropped.WithLabelValues(o.name, t.host).Add(1)
			}
		}
	}
}

func (o *output) Close() {
	o.ctxCancel()
	o.wg.Wait()

	o.Infof("Closing targets...")
	for _, t := range o.tgts {
		t.Close()
	}

	o.Infof("Closed")
}

func (o *output) getKeyCarbon(e *Event) (string, error) {
	pfx, err := getPrefix(e)
	if err != nil {
		return "", err
	}

	return pfx + e.Host + e.Service, nil
}

func (o *output) getKeyRiemann(e *Event) (string, error) {
	pfx, err := getPrefix(e)
	if err != nil {
		return "", err
	}

	return pfx + e.Host, nil
}

func (o *output) getTargetsFailover(key string) (tgts []*target) {
	for _, t := range o.tgts {
		if t.isAlive() {
			tgts = append(tgts, t)
			return
		}
	}

	return
}

func (o *output) getTargetsRoundRobin(key string) (tgts []*target) {
	if o.tgtNext >= len(o.tgts) {
		o.tgtNext = 0
	}

	tgts = append(tgts, o.tgts[o.tgtNext])
	o.tgtNext++
	return
}

func (o *output) getTargetsHash(key string) (tgts []*target) {
	id := xxhash.Sum64String(key) % o.tgtCnt
	tgts = append(tgts, o.tgts[id])
	return
}

func (o *output) getTargetsBroadcast(key string) (tgts []*target) {
	return o.tgts
}

func (o *output) statsTicker() {
	defer o.wg.Done()
	tick := time.NewTicker(cfg.StatsInterval.Duration)
	defer tick.Stop()

	for {
		select {
		case <-o.ctx.Done():
			return
		case <-tick.C:
			for _, r := range strings.Split(strings.TrimSpace(o.getStats()), "\n") {
				o.Infof(r)
			}
		}
	}
}

func (o *output) getStats() (s string) {
	s = fmt.Sprintf("processed %d droppedNoTargets %d\n",
		atomic.LoadUint64(&o.stats.processed),
		atomic.LoadUint64(&o.stats.droppedNoTargets),
	)

	for _, t := range o.tgts {
		s += fmt.Sprintf("%s: %s\n", t.host, t.getStats())
	}

	return
}
