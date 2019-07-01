package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pb "github.com/golang/protobuf/proto"
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

var (
	eventJSONFields = map[string]bool{
		"host":        true,
		"service":     true,
		"state":       true,
		"description": true,
		"time":        true,
		"metric":      true,
		"tags":        true,
		"attributes":  true,
		"ttl":         true,
	}
)

func eventFromJSON(msg []byte) (ev *Event, err error) {
	evJS := &eventJSON{}
	if err = json.Unmarshal(msg, evJS); err != nil {
		return
	}

	ev = &Event{
		Host:        pb.String(evJS.Host),
		Service:     pb.String(evJS.Service),
		State:       pb.String(evJS.State),
		Description: pb.String(evJS.Description),
		MetricD:     pb.Float64(evJS.Metric),
		Tags:        evJS.Tags,
		Ttl:         pb.Float32(evJS.TTL),
	}

	var tm time.Time
	if !evJS.Time.IsZero() {
		tm = evJS.Time
	} else {
		tm = time.Now()
	}

	ev.TimeMicros = pb.Int64(tm.UnixNano() / 1000)

	// Unmarshal again to a map
	m := map[string]interface{}{}
	if err = json.Unmarshal(msg, &m); err != nil {
		return
	}

	// Put any non-standard fields into attributes
	for k, v := range m {
		klc := strings.ToLower(k)

		if !eventJSONFields[klc] {
			ev.Attributes = append(ev.Attributes, &Attribute{
				Key:   pb.String(klc),
				Value: pb.String(fmt.Sprintf("%v", v)),
			})
		}
	}

	for _, attr := range evJS.Attributes {
		ev.Attributes = append(ev.Attributes, &Attribute{
			Key:   pb.String(attr.Key),
			Value: pb.String(attr.Value),
		})
	}

	return
}

func eventsFromMultipleJSONs(msg []byte) (evs []*Event, err error) {
	for _, p := range bytes.Split(msg, []byte("\n")) {
		p = bytes.TrimSpace(p)
		if len(p) == 0 {
			continue
		}

		ev, err := eventFromJSON(p)
		if err != nil {
			return nil, err
		}

		evs = append(evs, ev)
	}

	return
}
