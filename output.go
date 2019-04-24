package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/segmentio/fasthash/fnv1a"
	log "github.com/sirupsen/logrus"
)

type outputAlgo int
type outputType int

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

type target struct {
	host string
	typ  outputType

	conn net.Conn

	connTimeout time.Duration
	timeout     time.Duration

	batch     []*Event
	batchSize int
	batchPos  int

	o     *output
	alive bool

	stats struct {
		connFailed uint64
		dropped    uint64
	}

	ChanIn chan *Event

	sync.RWMutex
}

func (t *target) run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	var err error

	for {
		select {
		case <-ctx.Done():
			t.conn.Close()
			return

		default:
			t.Infof("Connecting...", t.host)
			if t.conn, err = t.connect(ctx); err != nil {
				atomic.AddUint64(&t.stats.connFailed, 1)
				t.Errorf("Connection failed: %s", err)
				time.Sleep(5 * time.Second)
				continue
			}
			t.setState(true)

			t.Infof("Connection established", t.host)
			if err = t.process(ctx); err != nil {
				t.Warnf("Connection broken: %s", err)
			}

			t.conn.Close()
			t.setState(false)
		}
	}
}

func (t *target) checkEOF(ctx context.Context) {
	var (
		n   int
		err error
	)

	buf := make([]byte, 1024)

	for {
		if n, err = t.conn.Read(buf); err == io.EOF {
			t.Warnf("Connection closed by peer")
			t.conn.Close()
			return
		} else if n != 0 {
			t.Warnf("Peer sent us something, should not happen")
			t.conn.Close()
			return
		} else {
			t.Warnf("Unexpected read error: %s", err)
			t.conn.Close()
			return
		}
	}
}

func (t *target) setState(s bool) {
	t.Lock()
	t.alive = s
	t.Unlock()
}

func (t *target) isAlive() bool {
	t.RLock()
	defer t.RUnlock()
	return t.alive
}

func (t *target) connect(ctx context.Context) (c net.Conn, err error) {
	dialer := &net.Dialer{
		Timeout: t.connTimeout,
	}

	return dialer.DialContext(ctx, "tcp", t.host)
}

func (t *target) process(ctx context.Context) (err error) {
	//var ev []*Event

	for {
		select {
		case <-ctx.Done():
			return

			//case ev = <-t.ChanIn:
		}
	}
}

func (t *target) getStats() string {
	return fmt.Sprintf("dropped %d connFailed %d", atomic.LoadUint64(&t.stats.dropped), atomic.LoadUint64(&t.stats.connFailed))
}

func (t *target) Infof(msg string, args ...interface{}) {
	log.Infof(t.o.name+" ("+t.host+"): "+msg, args...)
}

func (t *target) Warnf(msg string, args ...interface{}) {
	log.Warnf(t.o.name+" ("+t.host+"): "+msg, args...)
}

func (t *target) Errorf(msg string, args ...interface{}) {
	log.Errorf(t.o.name+" ("+t.host+"): "+msg, args...)
}

type output struct {
	name string

	typ  outputType
	algo outputAlgo

	tgts    []*target
	tgtNext int

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	getTargets func(string) []*target

	stats struct {
		dropped uint64
	}

	ChanIn chan string
}

func newOutput(cfg outputCfg) (*output, error) {
	o := &output{
		ChanIn: make(chan string, cfg.BufferSize),
	}

	o.ctx, o.cancel = context.WithCancel(context.Background())

	for _, h := range cfg.Targets {
		t := &target{
			host:        h,
			connTimeout: cfg.ConnectTimeout.Duration,
			timeout:     cfg.Timeout.Duration,
			o:           o,

			ChanIn: make(chan *Event, cfg.BufferSize),
		}

		o.tgts = append(o.tgts, t)
		o.wg.Add(1)
		go t.run(o.ctx, &o.wg)
	}

	a, ok := outputAlgoMap[cfg.Algo]
	if !ok {
		return nil, fmt.Errorf("Unknown algorithm '%s'", cfg.Algo)
	}

	switch a {
	case outputAlgoFailover:
		o.getTargets = o.getTargetsFailover
	case outputAlgoRoundRobin:
		o.getTargets = o.getTargetsRoundRobin
	case outputAlgoHash:
		o.getTargets = o.getTargetsHash
	case outputAlgoBroadcast:
		o.getTargets = o.getTargetsBroadcast
	}

	return o, nil
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
	hash := fnv1a.HashString64(key)
	id := hash % uint64(len(o.tgts))
	tgts = append(tgts, o.tgts[id])
	return
}

func (o *output) getTargetsBroadcast(key string) (tgts []*target) {
	return o.tgts
}
