package main

import (
	fmt "fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	riemanngo "github.com/riemann/riemann-go-client"
	"github.com/stretchr/testify/assert"
)

func sendTestEvent(addr string) (err error) {
	c := riemanngo.NewTCPClient(addr, 1*time.Second)
	if err = c.Connect(); err != nil {
		return
	}

	r, err := riemanngo.SendEvents(c, &[]riemanngo.Event{{
		Service:     "foo",
		Host:        "bar",
		Description: "baz",
	}})

	if err != nil {
		return fmt.Errorf("Unable to send event: %s", err)
	}

	if r.GetOk() != true {
		return fmt.Errorf("Not Ok response")
	}

	c.Close()
	return
}

func getTestListener(ch chan []*Event) (l *listener, err error) {
	for port := 0; port < 50; port++ {
		cfg.Listen = "127.0.0.1:" + strconv.Itoa(rand.Intn(20000)+20000)
		if l, err = newListener(ch); err == nil {
			return
		}
	}

	return
}

func Test_listener(t *testing.T) {
	var (
		ch = make(chan []*Event, 10)
	)

	l, err := getTestListener(ch)
	assert.Nil(t, err)

	err = sendTestEvent(cfg.Listen)
	assert.Nil(t, err)

	assert.Equal(t, "receivedBatches 1 receivedEvents 1 dropped 0", l.getStats())
	l.Close()

	evT := &Event{
		Service:     "foo",
		Host:        "bar",
		Description: "baz",
	}

	batch, ok := <-ch
	assert.True(t, ok)
	assert.Equal(t, 1, len(batch))

	ev := batch[0]
	assert.Equal(t, evT.Service, ev.Service)
	assert.Equal(t, evT.Host, ev.Host)
	assert.Equal(t, evT.Description, ev.Description)
}
