package main

import (
	"bytes"
	"fmt"
	"io"
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
)

const (
	riemannValueInt riemannValue = iota
	riemannValueFloat
	riemannValueDouble
	riemannValueAny
)

var (
	outputTypeMap = map[string]outputType{
		"carbon":  outputTypeCarbon,
		"riemann": outputTypeRiemann,
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

func parseRiemannFields(rfs []string) (rfns []riemannFieldName, err error) {
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
			b.WriteString(sep + e.State)
		case riemannFieldService:
			b.WriteString(sep + e.Service)
		case riemannFieldHost:
			b.WriteString(sep + e.Host)
		case riemannFieldDescription:
			b.WriteString(sep + e.Description)
		case riemannFieldTag:
			if eventHasTag(e, f.name) {
				b.WriteString(sep + f.name)
			}
		case riemannFieldAttr:
			if attr = eventGetAttr(e, f.name); attr != nil {
				b.WriteString(sep + attr.Value)
			}
		}
	}

	// Skip first dot
	return b.Bytes()[1:]
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

func eventToCarbon(e *Event, cf []riemannFieldName, cv riemannValue) []byte {
	var b bytes.Buffer

	b.Write(eventCompileFields(e, cf, "."))
	b.WriteByte(' ')

	val := strconv.FormatFloat(eventGetValue(e, cv), 'f', -1, 64)
	b.WriteString(val)
	b.WriteByte(' ')

	var t int64
	if e.TimeMicros > 0 {
		t = e.TimeMicros / 1000000
	} else {
		t = e.Time
	}

	b.WriteString(strconv.FormatInt(t, 10))
	return b.Bytes()
}

func guessProto(addr string) (proto string) {
	if _, err := net.ResolveTCPAddr("tcp", addr); err == nil {
		return "tcp"
	}

	return "unix"
}
