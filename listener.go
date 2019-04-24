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
	log "github.com/sirupsen/logrus"
)

type listener struct {
	listeners []*net.TCPListener
	timeout   time.Duration

	chanOut  chan *Event
	shutdown chan struct{}

	acceptWg sync.WaitGroup
	connWg   sync.WaitGroup

	stats struct {
		dropped uint64
	}

	conns map[string]*net.TCPConn
	sync.Mutex
}

func newListener(addrs []string, timeout time.Duration, chanOut chan *Event) (*listener, error) {
	l := &listener{
		timeout:  timeout,
		chanOut:  chanOut,
		shutdown: make(chan struct{}),
		conns:    map[string]*net.TCPConn{},
	}

	for _, addr := range addrs {
		lis, err := listen(addr)
		if err != nil {
			return nil, fmt.Errorf("Unable to listen to '%s': %s", addr, err)
		}

		l.listeners = append(l.listeners, lis)
		log.Infof("Listening to '%s'", addr)

		l.acceptWg.Add(1)
		go l.accept(lis)
	}

	if cfg.StatsInterval.Duration > 0 {
		go l.statsTicker()
	}

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
			log.Infof("Listener: dropped %d", atomic.LoadUint64(&l.stats.dropped))
		}
	}
}

func (l *listener) accept(lis *net.TCPListener) {
	defer l.acceptWg.Done()

	for {
		c, err := lis.AcceptTCP()
		if err != nil {
			select {
			case <-l.shutdown:
				log.Infof("Accepter '%s' shutting down", lis.Addr())
				return
			default:
				log.Errorf("Error accepting on '%s': %s", lis.Addr(), err)
			}
		}

		l.connWg.Add(1)
		log.Infof("New connection on '%s' from '%s'", lis.Addr(), c.RemoteAddr())
		l.Lock()
		id := c.RemoteAddr().String()
		if cc, ok := l.conns[id]; ok {
			log.Warnf("Duplicate connection from '%s', closing old one", id)
			cc.Close()
		}
		l.conns[id] = c
		l.Unlock()
		go l.handleConnection(newTimeoutConn(c, l.timeout))
	}
}

func (l *listener) handleConnection(c net.Conn) {
	defer func() {
		c.Close()
		log.Infof("Connection from '%s' closed", c.RemoteAddr())
		l.Lock()
		delete(l.conns, c.RemoteAddr().String())
		l.Unlock()
		l.connWg.Done()
	}()

	var err error
	for {
		if err = l.processMessage(c); err != nil {
			select {
			case <-l.shutdown:
			default:
				if err != io.EOF {
					log.Warnf("Unable to process message: %s", err)
				}
			}

			return
		}

		log.Debugf("Message processed")
	}
}

func (l *listener) processMessage(c net.Conn) (err error) {
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
		return fmt.Errorf("Unable to unmarshal Protobuf: %s", err)
	}

	log.Debugf("Message with %d events decoded", len(msg.Events))

	for _, ev := range msg.Events {
		select {
		case l.chanOut <- ev:
		case <-l.shutdown:
			return
		default:
			atomic.AddUint64(&l.stats.dropped, 1)
		}
	}

	msg.Reset()
	msg.Ok = true
	if buf, err = pb.Marshal(&msg); err != nil {
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

func (l *listener) Close() {
	log.Info("Listener shutting down...")
	close(l.shutdown)

	// Shut down listeners
	for _, lis := range l.listeners {
		lis.Close()
	}
	l.acceptWg.Wait()

	// Close active connections
	l.Lock()
	log.Infof("%d active connections, closing them", len(l.conns))
	for _, c := range l.conns {
		log.Infof("Closing connection from '%s'", c.RemoteAddr())
		c.Close()
	}
	l.Unlock()
	l.connWg.Wait()

	log.Info("Listener closed")
}

func readPacket(r io.Reader, p []byte) error {
	for len(p) > 0 {
		n, err := r.Read(p)
		p = p[n:]
		if err != nil {
			return err
		}
	}

	return nil
}
