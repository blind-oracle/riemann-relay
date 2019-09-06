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
	host string
	typ  outputType

	conns    []*tConn
	connCnt  int
	connNext int

	o *output

	stats struct {
		buffered uint64
		dropped  uint64
	}

	wg sync.WaitGroup

	connMtx sync.Mutex
	sync.RWMutex
	*logger
}

func newOutputTgt(h string, cf *outputCfg, o *output) (*target, error) {
	t := &target{
		host: h,
		typ:  o.typ,

		o:       o,
		connCnt: cf.Connections,

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

			c.Debugf("Clickhouse URL: %s", c.url)
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

func (t *target) isAlive() bool {
	for _, c := range t.conns {
		if c.isAlive() {
			return true
		}
	}

	return false
}

func (t *target) push(e *Event) {
	left := t.connCnt

	var c *tConn
	for left > 0 {
		t.Lock()
		if t.connNext >= t.connCnt {
			t.connNext = 0
		}

		c = t.conns[t.connNext]
		t.connNext++
		t.Unlock()

		if c.isAlive() && c.bufferEvent(e) {
			atomic.AddUint64(&t.stats.buffered, 1)
			promTgtBuffered.WithLabelValues(t.o.name, t.host).Add(1)
			return
		}

		left--
	}

	atomic.AddUint64(&t.stats.dropped, 1)
	promTgtDroppedBufferFull.WithLabelValues(t.o.name, t.host).Add(1)
}

func (t *target) getStats() (s []string) {
	var (
		cfT, sentT, ffT uint64
		bufT, szT       int
		rows            []string
	)

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

		r := fmt.Sprintf("   %d: buffered %d sent %d dropped %d connFailed %d flushFailed %d bufferFill %.3f",
			c.id,
			atomic.LoadUint64(&c.stats.buffered),
			sent,
			atomic.LoadUint64(&c.stats.dropped),
			cf,
			ff,
			float64(buf)/float64(sz),
		)

		rows = append(rows, r)
	}

	r := fmt.Sprintf("  buffered %d sent %d dropped %d connFailed %d flushFailed %d bufferFill %.3f",
		atomic.LoadUint64(&t.stats.buffered),
		sentT,
		atomic.LoadUint64(&t.stats.dropped),
		cfT,
		ffT,
		float64(bufT)/float64(szT),
	)

	return append(
		[]string{r},
		rows...,
	)
}
