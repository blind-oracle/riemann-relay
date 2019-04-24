package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/segmentio/fasthash/fnv1a"
	log "github.com/sirupsen/logrus"
)

type outputAlgo int
type outputType int
type writeBatchFunc func([]*Event) error

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
)

type output struct {
	name string

	typ  outputType
	algo outputAlgo

	tgts    []*target
	tgtCnt  int
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
}

func newOutput(cfg *outputCfg) (*output, error) {
	typ, ok := outputTypeMap[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("Unknown output type '%s'", cfg.Type)
	}

	algo, ok := outputAlgoMap[cfg.Algo]
	if !ok {
		return nil, fmt.Errorf("Unknown algorithm '%s'", cfg.Algo)
	}

	o := &output{
		name:   cfg.Name,
		typ:    typ,
		algo:   algo,
		tgtCnt: len(cfg.Targets),
		chanIn: make(chan []*Event),
	}

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

	for _, h := range cfg.Targets {
		t := &target{
			host:        h,
			typ:         typ,
			connTimeout: cfg.ConnectTimeout.Duration,
			timeout:     cfg.Timeout.Duration,
			o:           o,

			chanClose:    make(chan struct{}),
			chanDispatch: make(chan struct{}),

			chanIn: make(chan *Event, cfg.BufferSize),
		}

		t.ctx, t.ctxCancel = context.WithCancel(context.Background())

		switch typ {
		case outputTypeCarbon:
			t.writeBatch = t.writeBatchCarbon
		case outputTypeRiemann:
			t.writeBatch = t.writeBatchRiemann
		}

		t.batch.buf = make([]*Event, cfg.BatchSize)
		t.batch.size = cfg.BatchSize
		t.batch.timeout = cfg.BatchTimeout.Duration

		o.tgts = append(o.tgts, t)
		t.wg.Add(1)
		go t.run()
	}

	o.wg.Add(1)
	go o.dispatch()

	o.Infof("Output started")
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
		if key, err = o.getKey(e); err != nil {
			o.Warnf("Unable to get key, skipping event: %s", err)
			continue
		}

		if tgts = o.getTargets(key); len(tgts) == 0 {
			o.Debugf("No targets, skipping event")
			atomic.AddUint64(&o.stats.droppedNoTargets, 1)
			continue
		}

		for _, t := range tgts {
			select {
			case t.chanIn <- e:
				atomic.AddUint64(&o.stats.processed, 1)
			default:
				atomic.AddUint64(&t.stats.dropped, 1)
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
	id := fnv1a.HashString64(key) % uint64(len(o.tgts))
	tgts = append(tgts, o.tgts[id])
	return
}

func (o *output) getTargetsBroadcast(key string) (tgts []*target) {
	return o.tgts
}

func (o *output) logHdr(msg string) string {
	return o.name + ": " + msg
}

func (o *output) Debugf(msg string, args ...interface{}) {
	log.Debugf(o.logHdr(msg), args...)
}

func (o *output) Infof(msg string, args ...interface{}) {
	log.Infof(o.logHdr(msg), args...)
}

func (o *output) Warnf(msg string, args ...interface{}) {
	log.Warnf(o.logHdr(msg), args...)
}

func (o *output) Errorf(msg string, args ...interface{}) {
	log.Errorf(o.logHdr(msg), args...)
}
