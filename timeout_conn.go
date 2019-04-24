package main

import (
	"net"
	"time"
)

// TimeoutConn automatically applies a deadline on a conn upon every call
type TimeoutConn struct {
	*net.TCPConn
	timeout time.Duration
}

func newTimeoutConn(conn *net.TCPConn, timeout time.Duration) *TimeoutConn {
	return &TimeoutConn{
		TCPConn: conn,
		timeout: timeout,
	}
}

func (t *TimeoutConn) setDL() error {
	if t.timeout == 0 {
		return nil
	}

	if err := t.TCPConn.SetDeadline(time.Now().Add(t.timeout)); err != nil {
		return err
	}

	return nil
}

func (t *TimeoutConn) Write(p []byte) (n int, err error) {
	if err = t.setDL(); err != nil {
		return 0, err
	}

	return t.TCPConn.Write(p)
}

func (t *TimeoutConn) Read(p []byte) (n int, err error) {
	if err = t.setDL(); err != nil {
		return 0, err
	}

	return t.TCPConn.Read(p)
}
