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

	r, err := parseRiemannFields(f)
	assert.Nil(t, err)
	assert.Equal(t, testRfn, r)

	f[0] = "abcd"
	r, err = parseRiemannFields(f)
	assert.NotNil(t, err)

	f[0] = "attr"
	r, err = parseRiemannFields(f)
	assert.NotNil(t, err)

	f[0] = "tag"
	r, err = parseRiemannFields(f)
	assert.NotNil(t, err)

	f[0] = "service"
	r, err = parseRiemannFields(f)
	assert.NotNil(t, err)
}

func Test_eventCompileFields(t *testing.T) {
	fields := eventCompileFields(testEvent, testRfn, ".")

	assert.Equal(t,
		fields,
		[]byte("foo.bar.baz.fooz.b.val1"),
	)
}

func Test_eventGetAttr(t *testing.T) {
	assert.Equal(t, "val1", eventGetAttr(testEvent, "key1").Value)
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
