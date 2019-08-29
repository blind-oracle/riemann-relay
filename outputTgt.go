package main

import (
	"bytes"
	"context"
	"encoding/binary"
	fmt "fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"

	pb "github.com/golang/protobuf/proto"
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

func newOutputTgt(h string, cf *outputCfg, o *output) *target {
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

		if o.typ == outputTypeRiemann {
			c.timeoutRead = cf.TimeoutRead.Duration
		}

		switch o.typ {
		case outputTypeCarbon:
			c.writeBatch = t.writeBatchCarbon
		case outputTypeRiemann:
			c.writeBatch = t.writeBatchRiemann
		}

		t.conns = append(t.conns, c)

		c.wg.Add(3)
		go c.run(o.typ)
		go c.dispatch()
		go c.periodicFlush()
	}

	return t
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

func (t *target) writeBatchCarbon(c net.Conn, batch []*Event) (err error) {
	var buf bytes.Buffer

	t.Debugf("Preparing a batch of %d Carbon metrics", len(batch))
	for _, e := range batch {
		buf.Write(eventToCarbon(e, t.o.carbonFields, t.o.carbonValue))
		buf.WriteByte('\n')
	}

	t.Debugf("Sending batch")
	_, err = c.Write(buf.Bytes())
	return
}

func (t *target) writeBatchRiemann(c net.Conn, batch []*Event) (err error) {
	msg := &Msg{
		Events: batch,
	}

	buf, err := pb.Marshal(msg)
	if err != nil {
		return fmt.Errorf("Unable to marshal Protobuf: %s", err)
	}

	if err = binary.Write(c, binary.BigEndian, uint32(len(buf))); err != nil {
		return fmt.Errorf("Unable to write Protobuf length: %s", err)
	}

	if _, err = c.Write(buf); err != nil {
		return fmt.Errorf("Unable to write Protobuf body: %s", err)
	}
	t.Debugf("Protobuf message sent (%d bytes)", len(buf))

	var hdr uint32
	if err = binary.Read(c, binary.BigEndian, &hdr); err != nil {
		if err == io.EOF {
			return err
		}

		return fmt.Errorf("Unable to read Protobuf reply length: %s", err)
	}
	t.Debugf("Protobuf reply size read (%d bytes)", hdr)

	buf = make([]byte, hdr)
	if err = readPacket(c, buf); err != nil {
		return fmt.Errorf("Unable to read Protobuf reply body: %s", err)
	}
	t.Debugf("Protobuf reply body read")

	msg.Reset()
	if err = pb.Unmarshal(buf, msg); err != nil {
		return fmt.Errorf("Unable to unmarshal Protobuf reply: %s", err)
	}

	if !msg.GetOk() {
		return fmt.Errorf("Non-OK reply from Riemann")
	}

	t.Debugf("Protobuf reply body unmarshaled and OK")
	return
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

		r := fmt.Sprintf("   %d: buffered %d sent %d dropped %d connFailed %d flushFailed %d bufferFill %.1f%%",
			c.id,
			atomic.LoadUint64(&c.stats.buffered),
			sent,
			atomic.LoadUint64(&c.stats.dropped),
			cf,
			ff,
			float64(buf)/float64(sz)*100,
		)

		rows = append(rows, r)
	}

	r := fmt.Sprintf("  buffered %d sent %d dropped %d connFailed %d flushFailed %d bufferFill %.1f%%",
		atomic.LoadUint64(&t.stats.buffered),
		sentT,
		atomic.LoadUint64(&t.stats.dropped),
		cfT,
		ffT,
		float64(bufT)/float64(szT)*100,
	)

	return append(
		[]string{r},
		rows...,
	)
}
