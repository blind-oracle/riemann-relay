package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	jsTest = `
	{
		"host": "blah",
		"service": "foo",
		"description": "baz",
		"state": "ok",
		"tags": ["tag1", "tag2"],
		"metric": 123,
		"time": "2018-04-10T13:36:04.787Z",
		"attributes": [
			{
				"key": "key1",
				"value": "val1"
			}
		]
	}
	`
)

func Test_eventFromJSON(t *testing.T) {
	ev := &Event{
		Host:        "blah",
		Service:     "foo",
		Description: "baz",
		State:       "ok",
		Tags:        []string{"tag1", "tag2"},
		Attributes: []*Attribute{
			{
				Key:   "key1",
				Value: "val1",
			},
		},
		TimeMicros: 1523367364787000,
		MetricD:    123,
	}

	ev2, err := eventFromJSON([]byte(jsTest))
	assert.Nil(t, err)
	assert.Equal(t, ev, ev2)
}

func Benchmark_eventFromJSON(b *testing.B) {
	for i := 0; i < b.N; i++ {
		eventFromJSON([]byte(jsTest))
	}
}
