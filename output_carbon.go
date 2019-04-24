package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/segmentio/fasthash/fnv1a"
	log "github.com/sirupsen/logrus"
)

type targetCarbon struct {
	host string
	conn net.Conn

	connTimeout time.Duration
	timeout     time.Duration

	o      *outputCarbon
	ChanIn chan string
	alive  bool

	stats struct {
		connFailed uint64
		dropped    uint64
	}

	sync.RWMutex
}

func (t *targetCarbon) run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	var err error

	for {
		select {
		case <-ctx.Done():
			return

		default:
			t.o.Infof("%s: Connecting", t.host)
			if t.conn, err = t.connect(ctx); err != nil {
				atomic.AddUint64(&t.stats.connFailed, 1)
				t.o.Errorf("Connection to '%s' failed: %s", t.host, err)
				time.Sleep(5 * time.Second)
				continue
			}
			t.setState(true)

			t.o.Infof("%s: Connection established", t.host)
			if err = t.process(ctx); err != nil {
				t.o.Warnf("%s: Connection broken: %s", t.host, err)
			}

			t.conn.Close()
			t.setState(false)
		}
	}
}

func (t *targetCarbon) setState(s bool) {
	t.Lock()
	t.alive = s
	t.Unlock()
}

func (t *targetCarbon) isAlive() bool {
	t.RLock()
	defer t.RUnlock()
	return t.alive
}

func (t *targetCarbon) connect(ctx context.Context) (c net.Conn, err error) {
	dialer := &net.Dialer{
		Timeout: t.connTimeout,
	}

	return dialer.DialContext(ctx, "tcp", t.host)
}

func (t *targetCarbon) process(ctx context.Context) (err error) {
	var m string

	for {
		select {
		case <-ctx.Done():
			return

		case m = <-t.ChanIn:
			if err = t.sendMetrics(ctx, m); err != nil {
				t.o.Errorf("%s: Unable to send metric: %s", t.host, err)
				return
			}
		}
	}
}

func (t *targetCarbon) sendMetrics(ctx context.Context, m string) (err error) {
	_, err = t.conn.Write([]byte(m))
	return
}

func (t *targetCarbon) getStats() string {
	return fmt.Sprintf("dropped %d connFailed %d", atomic.LoadUint64(&t.stats.dropped), atomic.LoadUint64(&t.stats.connFailed))
}

type outputCarbon struct {
	name string

	algo    outputAlgo
	targets []*targetCarbon
	next    int

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	getTargets func(string) []*targetCarbon

	stats struct {
		dropped uint64
	}

	ChanIn chan string
}

func newOutputCarbon(name string, targets []string, connTimeout time.Duration, timeout time.Duration, algo string, bufferSize int) (*outputCarbon, error) {
	o := &outputCarbon{
		ChanIn: make(chan string, bufferSize),
	}

	o.ctx, o.cancel = context.WithCancel(context.Background())

	for _, h := range targets {
		t := &targetCarbon{
			host:        h,
			connTimeout: connTimeout,
			timeout:     timeout,
			o:           o,

			ChanIn: make(chan string, bufferSize),
		}

		o.targets = append(o.targets, t)
		o.wg.Add(1)
		go t.run(o.ctx, &o.wg)
	}

	a, ok := outputAlgoMap[algo]
	if !ok {
		return nil, fmt.Errorf("Unknown algorithm '%s'", algo)
	}

	switch a {
	case outputAlgoFailover:
		o.getTargets = o.getTargetsFailover
	case outputAlgoRoundRobin:
		o.getTargets = o.getTargetsRoundRobin
	case outputAlgoHash:
		o.getTargets = o.getTargetsHash
	}

	return o, nil
}

func (o *outputCarbon) run() {
	var (
		m    string
		tgts []*targetCarbon
	)

	for {
		select {
		case <-o.ctx.Done():
			return
		case m = <-o.ChanIn:
			tgts = o.getTargets(m)
			if len(tgts) == 0 {
				atomic.AddUint64(&o.stats.dropped, 1)
				continue
			}

			for _, t := range tgts {
				select {
				case t.ChanIn <- m:
				default:
					atomic.AddUint64(&t.stats.dropped, 1)
				}
			}
		}
	}
}

func (o *outputCarbon) getTargetsFailover(m string) (tgts []*targetCarbon) {
	for _, t := range o.targets {
		if t.isAlive() {
			tgts = append(tgts, t)
			return
		}
	}

	return
}

func (o *outputCarbon) getTargetsRoundRobin(m string) (tgts []*targetCarbon) {
	if o.next >= len(o.targets) {
		o.next = 0
	}

	tgts = append(tgts, o.targets[o.next])
	o.next++
	return
}

func (o *outputCarbon) getTargetsHash(m string) (tgts []*targetCarbon) {
	name := strings.SplitN(m, " ", 1)[0]
	hash := fnv1a.HashString64(name)
	id := hash % uint64(len(o.targets))
	tgts = append(tgts, o.targets[id])
	return
}

func (o *outputCarbon) Infof(msg string, args ...interface{}) {
	log.Infof(o.name+": "+msg, args...)
}

func (o *outputCarbon) Warnf(msg string, args ...interface{}) {
	log.Warnf(o.name+": "+msg, args...)
}

func (o *outputCarbon) Errorf(msg string, args ...interface{}) {
	log.Errorf(o.name+": "+msg, args...)
}

func eventToCarbon(e *Event) (string, error) {
	pfx, err := getPrefix(e)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s.%s.%s %f %d", pfx, e.Host, e.Service, e.MetricF, e.Time), nil
}

func getPrefix(e *Event) (string, error) {
	for _, a := range e.Attributes {
		if a.Key == "prefix" {
			return a.Value, nil
		}
	}

	return "", fmt.Errorf("No 'prefix' found in attributes")
}
