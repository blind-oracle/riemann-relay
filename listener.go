package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/golang/protobuf/proto"
)

type listener struct {
	listeners []*net.TCPListener
	timeout   time.Duration

	chanOut  chan []*Event
	shutdown chan struct{}

	wgAccept sync.WaitGroup
	wgConn   sync.WaitGroup
	wgReady  sync.WaitGroup

	stats struct {
		receivedBatches uint64
		receivedEvents  uint64
		dropped         uint64
	}

	conns map[string]*net.TCPConn
	sync.Mutex
	*logger
}

func newListener(addrs []string, timeout time.Duration, chanOut chan []*Event) (*listener, error) {
	l := &listener{
		timeout:  timeout,
		chanOut:  chanOut,
		shutdown: make(chan struct{}),
		conns:    map[string]*net.TCPConn{},
		logger:   &logger{"Listener"},
	}

	for _, addr := range addrs {
		lis, err := listen(addr)
		if err != nil {
			return nil, fmt.Errorf("Unable to listen to '%s': %s", addr, err)
		}

		l.listeners = append(l.listeners, lis)
		l.Infof("Listening to '%s'", addr)

		l.wgAccept.Add(1)
		l.wgReady.Add(1)
		go l.accept(lis)
	}

	if cfg.StatsInterval.Duration > 0 {
		go l.statsTicker()
	}

	l.wgReady.Wait()
	return l, nil
}

func listen(addr string) (*net.TCPListener, error) {
	laddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}

	return net.ListenTCP("tcp", laddr)
}

func (l *listener) statsTicker() {
	t := time.NewTicker(cfg.StatsInterval.Duration)
	defer t.Stop()

	for {
		select {
		case <-l.shutdown:
			return
		case <-t.C:
			l.Infof(l.getStats())
		}
	}
}

func (l *listener) getStats() string {
	return fmt.Sprintf("receivedBatches %d receivedEvents %d dropped %d",
		atomic.LoadUint64(&l.stats.receivedBatches),
		atomic.LoadUint64(&l.stats.receivedEvents),
		atomic.LoadUint64(&l.stats.dropped),
	)
}

func (l *listener) accept(lis *net.TCPListener) {
	defer l.wgAccept.Done()

	l.wgReady.Done()
	for {
		c, err := lis.AcceptTCP()
		if err != nil {
			select {
			case <-l.shutdown:
				l.Infof("%s: Accepter closing", lis.Addr())
				return
			default:
				l.Errorf("%s: Error accepting : %s", lis.Addr(), err)
			}
		}

		l.wgConn.Add(1)
		l.Infof("Connection to '%s' from '%s'", lis.Addr(), c.RemoteAddr())

		// Add connection to a map
		l.Lock()
		id := c.RemoteAddr().String()
		if cc, ok := l.conns[id]; ok {
			l.Warnf("Duplicate connection from '%s', closing old one", id)
			cc.Close()
		}
		l.conns[id] = c
		l.Unlock()

		go l.handleConnection(newTimeoutConn(c, l.timeout, l.timeout))
	}
}

func (l *listener) handleConnection(c net.Conn) {
	peer := c.RemoteAddr().String()

	defer func() {
		c.Close()
		l.Infof("%s: Connection closed", peer)

		// Remove connection from a map
		l.Lock()
		delete(l.conns, peer)
		l.Unlock()
		l.wgConn.Done()
	}()

	var err error
	for {
		if err = l.processMessage(c); err != nil {
			select {
			case <-l.shutdown:
				return
			default:
				if err != io.EOF {
					l.Warnf("%s: Unable to process message: %s", peer, err)
				}
			}

			return
		}
	}
}

func (l *listener) sendReply(ok bool, reason string, c net.Conn) (err error) {
	msg := &Msg{
		Ok:    ok,
		Error: reason,
	}

	buf := make([]byte, len(reason)+128)
	if buf, err = pb.Marshal(msg); err != nil {
		return fmt.Errorf("Unable to marshal Protobuf reply Msg: %s", err)
	}

	if err = binary.Write(c, binary.BigEndian, uint32(len(buf))); err != nil {
		return fmt.Errorf("Unable to write reply Protobuf length: %s", err)
	}

	if _, err = c.Write(buf); err != nil {
		return fmt.Errorf("Unable to write reply Protobuf body: %s", err)
	}

	return
}

func (l *listener) processMessage(c net.Conn) (err error) {
	peer := c.RemoteAddr().String()

	var hdr uint32
	if err = binary.Read(c, binary.BigEndian, &hdr); err != nil {
		if err == io.EOF {
			return err
		}

		return fmt.Errorf("Unable to read Protobuf length: %s", err)
	}

	buf := make([]byte, hdr)
	if err = readPacket(c, buf); err != nil {
		return fmt.Errorf("Unable to read Protobuf body: %s", err)
	}

	msg := Msg{}
	if err = pb.Unmarshal(buf, &msg); err != nil {
		err = fmt.Errorf("Unable to unmarshal Protobuf: %s", err)
		l.sendReply(false, err.Error(), c)
		return
	}

	l.Debugf("%s: Message with %d events decoded", peer, len(msg.Events))

	select {
	case l.chanOut <- msg.Events:
		atomic.AddUint64(&l.stats.receivedBatches, 1)
		atomic.AddUint64(&l.stats.receivedEvents, uint64(len(msg.Events)))
	case <-l.shutdown:
		return
	default:
		atomic.AddUint64(&l.stats.dropped, 1)
	}

	l.Debugf("%s: Message processing finished, OK sent", peer)
	return l.sendReply(true, "", c)
}

func (l *listener) Close() {
	l.Infof("Closing...")
	close(l.shutdown)

	// Shut down listeners
	for _, lis := range l.listeners {
		lis.Close()
	}
	l.wgAccept.Wait()

	// Close active connections
	l.Lock()
	l.Infof("%d active connections, closing", len(l.conns))
	for _, c := range l.conns {
		c.Close()
	}
	l.Unlock()
	l.wgConn.Wait()

	l.Infof("Closed")
}
