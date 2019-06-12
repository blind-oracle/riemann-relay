// Code generated by protoc-gen-go. DO NOT EDIT.
// source: riemann.proto

package main

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type Event struct {
	Time                 *int64       `protobuf:"varint,1,opt,name=time" json:"time,omitempty"`
	State                *string      `protobuf:"bytes,2,opt,name=state" json:"state,omitempty"`
	Service              *string      `protobuf:"bytes,3,opt,name=service" json:"service,omitempty"`
	Host                 *string      `protobuf:"bytes,4,opt,name=host" json:"host,omitempty"`
	Description          *string      `protobuf:"bytes,5,opt,name=description" json:"description,omitempty"`
	Tags                 []string     `protobuf:"bytes,7,rep,name=tags" json:"tags,omitempty"`
	Ttl                  *float32     `protobuf:"fixed32,8,opt,name=ttl" json:"ttl,omitempty"`
	Attributes           []*Attribute `protobuf:"bytes,9,rep,name=attributes" json:"attributes,omitempty"`
	TimeMicros           *int64       `protobuf:"varint,10,opt,name=time_micros,json=timeMicros" json:"time_micros,omitempty"`
	MetricSint64         *int64       `protobuf:"zigzag64,13,opt,name=metric_sint64,json=metricSint64" json:"metric_sint64,omitempty"`
	MetricD              *float64     `protobuf:"fixed64,14,opt,name=metric_d,json=metricD" json:"metric_d,omitempty"`
	MetricF              *float32     `protobuf:"fixed32,15,opt,name=metric_f,json=metricF" json:"metric_f,omitempty"`
	XXX_NoUnkeyedLiteral struct{}     `json:"-"`
	XXX_unrecognized     []byte       `json:"-"`
	XXX_sizecache        int32        `json:"-"`
}

func (m *Event) Reset()         { *m = Event{} }
func (m *Event) String() string { return proto.CompactTextString(m) }
func (*Event) ProtoMessage()    {}
func (*Event) Descriptor() ([]byte, []int) {
	return fileDescriptor_0e43e81d710c3d26, []int{0}
}

func (m *Event) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Event.Unmarshal(m, b)
}
func (m *Event) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Event.Marshal(b, m, deterministic)
}
func (m *Event) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Event.Merge(m, src)
}
func (m *Event) XXX_Size() int {
	return xxx_messageInfo_Event.Size(m)
}
func (m *Event) XXX_DiscardUnknown() {
	xxx_messageInfo_Event.DiscardUnknown(m)
}

var xxx_messageInfo_Event proto.InternalMessageInfo

func (m *Event) GetTime() int64 {
	if m != nil && m.Time != nil {
		return *m.Time
	}
	return 0
}

func (m *Event) GetState() string {
	if m != nil && m.State != nil {
		return *m.State
	}
	return ""
}

func (m *Event) GetService() string {
	if m != nil && m.Service != nil {
		return *m.Service
	}
	return ""
}

func (m *Event) GetHost() string {
	if m != nil && m.Host != nil {
		return *m.Host
	}
	return ""
}

func (m *Event) GetDescription() string {
	if m != nil && m.Description != nil {
		return *m.Description
	}
	return ""
}

func (m *Event) GetTags() []string {
	if m != nil {
		return m.Tags
	}
	return nil
}

func (m *Event) GetTtl() float32 {
	if m != nil && m.Ttl != nil {
		return *m.Ttl
	}
	return 0
}

func (m *Event) GetAttributes() []*Attribute {
	if m != nil {
		return m.Attributes
	}
	return nil
}

func (m *Event) GetTimeMicros() int64 {
	if m != nil && m.TimeMicros != nil {
		return *m.TimeMicros
	}
	return 0
}

func (m *Event) GetMetricSint64() int64 {
	if m != nil && m.MetricSint64 != nil {
		return *m.MetricSint64
	}
	return 0
}

