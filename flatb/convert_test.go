package flatb

import (
	"testing"

	rpb "github.com/blind-oracle/riemann-relay/riemannpb"
	fb "github.com/google/flatbuffers/go"
)

var (
	testEvent = &rpb.Event{
		Host:        "foo",
		Service:     "bar",
		State:       "bad",
		Description: "wooo",
		Time:        1234,
		MetricD:     1.0,
		Attributes: []*rpb.Attribute{
			&rpb.Attribute{
				Key:   "key1",
				Value: "value1",
			},
		},

		Tags: []string{"tag1", "tag2"},
	}

	testBatch = []*rpb.Event{}
)

func init() {
	for i := 0; i < 500; i++ {
		testBatch = append(testBatch, testEvent)
	}
}

func Benchmark_convert(b *testing.B) {
	fbb := fb.NewBuilder(131072)

	for i := 0; i < b.N; i++ {
		MetricFromRiemannEvents(fbb, testBatch)
		fbb.Reset()
	}
}
