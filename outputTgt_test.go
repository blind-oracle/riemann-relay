package main

import (
	"bufio"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testCfg = &outputCfg{
	Name:              "test",
	Type:              "riemann",
	Algo:              "hash",
	HashFields:        []string{"host"},
	CarbonFields:      []string{"service", "host", "description"},
	CarbonValue:       "any",
	ConnectTimeout:    duration{1 * time.Second},
	ReconnectInterval: duration{1 * time.Second},
	Timeout:           duration{1 * time.Second},
	BufferSize:        50,
	BatchSize:         1,
	BatchTimeout:      duration{1 * time.Second},
}

func Test_outputRiemann(t *testing.T) {
	cfg.StatsInterval.Duration = 50 * time.Millisecond

	ch := make(chan []*Event, 10)
	i, cf, err := getTestInput()
	assert.Nil(t, err)
	i.addChannel("test", ch)

	testCfg.Targets = []string{cf.Listen}
	test := func(algo string) {
		testCfg.Algo = algo
		o, err := newOutput(testCfg)
		assert.Nil(t, err)

		evT := &Event{
			Service:     "foo",
			Host:        "bar",
			Description: "baz",
		}

		time.Sleep(100 * time.Millisecond)
		o.chanIn <- []*Event{evT}

		batch, ok := <-ch
		assert.True(t, ok)
		assert.Equal(t, 1, len(batch))

		ev := batch[0]
		assert.Equal(t, evT.Service, ev.Service)
		assert.Equal(t, evT.Host, ev.Host)
		assert.Equal(t, evT.Description, ev.Description)

		o.Close()
	}

	for _, a := range outputAlgoMapRev {
		test(a)
	}

	i.Close()
}

func Test_outputCarbon(t *testing.T) {
	var (
		addr string
		err  error
		l    net.Listener
	)

	ch := make(chan string, 10)

	for i := 0; i < 50; i++ {
		addr = "127.0.0.1:" + strconv.Itoa(20000+rand.Intn(20000))
		if l, err = listen(addr); err == nil {
			break
		}
	}
	assert.Nil(t, err)

	accept := func() {
		c, err := l.Accept()
		assert.Nil(t, err)

		bio := bufio.NewReader(c)
		row, err := bio.ReadString('\n')
		assert.Nil(t, err)

		ch <- strings.TrimSpace(row)
	}

	go accept()

	testCfg.Type = "carbon"
	testCfg.Algo = "hash"
	testCfg.Targets = []string{addr}
	o, err := newOutput(testCfg)
	assert.Nil(t, err)

	evT := &Event{
		Service:      "foo",
		Host:         "bar",
		Description:  "baz",
		MetricSint64: 123,
		Time:         12345,
	}

	time.Sleep(100 * time.Millisecond)
	o.chanIn <- []*Event{evT}

	row := <-ch
	assert.Equal(t, "foo.bar.baz 123 12345", row)

	time.Sleep(100 * time.Millisecond)
	o.Close()
	l.Close()
}
