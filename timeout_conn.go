package main

import (
	"net"
	"time"
)

// TimeoutConn automatically applies a deadline on a conn upon every call
type TimeoutConn struct {
	net.Conn
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func newTimeoutConn(conn net.Conn, readTimeout time.Duration, writeTimeout time.Duration) *TimeoutConn {
	return &TimeoutConn{
		Conn:         conn,
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
	}
}

func (t *TimeoutConn) Write(p []byte) (n int, err error) {
	if t.writeTimeout > 0 {
		if err = t.Conn.SetWriteDeadline(time.Now().Add(t.writeTimeout)); err != nil {
			return
		}
	}

	return t.Conn.Write(p)
}

func (t *TimeoutConn) Read(p []byte) (n int, err error) {
	if t.readTimeout > 0 {
		if err = t.Conn.SetReadDeadline(time.Now().Add(t.readTimeout)); err != nil {
			return
		}
	}

	return t.Conn.Read(p)
}
