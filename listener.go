package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	pb "github.com/golang/protobuf/proto"
)

type wsError struct {
	Error string
}

type listener struct {
	listener   net.Listener
	listenerWS *http.Server
	wsUpgrader websocket.Upgrader

	timeout time.Duration

	chanOut  chan []*Event
	shutdown chan struct{}

	wgAccept sync.WaitGroup
	wgConn   sync.WaitGroup

	stats struct {
		receivedBatches uint64
		receivedEvents  uint64
		dropped         uint64
	}

	conns map[string]net.Conn
	sync.Mutex
	*logger
}

func newListener(chanOut chan []*Event) (l *listener, err error) {
	l = &listener{
		timeout:  cfg.Timeout.Duration,
		chanOut:  chanOut,
		shutdown: make(chan struct{}),
		conns:    map[string]net.Conn{},
		logger:   &logger{"Listener"},
	}

	l.wsUpgrader = websocket.Upgrader{
		HandshakeTimeout: l.timeout,
	}

	if cfg.Listen == "" && cfg.ListenWS == "" {
		return nil, fmt.Errorf("At least one of listenTCP/listenWS should be specified")
	}

	if cfg.Listen != "" {
		if l.listener, err = listen(cfg.Listen); err != nil {
			return nil, fmt.Errorf("Unable to listen to '%s': %s", cfg.Listen, err)
		}

		l.wgAccept.Add(1)
		go l.acceptTCP()
		l.Infof("Listening to '%s'", cfg.Listen)
	}

	if cfg.ListenWS != "" {
		mux := http.NewServeMux()
		mux.HandleFunc("/events", l.hanleWebsocketConnection)

		l.listenerWS = &http.Server{
			Handler:      mux,
			ReadTimeout:  l.timeout,
			WriteTimeout: l.timeout,
		}

		wsLis, err := listen(cfg.ListenWS)
		if err != nil {
			return nil, fmt.Errorf("Unable to Listen to Websocket HTTP: %s", err)
		}

		go func() {
			if err := l.listenerWS.Serve(wsLis); err != nil {
				if err == http.ErrServerClosed {
					return
				}

				l.Fatalf("Websocket HTTP listener error: %s", err)
			}
		}()

		l.Infof("Websocket listening to '%s'", cfg.ListenWS)
	}

	if cfg.StatsInterval.Duration > 0 {
		go l.statsTicker()
	}

	return l, nil
}

func listen(addr string) (net.Listener, error) {
	return net.Listen(guessProto(addr), addr)
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

func (l *listener) acceptTCP() {
	defer l.wgAccept.Done()

	for {
		c, err := l.listener.Accept()
		if err != nil {
			select {
			case <-l.shutdown:
				l.Infof("Accepter closing")
				return
			default:
				l.Errorf("Error accepting : %s", err)
			}
		}

		l.wgConn.Add(1)
		l.Infof("Connection from '%s'", c.RemoteAddr())

		// Add connection to a map
		l.Lock()
		id := c.RemoteAddr().String()
		if cc, ok := l.conns[id]; ok {
			l.Warnf("Duplicate connection from '%s', closing old one", id)
			cc.Close()
		}
		l.conns[id] = c
		l.Unlock()

		go l.handleTCPConnection(newTimeoutConn(c, l.timeout, l.timeout))
	}
}

func (l *listener) handleTCPConnection(c net.Conn) {
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
		if err = l.readTCPMessage(c); err != nil {
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

func (l *listener) sendReply(ok bool, reason string, c net.Conn) error {
	msg := &Msg{
		Ok:    ok,
		Error: reason,
	}

	buf, err := pb.Marshal(msg)
	if err != nil {
		return fmt.Errorf("Unable to marshal Protobuf reply Msg: %s", err)
	}

	if err = binary.Write(c, binary.BigEndian, uint32(len(buf))); err != nil {
		return fmt.Errorf("Unable to write reply Protobuf length: %s", err)
	}

	if _, err = c.Write(buf); err != nil {
		return fmt.Errorf("Unable to write reply Protobuf body: %s", err)
	}

	return nil
}

func (l *listener) hanleWebsocketConnection(w http.ResponseWriter, r *http.Request) {
	l.Infof("%s: Websocket connection", r.RemoteAddr)

	c, err := l.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		l.Errorf("%s: Websocket upgrade failed: %s", r.RemoteAddr, err)
	}

	defer func() {
		l.Infof("%s: Websocket connection closed", r.RemoteAddr)
		c.Close()
	}()

	var (
		wsMsgType int
		wsMsg     []byte
		ev        *Event
	)

	sendWSError := func(msg string) error {
		errMsg := &wsError{msg}
		js, _ := json.Marshal(errMsg)
		return c.WriteMessage(websocket.TextMessage, js)
	}

	for {
		if wsMsgType, wsMsg, err = c.ReadMessage(); err != nil {
			if websocket.IsCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}

			l.Errorf("%s: Unable to read Websocket message: %s", r.RemoteAddr, err)
			return
		}

		if wsMsgType != websocket.TextMessage {
			l.Warnf("%s: Unexpected message type received (%d), dropping", r.RemoteAddr, wsMsgType)
			if err = sendWSError("Unexpected message type"); err != nil {
				return
			}

			continue
		}

		if ev, err = eventFromJSON(wsMsg); err != nil {
			l.Warnf("%s: Unable to unmarshal Websocket message to JSON, dropping: %s", r.RemoteAddr, err)
			if err = sendWSError("Unable to parse event JSON"); err != nil {
				return
			}

			continue
		}

		l.sendEvents([]*Event{ev})
	}
}

func (l *listener) readTCPMessage(c net.Conn) (err error) {
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

	msg := &Msg{}
	if err = pb.Unmarshal(buf, msg); err != nil {
		l.Errorf("Unable to unmarshal Protobuf message: %s", err)
		// Don't disconnect just because of unmarshal error
		// Try to send error message and wait for next event batch
		return l.sendReply(false, "Unable to decode Protobuf message", c)
	}

	l.sendEvents(msg.Events)
	return l.sendReply(true, "", c)
}

func (l *listener) sendEvents(events []*Event) {
	for _, ev := range events {
		if ev.TimeMicros == 0 && ev.Time == 0 {
			ev.TimeMicros = time.Now().UnixNano() / 1000
		}
	}

	select {
	case l.chanOut <- events:
		atomic.AddUint64(&l.stats.receivedBatches, 1)
		atomic.AddUint64(&l.stats.receivedEvents, uint64(len(events)))
	case <-l.shutdown:
		return
	default:
		atomic.AddUint64(&l.stats.dropped, 1)
	}

	return
}

func (l *listener) Close() {
	l.Infof("Closing...")
	close(l.shutdown)

	if l.listener != nil {
		l.listener.Close()
		l.Infof("TCP closed")
	}

	if l.listenerWS != nil {
		ctx, cf := context.WithTimeout(context.Background(), 10*time.Second)
		l.listenerWS.Shutdown(ctx)
		cf()
		l.Infof("Websocket closed")
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
