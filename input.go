package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	rpb "github.com/blind-oracle/riemann-relay/riemannpb"
	"github.com/gorilla/websocket"

	pb "github.com/golang/protobuf/proto"
)

var (
	wsOkCloseReasons = []int{
		websocket.CloseAbnormalClosure,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
	}
)

type hFunc func([]*rpb.Event)

type wsError struct {
	Error string
}

type input struct {
	name string

	listener   net.Listener
	listenerWS *http.Server
	wsUpgrader websocket.Upgrader

	timeoutRead  time.Duration
	timeoutWrite time.Duration

	handlers map[string]hFunc
	shutdown chan struct{}

	wgAccept sync.WaitGroup
	wgConn   sync.WaitGroup

	stats struct {
		receivedBatches uint64
		receivedEvents  uint64
	}

	conns map[string]net.Conn
	sync.Mutex
	*logger
}

func newInput(c *inputCfg) (i *input, err error) {
	i = &input{
		name:         c.Name,
		handlers:     map[string]hFunc{},
		timeoutRead:  c.TimeoutRead.Duration,
		timeoutWrite: c.TimeoutWrite.Duration,
		shutdown:     make(chan struct{}),
		conns:        map[string]net.Conn{},
		logger:       &logger{fmt.Sprintf("Input %s", c.Name)},
	}

	if c.Listen == "" && c.ListenWS == "" {
		return nil, fmt.Errorf("At least one of listen/listen_ws should be specified")
	}

	if c.Listen != "" {
		if i.listener, err = listen(c.Listen); err != nil {
			return nil, fmt.Errorf("Unable to listen on '%s': %s", c.Listen, err)
		}

		i.wgAccept.Add(1)
		go i.acceptTCP()
		i.Infof("TCP: Listening to '%s'", c.Listen)
	}

	if c.ListenWS != "" {
		i.wsUpgrader = websocket.Upgrader{
			HandshakeTimeout: i.timeoutRead,
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/events", i.handleHTTPRequest)
		mux.HandleFunc("/events/", i.handleHTTPRequest)

		i.listenerWS = &http.Server{
			Handler:      mux,
			ReadTimeout:  i.timeoutRead,
			WriteTimeout: i.timeoutWrite,
		}

		wsLis, err := listen(c.ListenWS)
		if err != nil {
			return nil, fmt.Errorf("Unable to Listen to Websocket HTTP: %s", err)
		}

		go func() {
			if err := i.listenerWS.Serve(wsLis); err != nil {
				if err == http.ErrServerClosed {
					return
				}

				i.Fatalf("Websocket HTTP listener error: %s", err)
			}
		}()

		i.Infof("Websocket: Listening to '%s'", c.ListenWS)
	}

	if cfg.StatsInterval.Duration > 0 {
		go i.statsTicker()
	}

	i.Infof("Running")
	return i, nil
}

func listen(addr string) (net.Listener, error) {
	return net.Listen(guessProto(addr), addr)
}

func (i *input) addHandler(name string, hf hFunc) error {
	if _, ok := i.handlers[name]; ok {
		return fmt.Errorf("Output '%s' already registered", name)
	}

	i.handlers[name] = hf
	return nil
}

func (i *input) statsTicker() {
	t := time.NewTicker(cfg.StatsInterval.Duration)
	defer t.Stop()

	for {
		select {
		case <-i.shutdown:
			return
		case <-t.C:
			i.Infof(i.getStats())
		}
	}
}

func (i *input) getStats() string {
	i.Lock()
	defer i.Unlock()

	b := atomic.LoadUint64(&i.stats.receivedBatches)
	e := atomic.LoadUint64(&i.stats.receivedEvents)

	return fmt.Sprintf("receivedBatches %d receivedEvents %d (avg %.1f events/batch) conns %d",
		b,
		e,
		float64(e)/float64(b),
		len(i.conns),
	)
}

func (i *input) acceptTCP() {
	defer i.wgAccept.Done()

loop:
	for {
		c, err := i.listener.Accept()
		if err != nil {
			select {
			case <-i.shutdown:
				i.Warnf("Accepter closing")
				return
			default:
				i.Errorf("Error accepting : %s", err)
				time.Sleep(time.Second)
				continue loop
			}
		}

		i.wgConn.Add(1)
		i.Infof("Connection from '%s'", c.RemoteAddr())

		i.addConn(c)

		go i.handleTCPConnection(
			newTimeoutConn(c, i.timeoutRead, i.timeoutWrite),
		)
	}
}

func (i *input) addConn(c net.Conn) {
	i.Lock()
	id := c.RemoteAddr().String()
	if cc, ok := i.conns[id]; ok {
		i.Warnf("Duplicate connection from '%s', closing old one", id)
		cc.Close()
	}
	i.conns[id] = c
	i.Unlock()
}

func (i *input) delConn(c net.Conn) {
	i.Lock()
	id := c.RemoteAddr().String()
	delete(i.conns, id)
	i.Unlock()
}

func (i *input) handleTCPConnection(c net.Conn) {
	peer := c.RemoteAddr().String()

	defer func() {
		c.Close()
		i.Infof("%s: Connection closed", peer)

		i.delConn(c)
		i.wgConn.Done()
	}()

	var err error
	for {
		if err = i.readTCPMessage(c); err != nil {
			select {
			case <-i.shutdown:
				return

			default:
				if err != io.EOF {
					i.Warnf("%s: Unable to process message: %s", peer, err)
				}
			}

			return
		}
	}
}

func (i *input) sendReply(ok bool, reason string, c net.Conn) error {
	msg := &rpb.Msg{
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

func (i *input) handleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	i.Infof("%s: HTTP request (%s)", r.RemoteAddr, r.Method)

	switch r.Method {
	case http.MethodPut, http.MethodPost:
		i.handleHTTPEvent(w, r)
	case http.MethodGet:
		i.hanleWebsocketConnection(w, r)
	default:
		fmt.Fprintf(w, "Method '%s' not supported", r.Method)
		w.WriteHeader(400)
	}
}

func (i *input) hanleWebsocketConnection(w http.ResponseWriter, r *http.Request) {
	i.Infof("%s: Trying Websocket upgrade", r.RemoteAddr)
	c, err := i.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		i.Errorf("%s: Websocket upgrade failed: %s", r.RemoteAddr, err)
		return
	}

	i.addConn(c.UnderlyingConn())

	i.wgConn.Add(1)
	defer func() {
		i.wgConn.Done()
		i.Infof("%s: Websocket connection closed", r.RemoteAddr)
		c.Close()
		i.delConn(c.UnderlyingConn())
	}()

	var (
		wsMsgType int
		wsMsg     []byte
		ev        *rpb.Event
	)

	sendWSError := func(msg string) error {
		return c.WriteJSON(wsError{msg})
	}

	for {
		if wsMsgType, wsMsg, err = c.ReadMessage(); err != nil {
			if websocket.IsCloseError(err, wsOkCloseReasons...) {
				return
			}

			i.Errorf("%s: Unable to read Websocket message: %s", r.RemoteAddr, err)
			return
		}

		if wsMsgType != websocket.TextMessage {
			i.Warnf("%s: Unexpected message type received (%d), dropping", r.RemoteAddr, wsMsgType)
			if err = sendWSError("Unexpected message type"); err != nil {
				return
			}

			continue
		}

		if ev, err = eventFromJSON(wsMsg); err != nil {
			i.Warnf("%s: Unable to unmarshal Websocket message to JSON, dropping: %s", r.RemoteAddr, err)
			if err = sendWSError("Unable to parse event JSON"); err != nil {
				return
			}

			continue
		}

		i.sendEvents([]*rpb.Event{ev})
		if err = c.WriteMessage(websocket.TextMessage, []byte(`{}`)); err != nil {
			return
		}
	}
}

func (i *input) handleHTTPEvent(w http.ResponseWriter, r *http.Request) {
	i.wgConn.Add(1)

	defer func() {
		r.Body.Close()
		i.wgConn.Done()
	}()

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		i.Errorf("%s: Unable to read request body", r.RemoteAddr, err)
		return
	}

	evs, err := eventsFromMultipleJSONs(buf)
	if err != nil {
		i.Errorf("%s: Unable to parse event JSON: %s", r.RemoteAddr, err)
		http.Error(w, err.Error(), 400)
		return
	}

	i.Infof("%s: %d events parsed", r.RemoteAddr, len(evs))
	i.sendEvents(evs)
}

