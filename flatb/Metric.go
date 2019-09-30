// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package flatb

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type Metric struct {
	_tab flatbuffers.Table
}

func GetRootAsMetric(buf []byte, offset flatbuffers.UOffsetT) *Metric {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &Metric{}
	x.Init(buf, n+offset)
	return x
}

func (rcv *Metric) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *Metric) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *Metric) Host() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *Metric) Service() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *Metric) State() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *Metric) Description() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(10))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *Metric) Time() int64 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(12))
	if o != 0 {
		return rcv._tab.GetInt64(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *Metric) MutateTime(n int64) bool {
	return rcv._tab.MutateInt64Slot(12, n)
}

func (rcv *Metric) Value() float64 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(14))
	if o != 0 {
		return rcv._tab.GetFloat64(o + rcv._tab.Pos)
	}
	return 0.0
}

func (rcv *Metric) MutateValue(n float64) bool {
	return rcv._tab.MutateFloat64Slot(14, n)
}

func (rcv *Metric) Tags(j int) []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(16))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.ByteVector(a + flatbuffers.UOffsetT(j*4))
	}
	return nil
}

func (rcv *Metric) TagsLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(16))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *Metric) Attrs(obj *Attr, j int) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(18))
	if o != 0 {
		x := rcv._tab.Vector(o)
		x += flatbuffers.UOffsetT(j) * 4
		x = rcv._tab.Indirect(x)
		obj.Init(rcv._tab.Bytes, x)
		return true
	}
	return false
}

func (rcv *Metric) AttrsLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(18))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func MetricStart(builder *flatbuffers.Builder) {
	builder.StartObject(8)
}
func MetricAddHost(builder *flatbuffers.Builder, host flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(0, flatbuffers.UOffsetT(host), 0)
}
func MetricAddService(builder *flatbuffers.Builder, service flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(1, flatbuffers.UOffsetT(service), 0)
}
func MetricAddState(builder *flatbuffers.Builder, state flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(2, flatbuffers.UOffsetT(state), 0)
}
func MetricAddDescription(builder *flatbuffers.Builder, description flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(3, flatbuffers.UOffsetT(description), 0)
}
func MetricAddTime(builder *flatbuffers.Builder, time int64) {
	builder.PrependInt64Slot(4, time, 0)
}
func MetricAddValue(builder *flatbuffers.Builder, value float64) {
	builder.PrependFloat64Slot(5, value, 0.0)
}
func MetricAddTags(builder *flatbuffers.Builder, tags flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(6, flatbuffers.UOffsetT(tags), 0)
}
func MetricStartTagsVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 4)
}
func MetricAddAttrs(builder *flatbuffers.Builder, attrs flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(7, flatbuffers.UOffsetT(attrs), 0)
}
func MetricStartAttrsVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 4)
}
func MetricEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}