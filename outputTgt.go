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
	"time"

	pb "github.com/golang/protobuf/proto"
)

type target struct {
	host string
	typ  outputType

	conn net.Conn

	reconnectInterval time.Duration
	connTimeout       time.Duration
	timeout           time.Duration

	batch struct {
		buf     []*Event
		size    int
		count   int
		timeout time.Duration
		sync.Mutex
	}

	alive bool

	stats struct {
		sent        uint64
		connFailed  uint64
		dropped     uint64
		flushFailed uint64
	}

	chanClose         chan struct{}
	chanDispatchClose chan struct{}
	chanIn            chan *Event

	writeBatch writeBatchFunc

	o *output

	ctx       context.Context
	ctxCancel context.CancelFunc

	wg         sync.WaitGroup
	wgDispatch sync.WaitGroup
	wgReady    sync.WaitGroup

	connMtx sync.Mutex
	sync.RWMutex
	*logger
}

func newOutputTgt(h string, c *outputCfg, o *output) *target {
	t := &target{
		host:              h,
		typ:               o.typ,
		reconnectInterval: c.ReconnectInterval.Duration,
		connTimeout:       c.ConnectTimeout.Duration,
		timeout:           c.Timeout.Duration,
		o:                 o,

		chanDispatchClose: make(chan struct{}),
		chanIn:            make(chan *Event, cfg.BufferSize),
		logger:            &logger{fmt.Sprintf("%s: %s", c.Name, h)},
	}

	t.ctx, t.ctxCancel = context.WithCancel(context.Background())

	switch o.typ {
	case outputTypeCarbon:
		t.writeBatch = t.writeBatchCarbon
	case outputTypeRiemann:
		t.writeBatch = t.writeBatchRiemann
	}

	t.batch.buf = make([]*Event, c.BatchSize)
	t.batch.size = c.BatchSize
	t.batch.timeout = c.BatchTimeout.Duration

	t.wg.Add(1)
	t.wgReady.Add(1)
	go t.run()

	t.wgReady.Wait()
	return t
}

func (t *target) run() {
	defer t.wg.Done()

	t.wgDispatch.Add(1)
	go t.dispatch()

	t.wg.Add(1)
	go t.periodicFlush()

	t.connMtx.Lock()

	t.wgReady.Done()
loop:
	for {
		select {
		case <-t.ctx.Done():
			return

		default:
			t.Infof("Connecting...")
			c, err := t.connect()
			if err != nil {
				atomic.AddUint64(&t.stats.connFailed, 1)
				promTgtConnFailed.WithLabelValues(t.o.name, t.host).Add(1)
				t.Errorf("Connection failed, will retry in %.1f sec: %s", t.reconnectInterval.Seconds(), err)

				select {
				case <-t.ctx.Done():
					return
				case <-time.After(t.reconnectInterval):
				}

				continue loop
			}

			t.conn = newTimeoutConn(c, 0, t.timeout)

			if t.typ == outputTypeCarbon {
				t.wg.Add(1)
				go t.checkEOF()
			}

			t.setAlive(true)
			t.Infof("Connection established")
			t.chanClose = make(chan struct{})
			t.connMtx.Unlock()

			select {
			case <-t.ctx.Done():
				return

			case <-t.chanClose:
				t.connMtx.Lock()
				t.Errorf("Connection broken")

				select {
				case <-t.ctx.Done():
					return
				case <-time.After(t.reconnectInterval):
				}
			}
		}
	}
}

func (t *target) checkEOF() {
	defer t.wg.Done()

	var (
		n   int
		err error
	)

	buf := make([]byte, 1024)
	for {
		if n, err = t.conn.Read(buf); err == io.EOF {
			t.Warnf("Connection closed by peer")
			t.disconnect()
			return
		} else if n != 0 {
			t.Warnf("Peer sent us something, should not happen")
			t.disconnect()
			return
		} else {
			// Some other error, probably closed connection etc, don't care
			return
		}
	}
}

func (t *target) setAlive(s bool) {
	t.Lock()
	t.alive = s
	t.Unlock()
}

func (t *target) isAlive() bool {
	t.RLock()
	defer t.RUnlock()
	return t.alive
}

func (t *target) disconnect() {
	t.Lock()
	defer t.Unlock()

	if !t.alive {
		return
	}

	t.conn.Close()
	t.alive = false
	close(t.chanClose)
}

func (t *target) Close() {
	t.Infof("Waiting for the dispatcher to flush...")
	close(t.chanIn)
	close(t.chanDispatchClose)

	// TODO potential race with run()
	if !t.isAlive() {
		t.connMtx.Unlock()
	}

	t.wgDispatch.Wait()
	t.Infof("Dispatcher flushed")

	t.setAlive(false)
	if t.conn != nil {
		t.conn.Close()
	}

	t.ctxCancel()
	t.wg.Wait()

	t.Infof("Closed")
}

func (t *target) connect() (c net.Conn, err error) {
	dialer := &net.Dialer{
		Timeout: t.connTimeout,
	}

	return dialer.DialContext(t.ctx, guessProto(t.host), t.host)
}