func (m *Event) GetMetricD() float64 {
	if m != nil && m.MetricD != nil {
		return *m.MetricD
	}
	return 0
}

func (m *Event) GetMetricF() float32 {
	if m != nil && m.MetricF != nil {
		return *m.MetricF
	}
	return 0
}

type Msg struct {
	Ok                   *bool    `protobuf:"varint,2,opt,name=ok" json:"ok,omitempty"`
	Error                *string  `protobuf:"bytes,3,opt,name=error" json:"error,omitempty"`
	Events               []*Event `protobuf:"bytes,6,rep,name=events" json:"events,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Msg) Reset()         { *m = Msg{} }
func (m *Msg) String() string { return proto.CompactTextString(m) }
func (*Msg) ProtoMessage()    {}
func (*Msg) Descriptor() ([]byte, []int) {
	return fileDescriptor_0e43e81d710c3d26, []int{1}
}

func (m *Msg) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Msg.Unmarshal(m, b)
}
func (m *Msg) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Msg.Marshal(b, m, deterministic)
}
func (m *Msg) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Msg.Merge(m, src)
}
func (m *Msg) XXX_Size() int {
	return xxx_messageInfo_Msg.Size(m)
}
func (m *Msg) XXX_DiscardUnknown() {
	xxx_messageInfo_Msg.DiscardUnknown(m)
}

var xxx_messageInfo_Msg proto.InternalMessageInfo

func (m *Msg) GetOk() bool {
	if m != nil && m.Ok != nil {
		return *m.Ok
	}
	return false
}

func (m *Msg) GetError() string {
	if m != nil && m.Error != nil {
		return *m.Error
	}
	return ""
}

func (m *Msg) GetEvents() []*Event {
	if m != nil {
		return m.Events
	}
	return nil
}

type Attribute struct {
	Key                  *string  `protobuf:"bytes,1,req,name=key" json:"key,omitempty"`
	Value                *string  `protobuf:"bytes,2,opt,name=value" json:"value,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Attribute) Reset()         { *m = Attribute{} }
func (m *Attribute) String() string { return proto.CompactTextString(m) }
func (*Attribute) ProtoMessage()    {}
func (*Attribute) Descriptor() ([]byte, []int) {
	return fileDescriptor_0e43e81d710c3d26, []int{2}
}

func (m *Attribute) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Attribute.Unmarshal(m, b)
}
func (m *Attribute) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Attribute.Marshal(b, m, deterministic)
}
func (m *Attribute) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Attribute.Merge(m, src)
}
func (m *Attribute) XXX_Size() int {
	return xxx_messageInfo_Attribute.Size(m)
}
func (m *Attribute) XXX_DiscardUnknown() {
	xxx_messageInfo_Attribute.DiscardUnknown(m)
}

var xxx_messageInfo_Attribute proto.InternalMessageInfo

func (m *Attribute) GetKey() string {
	if m != nil && m.Key != nil {
		return *m.Key
	}
	return ""
}

func (m *Attribute) GetValue() string {
	if m != nil && m.Value != nil {
		return *m.Value
	}
	return ""
}

func init() {
	proto.RegisterType((*Event)(nil), "main.Event")
	proto.RegisterType((*Msg)(nil), "main.Msg")
	proto.RegisterType((*Attribute)(nil), "main.Attribute")
}

func init() { proto.RegisterFile("riemann.proto", fileDescriptor_0e43e81d710c3d26) }

