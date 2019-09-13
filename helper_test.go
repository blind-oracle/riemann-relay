package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testEvent = &Event{
		State:       "foo",
		Service:     "bar",
		Host:        "baz",
		Description: "fooz",
		Tags:        []string{"a", "b", "c"},
		Attributes: []*Attribute{
			{Key: "key1", Value: "val1"},
			{Key: "key2", Value: "val2"},
		},
		Time:         1234567,
		TimeMicros:   1234567000000,
		MetricSint64: 9876,
	}

	testRfn = []riemannFieldName{
		{riemannFieldState, ""},
		{riemannFieldService, ""},
		{riemannFieldHost, ""},
		{riemannFieldDescription, ""},
		{riemannFieldTag, "b"},
		{riemannFieldTag, "z"},
		{riemannFieldAttr, "key1"},
		{riemannFieldAttr, "key3"},
	}
)

func Test_parseRiemannFields(t *testing.T) {
	f := []string{
		"state",
		"service",
		"host",
		"description",
		"tag:b",
		"tag:z",
		"attr:key1",
		"attr:key3",
	}

	r, err := parseRiemannFields(f, true)
	assert.Nil(t, err)
	assert.Equal(t, testRfn, r)

	f[0] = "abcd"
	_, err = parseRiemannFields(f, true)
	assert.NotNil(t, err)

	f[0] = "attr"
	_, err = parseRiemannFields(f, true)
	assert.NotNil(t, err)

	f[0] = "tag"
	_, err = parseRiemannFields(f, true)
	assert.NotNil(t, err)

	f[0] = "service"
	_, err = parseRiemannFields(f, true)
	assert.NotNil(t, err)
}

// func Test_eventCompileFields(t *testing.T) {
// 	fields := eventCompileFields(testEvent, testRfn, ".")

// 	assert.Equal(t,
// 		fields,
// 		[]byte("foo.bar.baz.fooz.b.val1"),
// 	)
// }

// func Test_eventToCarbon(t *testing.T) {
// 	rf, _ := parseRiemannFields([]string{"state",
// 		"service",
// 		"host",
// 		"description",
// 	}, true)

// 	c := string(eventToCarbon(testEvent, rf, riemannValueAny))
// 	assert.Equal(t, "foo.bar.baz.fooz 9876 1234567", c)
// }

// func Benchmark_eventToCarbon(b *testing.B) {
// 	rf, _ := parseRiemannFields([]string{"state",
// 		"service",
// 		"host",
// 		"description",
// 	}, true)

// 	for i := 0; i < b.N; i++ {
// 		eventToCarbon(testEvent, rf, riemannValueAny)
// 	}
// }

// func Benchmark_eventCompileFields(b *testing.B) {
// 	for i := 0; i < b.N; i++ {
// 		eventCompileFields(testEvent, testRfn, ".")
// 	}
// }

func Benchmark_Test(b *testing.B) {
	a := "Asdfgasgf"
	for i := 0; i < b.N; i++ {
		b := []byte(a)
		_ = b
	}
}

func Test_eventGetAttr(t *testing.T) {
	assert.Equal(t, "val1", eventGetAttr(testEvent, "key1").GetValue())
	assert.Nil(t, eventGetAttr(testEvent, "foo"))
}

func Test_eventHasTag(t *testing.T) {
	assert.Equal(t, true, eventHasTag(testEvent, "a"))
	assert.Equal(t, false, eventHasTag(testEvent, "z"))
}

func Test_readPacket(t *testing.T) {
	c := []byte("abcdefghijk")
	b := bytes.NewReader(c)
	b1 := make([]byte, len(c))
	err := readPacket(b, b1)
	assert.Nil(t, err)
	assert.Equal(t, c, b1)
}

func Test_guessProto(t *testing.T) {
	assert.Equal(t, "tcp", guessProto("1.1.1.1:1"))
	assert.Equal(t, "unix", guessProto("/tmp/socket"))
}
