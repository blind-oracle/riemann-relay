package flatb

import (
	rpb "github.com/blind-oracle/riemann-relay/riemannpb"
	fb "github.com/google/flatbuffers/go"
)

const (
	maxBatchSize = 2048
	maxAttrs     = 64
)

// MetricFromRiemannEvents converts a slice of Riemann events to flatbuffer
func MetricFromRiemannEvents(b *fb.Builder, evs []*rpb.Event) {
	var (
		ia [maxAttrs]fb.UOffsetT
		im [maxBatchSize]fb.UOffsetT
	)

	mcnt := len(evs)
	if mcnt > maxBatchSize {
		panic("Max batch size reached")
	}

	for i, ev := range evs {
		acnt := len(ev.Attributes)
		if acnt > maxAttrs {
			panic("Max attr count reached")
		}

		for j, a := range ev.Attributes {
			key := b.CreateString(a.Key)
			value := b.CreateString(a.Value)

			AttrStart(b)
			AttrAddKey(b, key)
			AttrAddValue(b, value)
			ia[j] = AttrEnd(b)
		}

		MetricStartAttrsVector(b, acnt)
		for j := 0; j < acnt; j++ {
			b.PrependUOffsetT(ia[j])
		}
		attrs := b.EndVector(acnt)

		tcnt := len(ev.Tags)
		for j, t := range ev.Tags {
			tag := b.CreateString(t)
			ia[j] = tag
		}

		MetricStartTagsVector(b, tcnt)
		for j := 0; j < tcnt; j++ {
			b.PrependUOffsetT(ia[j])
		}
		tags := b.EndVector(tcnt)

		host := b.CreateString(ev.Host)
		svc := b.CreateString(ev.Service)
		state := b.CreateString(ev.State)
		descr := b.CreateString(ev.Description)

		MetricStart(b)
		MetricAddHost(b, host)
		MetricAddService(b, svc)
		MetricAddState(b, state)
		MetricAddDescription(b, descr)

		if ev.MetricD > 0 {
			MetricAddValue(b, ev.MetricD)
		} else if ev.MetricF > 0 {
			MetricAddValue(b, float64(ev.MetricD))
		} else {
			MetricAddValue(b, float64(ev.MetricSint64))
		}

		MetricAddTime(b, ev.Time)
		MetricAddAttrs(b, attrs)
		MetricAddTags(b, tags)

		im[i] = MetricEnd(b)
	}

	BatchStartMetricsVector(b, mcnt)
	for i := 0; i < mcnt; i++ {
		b.PrependUOffsetT(im[i])
	}
	metrics := b.EndVector(mcnt)

	BatchStart(b)
	BatchAddMetrics(b, metrics)
	batch := BatchEnd(b)
	b.Finish(batch)
}
