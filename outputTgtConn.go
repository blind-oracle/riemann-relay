package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	rpb "github.com/blind-oracle/riemann-relay/riemannpb"
	pb "github.com/golang/protobuf/proto"
	fh "github.com/valyala/fasthttp"
)

type tConn struct {
	host string
	url  string
	id   int

	conn net.Conn
	//httpCli *http.Client
	httpCli *fh.Client

	alive bool

	t *target

	batch struct {
		buf     []*rpb.Event
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
	chanIn    chan *rpb.Event

	ctx       context.Context
	ctxCancel context.CancelFunc

	writeBatch writeBatchFunc

	wg      sync.WaitGroup
	connMtx sync.Mutex

	stats struct {
		buffered    uint64
		sent        uint64
		dropped     uint64
		connFailed  uint64
		flushFailed uint64
	}

	*logger
	sync.RWMutex
}

func (c *tConn) run(typ outputType) {
	defer c.wg.Done()
	c.connMtx.Lock()

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
					c.connMtx.Unlock()
					return
				case <-time.After(c.reconnectInterval):
				}

				continue loop
			}

			c.Lock()
			c.conn = newTimeoutConn(conn, c.timeoutRead, c.timeoutWrite)
			c.alive = true
			c.t.setConnAlive(c.id, true)

			if typ == outputTypeCarbon {
				go c.checkEOF()
			}

			c.chanClose = make(chan struct{})
			c.Unlock()
			c.Infof("Connection established")

			c.connMtx.Unlock()

			select {
			case <-c.ctx.Done():
				return

			case <-c.chanClose:
				c.connMtx.Lock()
				c.Errorf("Connection broken")

				select {
				case <-c.ctx.Done():
					c.connMtx.Unlock()
					return
				case <-time.After(c.reconnectInterval):
				}
			}
		}
	}
}

func (c *tConn) isAlive() (o bool) {
	c.RLock()
	o = c.alive
	c.RUnlock()
	return
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
	if c.httpCli != nil {
		return
	}

	c.Lock()
	defer c.Unlock()

	if !c.alive {
		return
	}

	if err := c.conn.Close(); err != nil {
		c.Errorf("Unable to close connection: %s", err)
	}

	c.alive = false
	c.t.setConnAlive(c.id, false)
	close(c.chanClose)
}

func (c *tConn) dispatch() {
	defer c.wg.Done()

	var (
		e   *rpb.Event
		err error
	)

	for {
		select {
		case <-c.ctx.Done():
			if !c.isAlive() {
				c.Warnf("Connection is down, discarding %d events in buffer", len(c.chanIn))
				return
			}

			// Flush channel
			c.Warnf("Flushing buffer (%d events)", len(c.chanIn))

			for {
				select {
				case e = <-c.chanIn:
					if err = c.push(e); err != nil {
						c.Errorf("Unable to flush: %s", err)
						return
					}

				default:
					if err = c.tryFlush(); err != nil {
						c.Errorf("Unable to flush: %s", err)
					} else {
						c.Warnf("Buffer flushed")
					}

					return
				}
			}

		case e = <-c.chanIn:
			c.connMtx.Lock()
			if err = c.push(e); err != nil {
				c.Errorf("Unable to flush batch: %s", err)
				// Requeue the event
				c.chanIn <- e
			}
			c.connMtx.Unlock()
		}
	}
}

func (c *tConn) bufferEvent(e *rpb.Event) bool {
	select {
	case c.chanIn <- e:
		atomic.AddUint64(&c.stats.buffered, 1)
		return true
	default:
		atomic.AddUint64(&c.stats.dropped, 1)
		return false
	}
}

func (c *tConn) push(e *rpb.Event) (err error) {
	c.batch.Lock()
	defer c.batch.Unlock()

	if c.batch.count >= c.batch.size {
		if err = c.flush(); err != nil {
			return
		}
	}

	c.batch.buf[c.batch.count] = e
	c.batch.count++
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
			c.connMtx.Lock()
			c.tryFlush()
			c.connMtx.Unlock()

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

	return c.flush()
}