func (i *input) readTCPMessage(c net.Conn) (err error) {
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

	msg := &rpb.Msg{}
	if err = pb.Unmarshal(buf, msg); err != nil {
		i.Errorf("Unable to unmarshal Protobuf message: %s", err)
		// Don't disconnect just because of unmarshal error
		// Try to send error message and wait for next message
		return i.sendReply(false, "Unable to decode Protobuf message", c)
	}

	i.sendEvents(msg.Events)
	return i.sendReply(true, "", c)
}

func (i *input) sendEvents(events []*rpb.Event) {
	atomic.AddUint64(&i.stats.receivedBatches, 1)
	atomic.AddUint64(&i.stats.receivedEvents, uint64(len(events)))

	now := time.Now().UnixNano() / 1000
	for _, e := range events {
		if e.TimeMicros == 0 && e.Time == 0 {
			e.TimeMicros = now
		}

		e.MetricD = eventGetValue(e, riemannValueAny)
		e.MetricF = 0
		e.MetricSint64 = 0
	}

	for _, h := range i.handlers {
		h(events)
	}
}

func (i *input) close() {
	i.Warnf("Closing...")
	close(i.shutdown)

	if i.listener != nil {
		if err := i.listener.Close(); err != nil {
			i.Errorf("Unable to close TCP listener: %s", err)
		} else {
			i.Warnf("TCP listener closed")
		}
	}

	if i.listenerWS != nil {
		ctx, cf := context.WithTimeout(context.Background(), 10*time.Second)
		if err := i.listenerWS.Shutdown(ctx); err != nil {
			i.Errorf("Unable to close WS listener: %s", err)
		} else {
			i.Warnf("WS listener closed")
		}

		cf()
	}

	i.wgAccept.Wait()

	// Close active connections
	if i.listener != nil {
		i.Lock()
		i.Warnf("%d active connections, closing", len(i.conns))
		for _, c := range i.conns {
			if err := c.Close(); err != nil {
				i.Errorf("Unable to close connection from %d: %s", c.RemoteAddr(), err)
			} else {
				i.Infof("Connection from %s closed", c.RemoteAddr())
			}
		}
		i.Unlock()
	}

	i.Warnf("Waiting for connection handlers to close")
	i.wgConn.Wait()
	i.Warnf("Input closed")
}
