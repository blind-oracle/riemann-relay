package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	math "math"
	"net"
	"strconv"
	"strings"
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

func eventCompileFields(e *Event, hf []riemannFieldName, sep string) []byte {
	var (
		b    bytes.Buffer
		attr *Attribute
	)

	for _, f := range hf {
		switch f.f {
		case riemannFieldState:
			b.WriteString(sep + e.GetState())
		case riemannFieldService:
			b.WriteString(sep + e.GetService())
		case riemannFieldHost:
			b.WriteString(sep + e.GetHost())
		case riemannFieldDescription:
			b.WriteString(sep + e.GetDescription())
		case riemannFieldTag:
			if eventHasTag(e, f.name) {
				b.WriteString(sep + f.name)
			}
		case riemannFieldAttr:
			if attr = eventGetAttr(e, f.name); attr != nil {
				b.WriteString(sep + attr.GetValue())
			}
		case riemannFieldCustom:
			b.WriteString(sep + f.name)
		}
	}

	if b.Len() == 0 {
		return []byte{}
	}

	// Skip first dot
	return b.Bytes()[1:]
}

func eventWriteClickhouseBinary(e *Event, hf []riemannFieldName, v riemannValue, w io.Writer) (err error) {
	var attr *Attribute

	for _, f := range hf {
		var (
			s string
			i interface{}
		)

		switch f.f {
		case riemannFieldState:
			s = e.GetState()
		case riemannFieldService:
			s = e.GetService()
		case riemannFieldHost:
			s = e.GetHost()
		case riemannFieldDescription:
			s = e.GetDescription()
		case riemannFieldTag:
			if eventHasTag(e, f.name) {
				s = f.name
			}
		case riemannFieldAttr:
			if attr = eventGetAttr(e, f.name); attr != nil {
				s = attr.GetValue()
			}
		case riemannFieldCustom:
			s = f.name
		case riemannFieldTimestamp:
			i = uint32(eventGetTime(e))
		case riemannFieldValue:
			i = math.Float64bits(eventGetValue(e, v))
		}

		if s != "" {
			b := make([]byte, 8)
			n := binary.PutUvarint(b, uint64(len(s)))

			if _, err = w.Write(b[:n]); err != nil {
				return
			}

			if _, err = w.Write([]byte(s)); err != nil {
				return
			}
		} else if i != nil {
			if err = binary.Write(w, binary.LittleEndian, i); err != nil {
				return
			}
		}
	}

	return
}

func eventGetValue(e *Event, v riemannValue) (o float64) {
	switch v {
	case riemannValueInt:
		o = float64(e.GetMetricSint64())
	case riemannValueFloat:
		o = float64(e.GetMetricF())
	case riemannValueDouble:
		o = e.GetMetricD()
	case riemannValueAny:
		if e.GetMetricD() > 0 {
			o = e.GetMetricD()
		} else if e.GetMetricSint64() > 0 {
			o = float64(e.GetMetricSint64())
		} else {
			o = float64(e.GetMetricF())
		}
	}

	return
}

func eventGetAttr(e *Event, name string) *Attribute {
	for _, a := range e.Attributes {
		if a.GetKey() == name {
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
	if e.GetTimeMicros() > 0 {
		return e.GetTimeMicros() / 1000000
	}

	return e.GetTime()
}

func eventToCarbon(e *Event, cf []riemannFieldName, cv riemannValue) []byte {
	var b bytes.Buffer

	b.Write(eventCompileFields(e, cf, "."))
	b.WriteByte(' ')

	val := strconv.FormatFloat(eventGetValue(e, cv), 'f', -1, 64)
	b.WriteString(val)
	b.WriteByte(' ')
	b.WriteString(strconv.FormatInt(eventGetTime(e), 10))
	return b.Bytes()
}

func guessProto(addr string) (proto string) {
	if _, err := net.ResolveTCPAddr("tcp", addr); err == nil {
		return "tcp"
	}

	return "unix"
}
