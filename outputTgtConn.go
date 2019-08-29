package main

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type tConn struct {
	host string
	id   int

	conn  net.Conn
	alive bool

	t *target

	batch struct {
		buf     []*Event
		size    int
		count   int
		timeout time.Duration
		sync.Mutex
	}

	reconnectInterval time.Duration
	timeoutConnect    time.Duration
	timeoutRead       time.Duration
	timeoutWrite      time.Duration

	chanClose chan struct{}
	chanIn    chan *Event

	ctx       context.Context
	ctxCancel context.CancelFunc

	writeBatch writeBatchFunc

	wg sync.WaitGroup
	sync.RWMutex

	stats struct {
		buffered    uint64
		sent        uint64
		dropped     uint64
		connFailed  uint64
		flushFailed uint64
	}

	*logger
}

func (c *tConn) run(typ outputType) {
	defer c.wg.Done()

loop:
	for {
		select {
		case <-c.ctx.Done():
			return

		default:
			c.Infof("Connecting...")
			conn, err := c.connect()
			if err != nil {
				atomic.AddUint64(&c.stats.connFailed, 1)
				c.Errorf("Connection failed, will retry in %.1f sec: %s", c.reconnectInterval.Seconds(), err)

				select {
				case <-c.ctx.Done():
					return
				case <-time.After(c.reconnectInterval):
				}

				continue loop
			}

			c.Lock()
			c.conn = newTimeoutConn(conn, c.timeoutRead, c.timeoutWrite)
			c.alive = true
			c.Unlock()

			if typ == outputTypeCarbon {
				go c.checkEOF()
			}

			c.Infof("Connection established")
			c.chanClose = make(chan struct{})

			select {
			case <-c.ctx.Done():
				return

			case <-c.chanClose:
				c.Errorf("Connection broken")

				select {
				case <-c.ctx.Done():
					return
				case <-time.After(c.reconnectInterval):
				}
			}
		}
	}
}

func (c *tConn) isAlive() bool {
	c.RLock()
	defer c.RUnlock()
	return c.alive
}

func (c *tConn) checkEOF() {
	var (
		n   int
		err error
	)

	buf := make([]byte, 1024)
	for {
		if n, err = c.conn.Read(buf); err == io.EOF {
			c.Warnf("Connection closed by peer")
			c.disconnect()
			return
		} else if n != 0 {
			c.Warnf("Peer sent us something, should not happen")
			c.disconnect()
			return
		} else {
			// Some other error, probably closed connection etc, don't care
			return
		}
	}
}

func (c *tConn) connect() (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout:   c.timeoutConnect,
		KeepAlive: 30 * time.Second,
	}

	return dialer.DialContext(c.ctx, guessProto(c.host), c.host)
}

func (c *tConn) disconnect() {
	c.Lock()
	defer c.Unlock()

	if !c.alive {
		return
	}

	if err := c.conn.Close(); err != nil {
		c.Errorf("Unable to close connection: %s", err)
	}

	c.alive = false
	close(c.chanClose)
}

func (c *tConn) dispatch() {
	defer c.wg.Done()

	var (
		e   *Event
		err error
	)

	for {
		select {
		case <-c.ctx.Done():
			if !c.isAlive() {
				c.Warnf("Connection is down, discarding %d events in buffer", len(c.chanIn))

				// Flush the toilet
				for range c.chanIn {
				}

				return
			}

			// Flush channel
			c.Warnf("Flushing buffer (%d events)", len(c.chanIn))

			for {
				select {
				case e = <-c.chanIn:
					if err = c.push(e); err != nil {
						c.Errorf("Unable to flush batch: %s", err)
						return
					}

				default:
					c.Warnf("Buffer flushed")
					return
				}
			}

		case e = <-c.chanIn:
			if err = c.push(e); err != nil {
				c.Errorf("Unable to flush batch: %s", err)
				// Requeue the event
				c.chanIn <- e
				time.Sleep(1 * time.Second)
			}
		}
	}
}

func (c *tConn) bufferEvent(e *Event) bool {
	select {
	case c.chanIn <- e:
		atomic.AddUint64(&c.stats.buffered, 1)
		return true
	default:
		atomic.AddUint64(&c.stats.dropped, 1)
		return false
	}
}

func (c *tConn) push(e *Event) (err error) {
	c.batch.Lock()
	defer c.batch.Unlock()

	if c.batch.count >= c.batch.size {
		c.Debugf("Batch is full (%d/%d), flushing", c.batch.count, c.batch.size)

		if err = c.flush(); err != nil {
			return
		}
	}

	c.batch.buf[c.batch.count] = e
	c.batch.count++

	c.Debugf("Batch is now %d/%d", c.batch.count, c.batch.size)
	return nil
}

func (c *tConn) periodicFlush() {
	tick := time.NewTicker(c.batch.timeout)
	defer func() {
		tick.Stop()
		c.wg.Done()
	}()

	for {
		select {
		case <-tick.C:
			if c.isAlive() {
				c.tryFlush()
			}

		case <-c.ctx.Done():
			return
		}
	}
}

func (c *tConn) tryFlush() error {
	c.batch.Lock()
	defer c.batch.Unlock()

	if c.batch.count == 0 {
		return nil
	}

	c.Debugf("Time to flush the batch!")
	return c.flush()
}

// Assumes locked batch
func (c *tConn) flush() (err error) {
	c.Debugf("Waiting for the connection mutex")
	c.Lock()
	defer c.Unlock()

	c.Debugf("Flushing batch (%d events)", c.batch.count)
	ts := time.Now()

	if err = c.writeBatch(c.conn, c.batch.buf[:c.batch.count]); err != nil {
		atomic.AddUint64(&c.stats.flushFailed, 1)
		promTgtFlushFailed.WithLabelValues(c.t.o.name, c.t.host).Add(1)
		c.Errorf("Unable to flush batch: %s", err)

		if !isErrClosedConn(err) {
			c.disconnect()
		}

		return
	}

	dur := time.Since(ts).Seconds()
	c.Debugf("Batch flushed in %.2f sec", dur)

	promTgtFlushDuration.WithLabelValues(c.t.o.name, c.t.host).Observe(dur)
	atomic.AddUint64(&c.stats.sent, uint64(c.batch.count))
	promTgtSent.WithLabelValues(c.t.o.name, c.host).Add(float64(c.batch.count))

	c.batch.count = 0
	return
}

func (c *tConn) close() {
	c.ctxCancel()
	c.wg.Wait()
	c.disconnect()

	c.Warnf("Closed")
	return
}
