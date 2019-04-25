package main

import (
	"bytes"
	"context"
	fmt "fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type target struct {
	host string
	typ  outputType

	conn net.Conn

	connTimeout time.Duration
	timeout     time.Duration

	batch struct {
		buf     []*Event
		size    int
		count   int
		timeout time.Duration
		sync.Mutex
	}

	alive bool

	stats struct {
		processed  uint64
		connFailed uint64
		dropped    uint64
	}

	chanOpen     chan struct{}
	chanClose    chan struct{}
	chanDispatch chan struct{}
	chanIn       chan *Event

	writeBatch writeBatchFunc

	o *output

	ctx       context.Context
	ctxCancel context.CancelFunc

	wg         sync.WaitGroup
	wgDispatch sync.WaitGroup

	connMtx sync.RWMutex
	sync.RWMutex
	*logger
}

func (t *target) run() {
	defer t.wg.Done()

	t.wgDispatch.Add(1)
	go t.dispatch()

	t.wg.Add(1)
	go t.periodicFlush()

	t.connMtx.Lock()
	for {
		select {
		case <-t.ctx.Done():
			return

		default:
			t.Infof("Connecting...")
			c, err := t.connect()
			if err != nil {
				atomic.AddUint64(&t.stats.connFailed, 1)
				t.Errorf("Connection failed, will retry in 5 sec: %s", err)

				select {
				case <-t.ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}

				continue
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
				time.Sleep(5 * time.Second)
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
	close(t.chanDispatch)

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

	return dialer.DialContext(t.ctx, "tcp", t.host)
}

func (t *target) dispatch() {
	defer t.wgDispatch.Done()

	var (
		e  *Event
		ok bool
	)

	for {
		select {
		case <-t.chanDispatch:
			if !t.isAlive() {
				t.Infof("Connection is dead, will not flush buffers")
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
						t.Infof("%d events flushed", c)
						return
					}

					t.push(e)
					c++

				case <-dl:
					t.Warnf("Unable to flush buffers in 30 sec (%d events flushed), giving up", c)
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
	var (
		buf bytes.Buffer
		m   []byte
	)

	for _, e := range batch {
		if m, err = eventToCarbon(e); err != nil {
			return fmt.Errorf("Unable to convert event to Carbon, skipping: %s", err)
		}

		buf.Write(m)
		buf.WriteByte('\n')
	}

	_, err = t.conn.Write(buf.Bytes())
	return
}

func (t *target) writeBatchRiemann(batch []*Event) (err error) {
	return
}

func (t *target) push(e *Event) {
	t.batch.Lock()
	defer t.batch.Unlock()

	// Fall out if chan is already closed
	select {
	case <-t.chanDispatch:
		return
	default:
	}

	t.batch.buf[t.batch.count] = e
	t.batch.count++
	atomic.AddUint64(&t.stats.processed, 1)

	if t.batch.count >= t.batch.size {
		t.Debugf("Buffer is full (%d/%d), flushing", t.batch.count, t.batch.size)

	loop:
		for {
			select {
			case <-t.chanDispatch:
				return
			default:
				if err := t.flush(); err == nil {
					break loop
				}

				select {
				case <-t.chanDispatch:
					return
				case <-time.After(1 * time.Second):
				}
			}
		}
	}

	t.Debugf("Buffer is now %d/%d", t.batch.count, t.batch.size)
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

	t.flush()
	return
}

// Assumes locked batch
func (t *target) flush() (err error) {
	t.connMtx.Lock()
	defer t.connMtx.Unlock()

	t.Debugf("Flushing batch (%d events)", t.batch.count)
	ts := time.Now()

	if err = t.writeBatch(t.batch.buf[:t.batch.count]); err != nil {
		if !isErrClosedConn(err) {
			t.Errorf("Unable to flush batch: %s", err)
			t.disconnect()
		}

		return
	}

	t.batch.count = 0
	t.Debugf("Batch flushed in %.2f sec", time.Since(ts).Seconds())
	return
}

func (t *target) getStats() string {
	return fmt.Sprintf("processed %d dropped %d connFailed %d",
		atomic.LoadUint64(&t.stats.processed),
		atomic.LoadUint64(&t.stats.dropped),
		atomic.LoadUint64(&t.stats.connFailed),
	)
}
