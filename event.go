package main

import (
	"encoding/json"
	"time"
)

type attributeJSON struct {
	Key   string
	Value string
}

type eventJSON struct {
	Host        string
	Service     string
	State       string
	Description string
	Time        time.Time
	Metric      float64
	Tags        []string
	Attributes  []attributeJSON
	TTL         float32
}

func eventFromJSON(msg []byte) (ev *Event, err error) {
	evJS := &eventJSON{}
	if err = json.Unmarshal(msg, evJS); err != nil {
		return
	}

	ev = &Event{
		Host:        evJS.Host,
		Service:     evJS.Service,
		State:       evJS.State,
		Description: evJS.Description,
		MetricD:     evJS.Metric,
		Tags:        evJS.Tags,
		Ttl:         evJS.TTL,
	}

	if !evJS.Time.IsZero() {
		ev.TimeMicros = evJS.Time.UnixNano() / 1000
	} else {
		ev.TimeMicros = time.Now().UnixNano() / 1000
	}

	for _, attr := range evJS.Attributes {
		ev.Attributes = append(ev.Attributes, &Attribute{
			Key:   attr.Key,
			Value: attr.Value,
		})
	}

	return
}
