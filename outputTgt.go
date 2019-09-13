package main

import (
	"context"
	fmt "fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
)

type target struct {
	id   int
	host string
	typ  outputType

	conns         []*tConn
	connsAliveMap map[int]*tConn
	connsAlive    []*tConn
	connsAliveCnt int
	connsCnt      int
	connNext      int
	connMtx       sync.Mutex

	o *output

	stats struct {
		buffered uint64
		dropped  uint64
	}

	wg sync.WaitGroup

	sync.RWMutex
	*logger
}

func newOutputTgt(h string, cf *outputCfg, o *output) (*target, error) {
	t := &target{
		host: h,
		typ:  o.typ,

		o:             o,
		connsCnt:      cf.Connections,
		connsAliveMap: map[int]*tConn{},

		logger: &logger{fmt.Sprintf("%s: %s", cf.Name, h)},
	}

	for i := 0; i < cf.Connections; i++ {
		c := &tConn{
			t:    t,
			host: h,
			id:   i,

			reconnectInterval: cf.ReconnectInterval.Duration,
			timeoutConnect:    cf.TimeoutConnect.Duration,
			timeoutWrite:      cf.TimeoutWrite.Duration,

			chanIn: make(chan *Event, cf.BufferSize/cf.Connections),

			logger: &logger{fmt.Sprintf("%s: %s[%d]", cf.Name, h, i)},
		}

		c.batch.buf = make([]*Event, cf.BatchSize)
		c.batch.size = cf.BatchSize
		c.batch.timeout = cf.BatchTimeout.Duration

		c.ctx, c.ctxCancel = context.WithCancel(context.Background())

		switch o.typ {
		case outputTypeCarbon, outputTypeRiemann:
			if _, err := net.ResolveTCPAddr("tcp", h); err != nil {
				return nil, fmt.Errorf("Bad TCP address '%s': %s", h, err)
			}
		}

		switch o.typ {
		case outputTypeCarbon:
			c.writeBatch = c.writeBatchCarbon
		case outputTypeRiemann:
			c.timeoutRead = cf.TimeoutRead.Duration
			c.writeBatch = c.writeBatchRiemann
		case outputTypeClickhouse:
			if cf.CHTable == "" {
				return nil, fmt.Errorf("You need to specify 'ch_table'")
			}

			c.alive = true
			c.writeBatch = c.writeBatchClickhouse
			c.httpCli = &http.Client{
				Timeout: c.timeoutWrite,
			}

			u, err := url.Parse(h)
			if err != nil {
				return nil, fmt.Errorf("Unable to parse Clickhouse URL '%s': %s", h, err)
			}

			q := u.Query()
			q.Set("query", fmt.Sprintf("INSERT INTO %s FORMAT RowBinary", cf.CHTable))
			u.RawQuery = q.Encode()
			c.url = u.String()
		}

		t.conns = append(t.conns, c)

		switch o.typ {
		case outputTypeRiemann, outputTypeCarbon:
			c.wg.Add(1)
			go c.run(o.typ)
		}

		c.wg.Add(2)
		go c.dispatch()
		go c.periodicFlush()
	}

	return t, nil
}

func (t *target) close() {
	t.Warnf("Closing connections")
	for _, c := range t.conns {
		c.close()
	}

	t.Warnf("All connections closed")
	return
}

func (t *target) setConnAlive(id int, s bool) {
	t.Lock()

	if s {
		t.connsAliveMap[id] = t.conns[id]
	} else {
		delete(t.connsAliveMap, id)
	}

	t.connsAliveCnt = len(t.connsAliveMap)
	t.connsAlive = make([]*tConn, t.connsAliveCnt)
	i := 0
	for _, v := range t.connsAliveMap {
		t.connsAlive[i] = v
		i++
	}

	if t.connsAliveCnt == 0 {
		t.o.setTgtAlive(t.id, false)
	} else {
		t.o.setTgtAlive(t.id, true)
	}

	t.Unlock()
}

func (t *target) push(e *Event) bool {
	t.connMtx.Lock()
	next := t.connNext
	if t.connNext++; t.connNext >= t.connsCnt {
		t.connNext = 0
	}
	t.connMtx.Unlock()

	t.RLock()
	left := t.connsAliveCnt
	for left > 0 {
		if next >= t.connsAliveCnt {
			next = 0
		}

		if t.connsAlive[next].bufferEvent(e) {
			atomic.AddUint64(&t.stats.buffered, 1)
			promTgtBuffered.WithLabelValues(t.o.name, t.host).Add(1)
			t.RUnlock()
			return true
		}

		next++
		left--
	}
	t.RUnlock()

	atomic.AddUint64(&t.stats.dropped, 1)
	promTgtDroppedBufferFull.WithLabelValues(t.o.name, t.host).Add(1)

	return false
}

func (t *target) getStats() (s []string) {
	var (
		cfT, sentT, ffT uint64
		bufT, szT       int
		rows            []string
	)

	t.RLock()
	for _, c := range t.conns {
		cf := atomic.LoadUint64(&c.stats.connFailed)
		cfT += cf
		ff := atomic.LoadUint64(&c.stats.flushFailed)
		ffT += ff
		sent := atomic.LoadUint64(&c.stats.sent)
		sentT += sent
		buf, sz := len(c.chanIn), cap(c.chanIn)
		bufT += buf
		szT += sz

		_, alive := t.connsAliveMap[c.id]

		r := fmt.Sprintf("   %d: buffered %d sent %d dropped %d connFailed %d flushFailed %d bufferFill %.3f (alive: %t)",
			c.id,
			atomic.LoadUint64(&c.stats.buffered),
			sent,
			atomic.LoadUint64(&c.stats.dropped),
			cf,
			ff,
			float64(buf)/float64(sz),
			alive,
		)

		rows = append(rows, r)
	}
	t.RUnlock()

	r := fmt.Sprintf("  buffered %d sent %d dropped %d connFailed %d flushFailed %d bufferFill %.3f (alive: %t)",
		atomic.LoadUint64(&t.stats.buffered),
		sentT,
		atomic.LoadUint64(&t.stats.dropped),
		cfT,
		ffT,
		float64(bufT)/float64(szT),
		t.connsAliveCnt > 0,
	)

	return append(
		[]string{r},
		rows...,
	)
}
