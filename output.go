package main

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	hhash "github.com/minio/highwayhash"
)

const (
	ringIntervals = 8192
)

type output struct {
	name string
	typ  outputType

	algo         outputAlgo
	algoFailover bool

	hashFields   []riemannFieldName
	carbonFields []riemannFieldName
	carbonValue  riemannValue

	tgts     []*target
	tgtsRing []*target
	tgtCnt   uint64
	tgtNext  int

	wg sync.WaitGroup

	getTargets func([]byte) []*target

	stats struct {
		received         uint64
		droppedNoTargets uint64
	}

	chanShutdown chan struct{}

	rnd *rand.Rand
	*logger
}

func makeTargetsRing(tgts []*target) []*target {
	ring := make([]*target, ringIntervals)

	for i, j := 0, 0; i < ringIntervals; i++ {
		ring[i] = tgts[j]

		if j++; j >= len(tgts) {
			j = 0
		}
	}

	return ring
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

	o = &output{
		name: c.Name,
		typ:  typ,

		algo:         algo,
		algoFailover: c.AlgoFailover,

		tgtCnt:       uint64(len(c.Targets)),
		chanShutdown: make(chan struct{}),
		rnd:          rand.New(rand.NewSource(time.Now().UnixNano())),
		logger:       &logger{fmt.Sprintf("Output %s", c.Name)},
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
	default:
		return nil, fmt.Errorf("Unknown algo: %s", algo)
	}

	o.Infof("Starting output (type '%s', algo '%s')", typ, algo)

	if algo == outputAlgoHash {
		if o.hashFields, err = parseRiemannFields(c.HashFields); err != nil {
			return
		}

		if len(o.hashFields) == 0 {
			return nil, fmt.Errorf("You need to specify 'hash_fields'")
		}

		o.Infof("Hash fields: %v", o.hashFields)
	}

	if typ == outputTypeCarbon {
		if o.carbonFields, err = parseRiemannFields(c.CarbonFields); err != nil {
			return
		}

		if len(o.carbonFields) == 0 {
			return nil, fmt.Errorf("You need to specify 'carbon_fields' for output type 'carbon'")
		}

		if o.carbonValue, ok = riemannValueMap[c.CarbonValue]; !ok {
			return nil, fmt.Errorf("Unknown 'carbon_value': %s", c.CarbonValue)
		}

		o.Infof("Carbon fields: %v, value: %s", o.carbonFields, o.carbonValue)
	}

	for _, h := range c.Targets {
		o.tgts = append(o.tgts, newOutputTgt(h, c, o))
	}

	if algo == outputAlgoHash {
		o.tgtsRing = makeTargetsRing(o.tgts)
	}

	if cfg.StatsInterval.Duration > 0 {
		o.wg.Add(1)
		go o.statsTicker()
	}

	o.Infof("Running")
	return o, nil
}

func (o *output) pushBatch(batch []*Event) {
	var (
		key  []byte
		tgts []*target
	)

	l := uint64(len(batch))
	atomic.AddUint64(&o.stats.received, l)
	promOutReceived.WithLabelValues(o.name).Add(float64(l))

	for _, e := range batch {
		if o.algo == outputAlgoHash {
			key = o.getKey(e)
			o.Debugf("Hash key: %s", key)
		}

		// Compute a list of targets to send
		if tgts = o.getTargets(key); len(tgts) == 0 {
			o.Debugf("No targets, dropping event")
			atomic.AddUint64(&o.stats.droppedNoTargets, 1)
			promOutDroppedNoTargets.WithLabelValues(o.name).Add(1)
			continue
		}

		// Push the event to all selected targets
		for _, t := range tgts {
			t.push(e)
		}
	}
}

func (o *output) close() {
	close(o.chanShutdown)
	o.wg.Wait()

	o.Warnf("Closing targets...")
	for _, t := range o.tgts {
		t.close()
	}

	o.Warnf("All targets closed")
}

func (o *output) getKey(e *Event) []byte {
	return eventCompileFields(e, o.hashFields, ".")
}

func (o *output) getLiveTarget(random bool) (tgts []*target) {
	for _, t := range o.tgts {
		if t.isAlive() {
			tgts = append(tgts, t)
			if !random {
				return
			}
		}
	}

	if len(tgts) == 0 {
		return
	}

	return []*target{
		tgts[o.rnd.Intn(len(tgts))],
	}
}

func (o *output) getTargetsFailover(key []byte) (tgts []*target) {
	return o.getLiveTarget(false)
}

func (o *output) getTargetsRoundRobin(key []byte) (tgts []*target) {
	if o.tgtNext >= len(o.tgts) {
		o.tgtNext = 0
	}

	tgts = []*target{o.tgts[o.tgtNext]}
	o.tgtNext++

	if o.algoFailover && !tgts[0].isAlive() {
		tgts = o.getLiveTarget(true)
	}

	return
}

func (o *output) getTargetsHash(key []byte) []*target {
	i := hhash.Sum64(key, hashKey) % ringIntervals
	left := o.tgtCnt

	var t *target
	for left > 0 {
		t = o.tgtsRing[i]

		if t.isAlive() {
			return []*target{t}
		}

		left--
		if i++; i >= ringIntervals {
			i = 0
		}
	}

	return []*target{}
}

func (o *output) getTargetsBroadcast(key []byte) (tgts []*target) {
	return o.tgts
}

func (o *output) statsTicker() {
	tick := time.NewTicker(cfg.StatsInterval.Duration)

	defer func() {
		tick.Stop()
		o.wg.Done()
	}()

	for {
		select {
		case <-o.chanShutdown:
			return

		case <-tick.C:
			for _, r := range o.getStats() {
				o.Infof(r)
			}
		}
	}
}

func (o *output) getStats() (s []string) {
	s = append(s, fmt.Sprintf("received %d droppedNoTargets %d",
		atomic.LoadUint64(&o.stats.received),
		atomic.LoadUint64(&o.stats.droppedNoTargets),
	))

	for _, t := range o.tgts {
		s = append(s, fmt.Sprintf(" %s:", t.host))
		s = append(s, t.getStats()...)
	}

	return
}
