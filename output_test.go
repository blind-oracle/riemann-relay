package main

import (
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var testCfg = &outputCfg{
	Name:              "test",
	Type:              "riemann",
	Algo:              "hash",
	HashFields:        []string{"host"},
	CarbonValue:       "any",
	ConnectTimeout:    duration{1 * time.Second},
	ReconnectInterval: duration{1 * time.Second},
	Timeout:           duration{1 * time.Second},
	BufferSize:        50,
	BatchSize:         1,
	BatchTimeout:      duration{1 * time.Second},
}

func Test_output(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	ch := make(chan []*Event, 10)
	l, addr, err := getTestListener(ch)
	assert.Nil(t, err)

	testCfg.Targets = []string{addr}
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

	l.Close()
}
