package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	rpb "github.com/blind-oracle/riemann-relay/riemannpb"
	hhash "github.com/minio/highwayhash"
)

const (
	ringIntervals = 1024
)

var (
	hashKey = []byte{
		0xf7, 0x74, 0x6b, 0xd7, 0xc2, 0x19, 0xe4, 0xa8,
		0xc4, 0x8d, 0xc3, 0xd5, 0x0f, 0x7b, 0x1f, 0x54,
		0x46, 0xa5, 0xdf, 0x7c, 0x64, 0x55, 0x1c, 0x8d,
		0x77, 0x94, 0xbb, 0x5d, 0x9f, 0x63, 0x54, 0x63,
	}
)

type output struct {
	name string
	typ  outputType

	algo         outputAlgo
	algoFailover bool

	hashFields    []riemannFieldName
	riemannFields []riemannFieldName
	riemannValue  riemannValue

	tgts              []*target
	tgtsAlive         []*target
	tgtsAliveMap      map[int]*target
	tgtsAliveRing     []*target
	tgtsAliveCnt      int
	tgtsRingIntervals uint64
	tgtCnt            int
	tgtNext           int
	tgtMtx            sync.Mutex
	tgtsPool          sync.Pool

	wg sync.WaitGroup

	getTargets func([]*target, []byte) int

	stats struct {
		received              uint64
		droppedNoTargetsAlive uint64
		droppedBufferOverflow uint64
	}

	chanShutdown chan struct{}

	rnd *rand.Rand
	*logger
	sync.RWMutex
}

