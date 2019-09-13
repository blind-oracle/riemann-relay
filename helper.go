package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	math "math"
	"net"
	"strings"
	"sync"
	"unsafe"
)

type outputAlgo uint8
type outputType uint8
type riemannField uint8
type riemannFieldName struct {
	f    riemannField
	name string
}
type riemannValue uint8
type writeBatchFunc func([]*Event) error

func (a outputAlgo) String() string {
	return outputAlgoMapRev[a]
}

func (t outputType) String() string {
	return outputTypeMapRev[t]
}

func (f riemannField) String() string {
	return riemannFieldMapRev[f]
}

func (fn riemannFieldName) String() string {
	switch fn.f {
	case riemannFieldAttr, riemannFieldTag:
		return fmt.Sprintf("%s:%s", fn.f, fn.name)
	default:
		return fn.f.String()
	}
}

func (v riemannValue) String() string {
	return riemannValueMapRev[v]
}

const (
	outputTypeCarbon outputType = iota
	outputTypeRiemann
	outputTypeClickhouse
)

const (
	outputAlgoHash outputAlgo = iota
	outputAlgoFailover
	outputAlgoRoundRobin
	outputAlgoBroadcast
)

const (
	riemannFieldState riemannField = iota
	riemannFieldService
	riemannFieldHost
	riemannFieldDescription
	riemannFieldTag
	riemannFieldAttr
	riemannFieldCustom
	riemannFieldTimestamp
	riemannFieldValue
)

const (
	riemannValueInt riemannValue = iota
	riemannValueFloat
	riemannValueDouble
	riemannValueAny
)

var (
	outputTypeMap = map[string]outputType{
		"carbon":     outputTypeCarbon,
		"riemann":    outputTypeRiemann,
		"clickhouse": outputTypeClickhouse,
	}

	outputAlgoMap = map[string]outputAlgo{
		"hash":       outputAlgoHash,
		"failover":   outputAlgoFailover,
		"roundrobin": outputAlgoRoundRobin,
		"broadcast":  outputAlgoBroadcast,
	}

	riemannFieldMap = map[string]riemannField{
		"state":       riemannFieldState,
		"service":     riemannFieldService,
		"host":        riemannFieldHost,
		"description": riemannFieldDescription,
		"tag":         riemannFieldTag,
		"attr":        riemannFieldAttr,
		"custom":      riemannFieldCustom,
		"timestamp":   riemannFieldTimestamp,
		"value":       riemannFieldValue,
	}

	riemannValueMap = map[string]riemannValue{
		"int":    riemannValueInt,
		"float":  riemannValueFloat,
		"double": riemannValueDouble,
		"any":    riemannValueAny,
	}

	outputTypeMapRev   = map[outputType]string{}
	outputAlgoMapRev   = map[outputAlgo]string{}
	riemannFieldMapRev = map[riemannField]string{}
	riemannValueMapRev = map[riemannValue]string{}

	bufferPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 131072))
		},
	}

	bufferPoolSmall = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 128))
		},
	}
)

func init() {
	for k, v := range outputTypeMap {
		outputTypeMapRev[v] = k
	}

	for k, v := range outputAlgoMap {
		outputAlgoMapRev[v] = k
	}

	for k, v := range riemannFieldMap {
		riemannFieldMapRev[v] = k
	}

	for k, v := range riemannValueMap {
		riemannValueMapRev[v] = k
	}
}

func unsafeString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func parseRiemannFields(rfs []string, onlyStrings bool) (rfns []riemannFieldName, err error) {
	rhMap := map[string]bool{}
	for _, rh := range rfs {
		if rhMap[rh] {
			return nil, fmt.Errorf("Riemann field '%s' is specified more than once", rh)
		}

		rhMap[rh] = true
	}

	for _, f := range rfs {
		t := strings.SplitN(f, ":", 2)
		rf, ok := riemannFieldMap[t[0]]
		if !ok {
			return nil, fmt.Errorf("Unknown Riemann field '%s'", t[0])
		}

		fn := riemannFieldName{rf, ""}
		switch rf {
		case riemannFieldAttr, riemannFieldTag:
			if len(t) != 2 || t[1] == "" {
				return nil, fmt.Errorf("Field type '%s' requires a name after ':'", rf)
			}

			fn.name = t[1]
		case riemannFieldTimestamp, riemannFieldValue:
			if onlyStrings {
				return nil, fmt.Errorf("Timestamp and Value Riemann fields are not allowed here")
			}
		}

		rfns = append(rfns, fn)
	}

	return
}

