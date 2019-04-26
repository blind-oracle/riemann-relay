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

type output struct {
	name string

	typ  outputType
	algo outputAlgo

	hashFields   []riemannFieldName
	carbonFields []riemannFieldName
	carbonValue  riemannValue

	tgts    []*target
	tgtCnt  uint64
	tgtNext int

	wg sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc

	getTargets func([]byte) []*target

	stats struct {
		processed        uint64
		droppedNoTargets uint64
	}

	chanIn chan []*Event
	*logger
}

func newOutput(c *outputCfg) (o *output, err error) {
	typ, ok := outputTypeMap[c.Type]
	if !ok {
		return nil, fmt.Errorf("Unknown output type '%s'", c.Type)
	}

	algo, ok := outputAlgoMap[c.Algo]
	if !ok {
		return nil, fmt.Errorf("Unknown algorithm '%s'", c.Algo)
	}

	cbv, ok := riemannValueMap[c.CarbonValue]
	if !ok {
		return nil, fmt.Errorf("Unknown Carbon value '%s'", c.CarbonValue)
	}

	o = &output{
		name:        c.Name,
		typ:         typ,
		algo:        algo,
		carbonValue: cbv,
		tgtCnt:      uint64(len(c.Targets)),
		chanIn:      make(chan []*Event),
		logger:      &logger{c.Name},
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

	if algo == outputAlgoHash {
		if o.hashFields, err = parseRiemannFields(c.HashFields); err != nil {
			return
		}

		if len(o.hashFields) == 0 {
			return nil, fmt.Errorf("You need to specify hash_fields")
		}
	}

	if typ == outputTypeCarbon {
		if o.carbonFields, err = parseRiemannFields(c.CarbonFields); err != nil {
			return
		}

		if len(o.carbonFields) == 0 {
			return nil, fmt.Errorf("You need to specify carbon_fields for output type 'carbon'")
		}
	}

	o.Infof("Starting output (type '%s', algo '%s', hash fields: %v, carbon fields: %v)", typ, algo, o.hashFields, o.carbonFields)
	o.ctx, o.ctxCancel = context.WithCancel(context.Background())

	for _, h := range c.Targets {
		o.tgts = append(o.tgts, newOutputTgt(h, c, o))
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
		key  []byte
		tgts []*target
	)

	for _, e := range batch {
		if o.algo == outputAlgoHash {
			key = o.getKey(e)
			o.Debugf("Hash key: %s", key)
		}

		// Compute a list of targets to send
		if tgts = o.getTargets(key); len(tgts) == 0 {
			o.Debugf("No targets, dropping event")
			atomic.AddUint64(&o.stats.droppedNoTargets, 1)
			promOutNoTarget.WithLabelValues(o.name).Add(1)
			continue
		}

		// Push the batch to all targets
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

func (o *output) getKey(e *Event) []byte {
	return eventCompileFields(e, o.hashFields, ".")
}

func (o *output) getTargetsFailover(key []byte) (tgts []*target) {
	for _, t := range o.tgts {
		if t.isAlive() {
			tgts = append(tgts, t)
			return
		}
	}

	return
}

func (o *output) getTargetsRoundRobin(key []byte) (tgts []*target) {
	if o.tgtNext >= len(o.tgts) {
		o.tgtNext = 0
	}

	tgts = append(tgts, o.tgts[o.tgtNext])
	o.tgtNext++
	return
}

func (o *output) getTargetsHash(key []byte) (tgts []*target) {
	id := xxhash.Sum64(key) % o.tgtCnt
	tgts = append(tgts, o.tgts[id])
	return
}

func (o *output) getTargetsBroadcast(key []byte) (tgts []*target) {
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