// Assumes locked batch
func (c *tConn) flush() (err error) {
	ts := time.Now()

	if err = c.writeBatch(c.batch.buf[:c.batch.count]); err != nil {
		atomic.AddUint64(&c.stats.flushFailed, 1)
		promTgtFlushFailed.WithLabelValues(c.t.o.name, c.t.host).Add(1)
		c.Errorf("Unable to flush batch: %s", err)

		if !isErrClosedConn(err) {
			c.disconnect()
		}

		return
	}

	dur := time.Since(ts).Seconds()

	promTgtFlushDuration.WithLabelValues(c.t.o.name, c.t.host).Observe(dur)
	atomic.AddUint64(&c.stats.sent, uint64(c.batch.count))
	promTgtSent.WithLabelValues(c.t.o.name, c.host).Add(float64(c.batch.count))

	c.batch.count = 0
	return
}

func (c *tConn) close() {
	c.Infof("Closing...")

	c.ctxCancel()
	c.wg.Wait()

	if c.httpCli == nil {
		c.disconnect()
	}

	c.Warnf("Closed")
	return
}

func (c *tConn) writeBatchCarbon(batch []*rpb.Event) (err error) {
	b := bufferPool.Get().(*bytes.Buffer)

	for _, e := range batch {
		eventWriteCompileFields(b, e, c.t.o.riemannFields, '.')
		b.WriteByte(' ')

		val := strconv.FormatFloat(eventGetValue(e, c.t.o.riemannValue), 'f', -1, 64)
		b.WriteString(val)
		b.WriteByte(' ')
		b.WriteString(strconv.FormatInt(eventGetTime(e), 10))
		b.WriteByte('\n')
	}

	_, err = c.conn.Write(b.Bytes())

	b.Reset()
	bufferPool.Put(b)
	return
}

func (c *tConn) writeBatchRiemann(batch []*rpb.Event) (err error) {
	msg := &rpb.Msg{
		Events: batch,
	}

	buf, err := pb.Marshal(msg)
	if err != nil {
		return fmt.Errorf("Unable to marshal Protobuf: %s", err)
	}

	if err = binary.Write(c.conn, binary.BigEndian, uint32(len(buf))); err != nil {
		return fmt.Errorf("Unable to write Protobuf length: %s", err)
	}

	if _, err = c.conn.Write(buf); err != nil {
		return fmt.Errorf("Unable to write Protobuf body: %s", err)
	}

	var hdr uint32
	if err = binary.Read(c.conn, binary.BigEndian, &hdr); err != nil {
		if err == io.EOF {
			return err
		}

		return fmt.Errorf("Unable to read Protobuf reply length: %s", err)
	}

	buf = make([]byte, hdr)
	if err = readPacket(c.conn, buf); err != nil {
		return fmt.Errorf("Unable to read Protobuf reply body: %s", err)
	}

	msg.Reset()
	if err = pb.Unmarshal(buf, msg); err != nil {
		return fmt.Errorf("Unable to unmarshal Protobuf reply: %s", err)
	}

	if !msg.GetOk() {
		return fmt.Errorf("Non-OK reply from Riemann")
	}

	return
}

func (c *tConn) writeBatchClickhouse(batch []*rpb.Event) (err error) {
	rr, wr := io.Pipe()
	go func() {
		defer wr.Close()
		for _, e := range batch {
			if err := eventWriteClickhouseBinary(wr, e, c.t.o.riemannFields, c.t.o.riemannValue); err != nil {
				return
			}
		}
	}()

	req := fh.AcquireRequest()
	resp := fh.AcquireResponse()

	defer func() {
		fh.ReleaseRequest(req)
		fh.ReleaseResponse(resp)
	}()

	req.SetBodyStream(rr, -1)
	req.Header.SetMethod("POST")
	req.SetRequestURI(c.url)

	if err = c.httpCli.Do(req, resp); err != nil {
		return fmt.Errorf("HTTP request failed: %s", err)
	}

	if resp.Header.StatusCode() != 200 {
		return fmt.Errorf("HTTP code is not 200: %d (%s)", resp.Header.StatusCode(), string(resp.Body()))
	}

	return
}