func (t *target) dispatch() {
	defer t.wgDispatch.Done()

	var (
		e  *Event
		ok bool
	)

	for {
		select {
		case <-t.chanDispatchClose:
			if !t.isAlive() {
				t.Infof("Connection is down, will not flush buffers")
				return
			}

			// Flush channel
			t.Infof("Flushing buffers...")
			dl := time.After(30 * time.Second)
			c := 0

			for {
				select {
				case e, ok = <-t.chanIn:
					if !ok {
						t.Infof("Buffer flushed (%d events)", c)
						return
					}

					t.push(e)
					c++

				case <-dl:
					t.Errorf("Unable to flush buffers in 30 sec (%d events flushed so far), giving up", c)
					return
				}
			}

		case e, ok = <-t.chanIn:
			if !ok {
				continue
			}

			t.push(e)
		}
	}
}

func (t *target) writeBatchCarbon(batch []*Event) (err error) {
	var buf bytes.Buffer

	t.Debugf("Preparing a batch of %d Carbon metrics", len(batch))
	for _, e := range batch {
		buf.Write(eventToCarbon(e, t.o.carbonFields, t.o.carbonValue))
		buf.WriteByte('\n')
	}

	t.Debugf("Sending batch")
	_, err = t.conn.Write(buf.Bytes())
	return
}

func (t *target) writeBatchRiemann(batch []*Event) (err error) {
	msg := &Msg{
		Events: batch,
	}

	buf, err := pb.Marshal(msg)
	if err != nil {
		return fmt.Errorf("Unable to marshal Protobuf: %s", err)
	}

	if err = binary.Write(t.conn, binary.BigEndian, uint32(len(buf))); err != nil {
		return fmt.Errorf("Unable to write Protobuf length: %s", err)
	}

	if _, err = t.conn.Write(buf); err != nil {
		return fmt.Errorf("Unable to write Protobuf body: %s", err)
	}
	t.Debugf("Protobuf message sent (%d bytes)", len(buf))

	var hdr uint32
	if err = binary.Read(t.conn, binary.BigEndian, &hdr); err != nil {
		if err == io.EOF {
			return err
		}

		return fmt.Errorf("Unable to read Protobuf reply length: %s", err)
	}
	t.Debugf("Protobuf reply size read (%d bytes)", hdr)

	buf = make([]byte, hdr)
	if err = readPacket(t.conn, buf); err != nil {
		return fmt.Errorf("Unable to read Protobuf reply body: %s", err)
	}
	t.Debugf("Protobuf reply body read")

	msg.Reset()
	if err = pb.Unmarshal(buf, msg); err != nil {
		return fmt.Errorf("Unable to unmarshal Protobuf reply: %s", err)
	}

	if !msg.Ok {
		return fmt.Errorf("Non-OK reply from Riemann")
	}

	t.Debugf("Protobuf reply body unmarshaled and OK")
	return
}

func (t *target) push(e *Event) {
	t.batch.Lock()
	defer t.batch.Unlock()

	// Fall out if we're shutting down
	select {
	case <-t.chanDispatchClose:
		return
	default:
	}

	t.batch.buf[t.batch.count] = e
	t.batch.count++
	atomic.AddUint64(&t.stats.sent, 1)
	promTgtSent.WithLabelValues(t.o.name, t.host).Add(1)

	if t.batch.count >= t.batch.size {
		t.Debugf("Batch is full (%d/%d), flushing", t.batch.count, t.batch.size)

	loop:
		for {
			select {
			case <-t.chanDispatchClose:
				return
			default:
				if err := t.flush(); err == nil {
					break loop
				}

				select {
				case <-t.chanDispatchClose:
					return
				case <-time.After(1 * time.Second):
				}
			}
		}
	}

	t.Debugf("Batch is now %d/%d", t.batch.count, t.batch.size)
}

func (t *target) periodicFlush() {
	defer t.wg.Done()
	tick := time.NewTicker(t.batch.timeout)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			if t.isAlive() {
				t.tryFlush()
			}

		// Flush the buffer on exit
		case <-t.ctx.Done():
			if t.isAlive() {
				t.tryFlush()
			}

			return
		}
	}
}

func (t *target) tryFlush() {
	t.batch.Lock()
	defer t.batch.Unlock()

	if t.batch.count == 0 {
		return
	}

	t.Debugf("Time to flush the batch!")
	t.flush()
}

// Assumes locked batch
func (t *target) flush() (err error) {
	t.Debugf("Waiting for the connection mutex")
	t.connMtx.Lock()
	defer t.connMtx.Unlock()

	t.Debugf("Flushing batch (%d events)", t.batch.count)
	ts := time.Now()

	if err = t.writeBatch(t.batch.buf[:t.batch.count]); err != nil {
		if !isErrClosedConn(err) {
			atomic.AddUint64(&t.stats.flushFailed, 1)
			promTgtFlushFailed.WithLabelValues(t.o.name, t.host).Add(1)
			t.Errorf("Unable to flush batch: %s", err)
			t.disconnect()
		}

		return
	}

	t.batch.count = 0

	dur := time.Since(ts).Seconds()
	t.Debugf("Batch flushed in %.2f sec", dur)
	promTgtFlushDuration.WithLabelValues(t.o.name, t.host).Observe(dur)

	return
}

func (t *target) getStats() string {
	return fmt.Sprintf("sent %d dropped %d connFailed %d flushFailed %d",
		atomic.LoadUint64(&t.stats.sent),
		atomic.LoadUint64(&t.stats.dropped),
		atomic.LoadUint64(&t.stats.connFailed),
		atomic.LoadUint64(&t.stats.flushFailed),
	)
}
