package main

import (
	"testing"

	pb "github.com/golang/protobuf/proto"
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
		Host:        pb.String("blah"),
		Service:     pb.String("foo"),
		Description: pb.String("baz"),
		State:       pb.String("ok"),
		Tags:        []string{"tag1", "tag2"},
		Attributes: []*Attribute{
			{
				Key:   pb.String("key1"),
				Value: pb.String("val1"),
			},
		},
		Ttl:        pb.Float32(0),
		TimeMicros: pb.Int64(1523367364787000),
		MetricD:    pb.Float64(123),
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