func makeTargetsRing(tgts []*target) []*target {
	rounded := (ringIntervals / len(tgts)) * len(tgts)
	ring := make([]*target, rounded)
	tgtCnt := len(tgts)

	for i, j := 0, 0; i < rounded; i++ {
		ring[i] = tgts[j]

		if j++; j >= tgtCnt {
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

		tgtCnt:       len(c.Targets),
		tgtsAliveMap: map[int]*target{},
		chanShutdown: make(chan struct{}),
		rnd:          rand.New(rand.NewSource(time.Now().UnixNano())),
		logger:       &logger{fmt.Sprintf("Output %s", c.Name)},
	}

	o.tgtsPool = sync.Pool{
		New: func() interface{} {
			return make([]*target, o.tgtCnt)
		},
	}

	switch algo {
	case outputAlgoFailover:
		o.getTargets = o.getTargetsAlive
		o.algoFailover = true
	case outputAlgoRoundRobin:
		o.getTargets = o.getTargetsRoundRobin
	case outputAlgoHash:
		o.getTargets = o.getTargetsHash
	case outputAlgoBroadcast:
		o.getTargets = o.getTargetsAlive
	default:
		return nil, fmt.Errorf("Unknown algo: %s", algo)
	}

	o.Infof("Starting output (type '%s', algo '%s')", typ, algo)

	if algo == outputAlgoHash {
		if o.hashFields, err = parseRiemannFields(c.HashFields, true); err != nil {
			return
		}

		if len(o.hashFields) == 0 {
			return nil, fmt.Errorf("You need to specify 'hash_fields'")
		}

		o.Infof("Hash fields: %v", o.hashFields)
	}

	switch typ {
	case outputTypeCarbon, outputTypeClickhouse:
		onlyStrings := true
		if typ == outputTypeClickhouse {
			onlyStrings = false
		}

		if o.riemannFields, err = parseRiemannFields(c.RiemannFields, onlyStrings); err != nil {
			return
		}

		if len(o.riemannFields) == 0 {
			return nil, fmt.Errorf("You need to specify 'riemann_fields' for this output type")
		}

		if o.riemannValue, ok = riemannValueMap[c.RiemannValue]; !ok {
			return nil, fmt.Errorf("Unknown 'riemann_value': %s", c.RiemannValue)
		}

		o.Infof("Riemann fields: %v, value: %s", o.riemannFields, o.riemannValue)
	}

	for id, h := range c.Targets {
		tgt, err := newOutputTgt(h, c, o)
		if err != nil {
			return nil, fmt.Errorf("Unable to create target '%s': %s", h, err)
		}

		tgt.id = id
		o.tgts = append(o.tgts, tgt)

		switch typ {
		case outputTypeClickhouse, outputTypeFlatbuf:
			for _, conn := range tgt.conns {
				tgt.setConnAlive(conn.id, true)
			}
		}
	}

	if cfg.StatsInterval.Duration > 0 {
		o.wg.Add(1)
		go o.statsTicker()
	}

	o.Infof("Running")
	return o, nil
}

func (o *output) pushBatch(batch []*rpb.Event) {
	var (
		key                   *bytes.Buffer
		tgts                  = o.tgtsPool.Get().([]*target)
		n                     int
		droppedBufferOverflow uint64
	)

	if o.algo == outputAlgoHash {
		key = bufferPoolSmall.Get().(*bytes.Buffer)
	}

	l := len(batch)
	atomic.AddUint64(&o.stats.received, uint64(l))
	promOutReceived.WithLabelValues(o.name).Add(float64(l))

	o.RLock()
	if o.tgtsAliveCnt == 0 {
		atomic.AddUint64(&o.stats.droppedNoTargetsAlive, uint64(l))
		promOutDroppedNoTargetsAlive.WithLabelValues(o.name).Add(float64(l))
		o.RUnlock()
		return
	}

	for _, e := range batch {
		// Compute a list of targets to send
		if key != nil {
			eventWriteCompileFields(key, e, o.hashFields, '.')
			n = o.getTargets(tgts, key.Bytes())
			key.Reset()
		} else {
			n = o.getTargets(tgts, nil)
		}

		// Push the event to all selected targets
		ok := false
		for _, t := range tgts[:n] {
			if t.push(e) {
				ok = true
				if o.algo != outputAlgoBroadcast {
					break
				}
			} else {
				if !o.algoFailover {
					break
				}
			}
		}

		if !ok {
			droppedBufferOverflow++
		}
	}

	atomic.AddUint64(&o.stats.droppedBufferOverflow, droppedBufferOverflow)
	promOutDroppedBufferFull.WithLabelValues(o.name).Add(float64(droppedBufferOverflow))

	o.tgtsPool.Put(tgts)
	if key != nil {
		bufferPoolSmall.Put(key)
	}

	o.RUnlock()
}

func (o *output) setTgtAlive(id int, s bool) {
	o.Lock()
	if s {
		o.tgtsAliveMap[id] = o.tgts[id]
	} else {
		delete(o.tgtsAliveMap, id)
	}

	o.tgtsAliveCnt = len(o.tgtsAliveMap)
	o.tgtsAlive = make([]*target, o.tgtsAliveCnt)
	i := 0
	for _, v := range o.tgtsAliveMap {
		o.tgtsAlive[i] = v
		i++
	}

	if o.tgtsAliveCnt > 0 && o.algo == outputAlgoHash {
		o.tgtsAliveRing = makeTargetsRing(o.tgtsAlive)
		o.tgtsRingIntervals = uint64(len(o.tgtsAliveRing))
	} else {
		o.tgtsRingIntervals = 0
	}

	o.Unlock()
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

func (o *output) getTargetsAlive(tgts []*target, key []byte) int {
	return copy(tgts, o.tgtsAlive)
}

func (o *output) getTargetsRoundRobin(tgts []*target, key []byte) (n int) {
	o.tgtMtx.Lock()
	i := o.tgtNext
	if o.tgtNext++; o.tgtNext >= o.tgtCnt {
		o.tgtNext = 0
	}
	o.tgtMtx.Unlock()

	j := 0
	for j < o.tgtsAliveCnt {
		tgts[j] = o.tgtsAlive[i]
		j++

		if i++; i >= o.tgtsAliveCnt {
			i = 0
		}
	}

	n = o.tgtsAliveCnt
	return
}

func (o *output) getTargetsHash(tgts []*target, key []byte) (n int) {
	i := hhash.Sum64(key, hashKey) % o.tgtsRingIntervals
	j := 0

	for j < o.tgtsAliveCnt {
		tgts[j] = o.tgtsAliveRing[i]
		j++

		if i++; i >= o.tgtsRingIntervals {
			i = 0
		}
	}

	n = o.tgtsAliveCnt
	return
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
	s = append(s, fmt.Sprintf("received %d droppedNoTargetsAlive %d droppedBufferOverflow %d",
		atomic.LoadUint64(&o.stats.received),
		atomic.LoadUint64(&o.stats.droppedNoTargetsAlive),
		atomic.LoadUint64(&o.stats.droppedBufferOverflow),
	))

	o.RLock()
	for _, t := range o.tgts {
		s = append(s, fmt.Sprintf(" %s:", t.host))
		s = append(s, t.getStats()...)
	}
	o.RUnlock()

	return
}