func eventWriteField(w io.Writer, e *Event, f riemannFieldName, v riemannValue) (int, error) {
	switch f.f {
	case riemannFieldState:
		return w.Write([]byte(e.State))
	case riemannFieldService:
		return w.Write([]byte(e.Service))
	case riemannFieldHost:
		return w.Write([]byte(e.Host))
	case riemannFieldDescription:
		return w.Write([]byte(e.Description))
	case riemannFieldTag:
		if eventHasTag(e, f.name) {
			return w.Write([]byte(f.name))
		}
	case riemannFieldAttr:
		if attr := eventGetAttr(e, f.name); attr != nil {
			return w.Write([]byte(attr.Value))
		}
	case riemannFieldCustom:
		return w.Write([]byte(f.name))
	case riemannFieldTimestamp:
		var t uint32
		if e.TimeMicros > 0 {
			t = uint32(e.TimeMicros / 1000000)
		} else {
			t = uint32(e.Time)
		}

		return 4, binary.Write(w, binary.LittleEndian, t)
	case riemannFieldValue:
		t := math.Float64bits(eventGetValue(e, v))
		return 8, binary.Write(w, binary.LittleEndian, t)
	}

	return 0, nil
}

func eventFieldLen(e *Event, f riemannFieldName) int {
	switch f.f {
	case riemannFieldState:
		return len(e.State)
	case riemannFieldService:
		return len(e.Service)
	case riemannFieldHost:
		return len(e.Host)
	case riemannFieldDescription:
		return len(e.Description)
	case riemannFieldTag:
		if eventHasTag(e, f.name) {
			return len(f.name)
		}

		return 0
	case riemannFieldAttr:
		if attr := eventGetAttr(e, f.name); attr != nil {
			return len(attr.Value)
		}

		return 0
	case riemannFieldCustom:
		return len(f.name)
	}

	return 0
}

func eventWriteCompileFields(b *bytes.Buffer, e *Event, hf []riemannFieldName, sep byte) {
	for i, f := range hf {
		if i != 0 {
			b.WriteByte(sep)
		}

		eventWriteField(b, e, f, riemannValueAny)
	}
}

func eventWriteClickhouseBinary(w io.Writer, e *Event, hf []riemannFieldName, v riemannValue) (err error) {
	for _, f := range hf {
		switch f.f {
		default:
			b := make([]byte, 8)
			l := eventFieldLen(e, f)
			n := binary.PutUvarint(b, uint64(l))

			if _, err = w.Write(b[:n]); err != nil {
				return
			}

		case riemannFieldValue, riemannFieldTimestamp:
		}

		if _, err = eventWriteField(w, e, f, v); err != nil {
			return
		}
	}

	return
}

func eventGetValue(e *Event, v riemannValue) (o float64) {
	switch v {
	case riemannValueInt:
		o = float64(e.MetricSint64)
	case riemannValueFloat:
		o = float64(e.MetricF)
	case riemannValueDouble:
		o = e.MetricD
	case riemannValueAny:
		if e.MetricD > 0 {
			o = e.MetricD
		} else if e.MetricSint64 > 0 {
			o = float64(e.MetricSint64)
		} else {
			o = float64(e.MetricF)
		}
	}

	return
}

func eventGetAttr(e *Event, name string) *Attribute {
	for _, a := range e.Attributes {
		if a.Key == name {
			return a
		}
	}

	return nil
}

func eventHasTag(e *Event, name string) bool {
	for _, t := range e.Tags {
		if t == name {
			return true
		}
	}

	return false
}

// Ugly, but probably no better way to detect this
func isErrClosedConn(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection")
}

// Reads until 'p' is full
func readPacket(r io.Reader, p []byte) error {
	for len(p) > 0 {
		n, err := r.Read(p)
		p = p[n:]
		if err != nil {
			return err
		}
	}

	return nil
}

func eventGetTime(e *Event) int64 {
	if e.TimeMicros > 0 {
		return e.TimeMicros / 1000000
	}

	return e.Time
}

// func eventToCarbon(w io.Writer, e *Event, cf []riemannFieldName, cv riemannValue) []byte {

// 	b.Reset()
// 	bufferPool.Put(b)

// 	return b.Bytes()
// }

func guessProto(addr string) (proto string) {
	if _, err := net.ResolveTCPAddr("tcp", addr); err == nil {
		return "tcp"
	}

	return "unix"
}