var fileDescriptor_0e43e81d710c3d26 = []byte{
	// 326 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x4c, 0x91, 0xcd, 0x4e, 0xc3, 0x30,
	0x10, 0x84, 0xe5, 0xa4, 0x7f, 0xd9, 0xd0, 0x16, 0x59, 0x1c, 0x96, 0x13, 0x56, 0x7b, 0xc9, 0xa9,
	0x48, 0x80, 0xb8, 0x23, 0x01, 0xb7, 0x4a, 0xc8, 0x3c, 0x40, 0x15, 0x52, 0x53, 0xac, 0x36, 0x71,
	0x65, 0x6f, 0x2b, 0xf1, 0x9e, 0x3c, 0x10, 0xf2, 0x26, 0xad, 0x7a, 0x9b, 0xf9, 0xd6, 0x71, 0x3c,
	0xb3, 0x30, 0xf6, 0xd6, 0xd4, 0x65, 0xd3, 0x2c, 0xf6, 0xde, 0x91, 0x93, 0xbd, 0xba, 0xb4, 0xcd,
	0xec, 0x2f, 0x81, 0xfe, 0xdb, 0xd1, 0x34, 0x24, 0x25, 0xf4, 0xc8, 0xd6, 0x06, 0x85, 0x12, 0x45,
	0xaa, 0x59, 0xcb, 0x1b, 0xe8, 0x07, 0x2a, 0xc9, 0x60, 0xa2, 0x44, 0x91, 0xe9, 0xd6, 0x48, 0x84,
	0x61, 0x30, 0xfe, 0x68, 0x2b, 0x83, 0x29, 0xf3, 0x93, 0x8d, 0x77, 0xfc, 0xb8, 0x40, 0xd8, 0x63,
	0xcc, 0x5a, 0x2a, 0xc8, 0xd7, 0x26, 0x54, 0xde, 0xee, 0xc9, 0xba, 0x06, 0xfb, 0x3c, 0xba, 0x44,
	0xfc, 0xe7, 0x72, 0x13, 0x70, 0xa8, 0xd2, 0xf8, 0x55, 0xd4, 0xf2, 0x1a, 0x52, 0xa2, 0x1d, 0x8e,
	0x94, 0x28, 0x12, 0x1d, 0xa5, 0xbc, 0x07, 0x28, 0x89, 0xbc, 0xfd, 0x3a, 0x90, 0x09, 0x98, 0xa9,
	0xb4, 0xc8, 0x1f, 0xa6, 0x8b, 0x18, 0x62, 0xf1, 0x72, 0xe2, 0xfa, 0xe2, 0x88, 0xbc, 0x83, 0x3c,
	0x86, 0x58, 0xd5, 0xb6, 0xf2, 0x2e, 0x20, 0x70, 0x2e, 0x88, 0x68, 0xc9, 0x44, 0xce, 0x61, 0x5c,
	0x1b, 0xf2, 0xb6, 0x5a, 0x05, 0xdb, 0xd0, 0xf3, 0x13, 0x8e, 0x95, 0x28, 0xa4, 0xbe, 0x6a, 0xe1,
	0x27, 0x33, 0x79, 0x0b, 0xa3, 0xee, 0xd0, 0x1a, 0x27, 0x4a, 0x14, 0x42, 0x0f, 0x5b, 0xff, 0x7a,
	0x31, 0xfa, 0xc6, 0x29, 0x3f, 0xb4, 0x1b, 0xbd, 0xcf, 0x3e, 0x20, 0x5d, 0x86, 0x8d, 0x9c, 0x40,
	0xe2, 0xb6, 0x5c, 0xde, 0x48, 0x27, 0x6e, 0x1b, 0xfb, 0x34, 0xde, 0x3b, 0xdf, 0xf5, 0xd6, 0x1a,
	0x39, 0x87, 0x81, 0x89, 0x2b, 0x08, 0x38, 0xe0, 0x54, 0x79, 0x9b, 0x8a, 0xd7, 0xa2, 0xbb, 0xd1,
	0xec, 0x11, 0xb2, 0x73, 0xcc, 0xd8, 0xce, 0xd6, 0xfc, 0xa2, 0x50, 0x49, 0x91, 0xe9, 0x28, 0xe3,
	0xcd, 0xc7, 0x72, 0x77, 0x38, 0x6f, 0x8a, 0xcd, 0x7f, 0x00, 0x00, 0x00, 0xff, 0xff, 0x86, 0xc2,
	0xad, 0x50, 0xf3, 0x01, 0x00, 0x00,
}
