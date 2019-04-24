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

type State struct {
	Time                 int64    `protobuf:"varint,1,opt,name=time,proto3" json:"time,omitempty"`
	State                string   `protobuf:"bytes,2,opt,name=state,proto3" json:"state,omitempty"`
	Service              string   `protobuf:"bytes,3,opt,name=service,proto3" json:"service,omitempty"`
	Host                 string   `protobuf:"bytes,4,opt,name=host,proto3" json:"host,omitempty"`
	Description          string   `protobuf:"bytes,5,opt,name=description,proto3" json:"description,omitempty"`
	Once                 bool     `protobuf:"varint,6,opt,name=once,proto3" json:"once,omitempty"`
	Tags                 []string `protobuf:"bytes,7,rep,name=tags,proto3" json:"tags,omitempty"`
	Ttl                  float32  `protobuf:"fixed32,8,opt,name=ttl,proto3" json:"ttl,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *State) Reset()         { *m = State{} }
func (m *State) String() string { return proto.CompactTextString(m) }
func (*State) ProtoMessage()    {}
func (*State) Descriptor() ([]byte, []int) {
	return fileDescriptor_0e43e81d710c3d26, []int{0}
}

func (m *State) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_State.Unmarshal(m, b)
}
func (m *State) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_State.Marshal(b, m, deterministic)
}
func (m *State) XXX_Merge(src proto.Message) {
	xxx_messageInfo_State.Merge(m, src)
}
func (m *State) XXX_Size() int {
	return xxx_messageInfo_State.Size(m)
}
func (m *State) XXX_DiscardUnknown() {
	xxx_messageInfo_State.DiscardUnknown(m)
}

var xxx_messageInfo_State proto.InternalMessageInfo

func (m *State) GetTime() int64 {
	if m != nil {
		return m.Time
	}
	return 0
}

func (m *State) GetState() string {
	if m != nil {
		return m.State
	}
	return ""
}

func (m *State) GetService() string {
	if m != nil {
		return m.Service
	}
	return ""
}

func (m *State) GetHost() string {
	if m != nil {
		return m.Host
	}
	return ""
}

func (m *State) GetDescription() string {
	if m != nil {
		return m.Description
	}
	return ""
}

func (m *State) GetOnce() bool {
	if m != nil {
		return m.Once
	}
	return false
}

func (m *State) GetTags() []string {
	if m != nil {
		return m.Tags
	}
	return nil
}

func (m *State) GetTtl() float32 {
	if m != nil {
		return m.Ttl
	}
	return 0
}

type Event struct {
	Time                 int64        `protobuf:"varint,1,opt,name=time,proto3" json:"time,omitempty"`
	State                string       `protobuf:"bytes,2,opt,name=state,proto3" json:"state,omitempty"`
	Service              string       `protobuf:"bytes,3,opt,name=service,proto3" json:"service,omitempty"`
	Host                 string       `protobuf:"bytes,4,opt,name=host,proto3" json:"host,omitempty"`
	Description          string       `protobuf:"bytes,5,opt,name=description,proto3" json:"description,omitempty"`
	Tags                 []string     `protobuf:"bytes,7,rep,name=tags,proto3" json:"tags,omitempty"`
	Ttl                  float32      `protobuf:"fixed32,8,opt,name=ttl,proto3" json:"ttl,omitempty"`
	Attributes           []*Attribute `protobuf:"bytes,9,rep,name=attributes,proto3" json:"attributes,omitempty"`
	TimeMicros           int64        `protobuf:"varint,10,opt,name=time_micros,json=timeMicros,proto3" json:"time_micros,omitempty"`
	MetricSint64         int64        `protobuf:"zigzag64,13,opt,name=metric_sint64,json=metricSint64,proto3" json:"metric_sint64,omitempty"`
	MetricD              float64      `protobuf:"fixed64,14,opt,name=metric_d,json=metricD,proto3" json:"metric_d,omitempty"`
	MetricF              float32      `protobuf:"fixed32,15,opt,name=metric_f,json=metricF,proto3" json:"metric_f,omitempty"`
	XXX_NoUnkeyedLiteral struct{}     `json:"-"`
	XXX_unrecognized     []byte       `json:"-"`
	XXX_sizecache        int32        `json:"-"`
}

func (m *Event) Reset()         { *m = Event{} }
func (m *Event) String() string { return proto.CompactTextString(m) }
func (*Event) ProtoMessage()    {}
func (*Event) Descriptor() ([]byte, []int) {
	return fileDescriptor_0e43e81d710c3d26, []int{1}
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
	if m != nil {
		return m.Time
	}
	return 0
}

func (m *Event) GetState() string {
	if m != nil {
		return m.State
	}
	return ""
}

func (m *Event) GetService() string {
	if m != nil {
		return m.Service
	}
	return ""
}

func (m *Event) GetHost() string {
	if m != nil {
		return m.Host
	}
	return ""
}

func (m *Event) GetDescription() string {
	if m != nil {
		return m.Description
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
	if m != nil {
		return m.Ttl
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
	if m != nil {
		return m.TimeMicros
	}
	return 0
}

func (m *Event) GetMetricSint64() int64 {
	if m != nil {
		return m.MetricSint64
	}
	return 0
}

func (m *Event) GetMetricD() float64 {
	if m != nil {
		return m.MetricD
	}
	return 0
}

func (m *Event) GetMetricF() float32 {
	if m != nil {
		return m.MetricF
	}
	return 0
}

type Query struct {
	String_              string   `protobuf:"bytes,1,opt,name=string,proto3" json:"string,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Query) Reset()         { *m = Query{} }
func (m *Query) String() string { return proto.CompactTextString(m) }
func (*Query) ProtoMessage()    {}
func (*Query) Descriptor() ([]byte, []int) {
	return fileDescriptor_0e43e81d710c3d26, []int{2}
}

func (m *Query) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Query.Unmarshal(m, b)
}
func (m *Query) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Query.Marshal(b, m, deterministic)
}
func (m *Query) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Query.Merge(m, src)
}
func (m *Query) XXX_Size() int {
	return xxx_messageInfo_Query.Size(m)
}
func (m *Query) XXX_DiscardUnknown() {
	xxx_messageInfo_Query.DiscardUnknown(m)
}

var xxx_messageInfo_Query proto.InternalMessageInfo

func (m *Query) GetString_() string {
	if m != nil {
		return m.String_
	}
	return ""
}

type Msg struct {
	Ok                   bool     `protobuf:"varint,2,opt,name=ok,proto3" json:"ok,omitempty"`
	Error                string   `protobuf:"bytes,3,opt,name=error,proto3" json:"error,omitempty"`
	States               []*State `protobuf:"bytes,4,rep,name=states,proto3" json:"states,omitempty"`
	Query                *Query   `protobuf:"bytes,5,opt,name=query,proto3" json:"query,omitempty"`
	Events               []*Event `protobuf:"bytes,6,rep,name=events,proto3" json:"events,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Msg) Reset()         { *m = Msg{} }
func (m *Msg) String() string { return proto.CompactTextString(m) }
func (*Msg) ProtoMessage()    {}
func (*Msg) Descriptor() ([]byte, []int) {
	return fileDescriptor_0e43e81d710c3d26, []int{3}
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
	if m != nil {
		return m.Ok
	}
	return false
}

func (m *Msg) GetError() string {
	if m != nil {
		return m.Error
	}
	return ""
}

func (m *Msg) GetStates() []*State {
	if m != nil {
		return m.States
	}
	return nil
}

func (m *Msg) GetQuery() *Query {
	if m != nil {
		return m.Query
	}
	return nil
}

func (m *Msg) GetEvents() []*Event {
	if m != nil {
		return m.Events
	}
	return nil
}

type Attribute struct {
	Key                  string   `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Value                string   `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Attribute) Reset()         { *m = Attribute{} }
func (m *Attribute) String() string { return proto.CompactTextString(m) }
func (*Attribute) ProtoMessage()    {}
func (*Attribute) Descriptor() ([]byte, []int) {
	return fileDescriptor_0e43e81d710c3d26, []int{4}
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
	if m != nil {
		return m.Key
	}
	return ""
}

func (m *Attribute) GetValue() string {
	if m != nil {
		return m.Value
	}
	return ""
}

func init() {
	proto.RegisterType((*State)(nil), "main.State")
	proto.RegisterType((*Event)(nil), "main.Event")
	proto.RegisterType((*Query)(nil), "main.Query")
	proto.RegisterType((*Msg)(nil), "main.Msg")
	proto.RegisterType((*Attribute)(nil), "main.Attribute")
}

func init() { proto.RegisterFile("riemann.proto", fileDescriptor_0e43e81d710c3d26) }

var fileDescriptor_0e43e81d710c3d26 = []byte{
	// 405 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xc4, 0x53, 0xcd, 0x6e, 0xd4, 0x30,
	0x10, 0x96, 0x93, 0x4d, 0x36, 0x99, 0xb0, 0x2d, 0x1a, 0x21, 0x64, 0x4e, 0x35, 0xe9, 0x25, 0xa7,
	0x45, 0xa2, 0x88, 0x3b, 0x12, 0x70, 0xeb, 0x01, 0xf7, 0x01, 0xaa, 0x34, 0x35, 0x8b, 0xb5, 0x8d,
	0x5d, 0x6c, 0xef, 0x4a, 0x7d, 0x13, 0xde, 0x85, 0xd7, 0xe0, 0x81, 0xd0, 0x4c, 0xd2, 0x55, 0x6e,
	0x1c, 0xb9, 0x7d, 0x3f, 0xde, 0xd9, 0x6f, 0x3e, 0x3b, 0xb0, 0x09, 0xd6, 0x8c, 0xbd, 0x73, 0xdb,
	0xc7, 0xe0, 0x93, 0xc7, 0xd5, 0xd8, 0x5b, 0xd7, 0xfe, 0x16, 0x50, 0xdc, 0xa4, 0x3e, 0x19, 0x44,
	0x58, 0x25, 0x3b, 0x1a, 0x29, 0x94, 0xe8, 0x72, 0xcd, 0x18, 0x5f, 0x41, 0x11, 0xc9, 0x94, 0x99,
	0x12, 0x5d, 0xad, 0x27, 0x82, 0x12, 0xd6, 0xd1, 0x84, 0xa3, 0x1d, 0x8c, 0xcc, 0x59, 0x7f, 0xa6,
	0x34, 0xe3, 0x87, 0x8f, 0x49, 0xae, 0x58, 0x66, 0x8c, 0x0a, 0x9a, 0x7b, 0x13, 0x87, 0x60, 0x1f,
	0x93, 0xf5, 0x4e, 0x16, 0x6c, 0x2d, 0x25, 0xfa, 0x95, 0x77, 0x83, 0x91, 0xa5, 0x12, 0x5d, 0xa5,
	0x19, 0x73, 0x9a, 0x7e, 0x17, 0xe5, 0x5a, 0xe5, 0x34, 0x89, 0x30, 0xbe, 0x84, 0x3c, 0xa5, 0x07,
	0x59, 0x29, 0xd1, 0x65, 0x9a, 0x60, 0xfb, 0x27, 0x83, 0xe2, 0xcb, 0xd1, 0xb8, 0xf4, 0x7f, 0xd3,
	0xff, 0x3b, 0x29, 0xbe, 0x03, 0xe8, 0x53, 0x0a, 0xf6, 0xee, 0x90, 0x4c, 0x94, 0xb5, 0xca, 0xbb,
	0xe6, 0xfd, 0xf9, 0x96, 0xae, 0x60, 0xfb, 0xe9, 0x59, 0xd7, 0x8b, 0x23, 0x78, 0x01, 0x0d, 0x2d,
	0x71, 0x3b, 0xda, 0x21, 0xf8, 0x28, 0x81, 0xf7, 0x02, 0x92, 0xae, 0x59, 0xc1, 0x4b, 0xd8, 0x8c,
	0x26, 0x05, 0x3b, 0xdc, 0x46, 0xeb, 0xd2, 0xc7, 0x0f, 0x72, 0xa3, 0x44, 0x87, 0xfa, 0xc5, 0x24,
	0xde, 0xb0, 0x86, 0x6f, 0xa0, 0x9a, 0x0f, 0xdd, 0xcb, 0x33, 0x25, 0x3a, 0xa1, 0xd7, 0x13, 0xff,
	0xbc, 0xb0, 0xbe, 0xcb, 0x73, 0x0e, 0x3a, 0x5b, 0x5f, 0xdb, 0x0b, 0x28, 0xbe, 0x1d, 0x4c, 0x78,
	0xc2, 0xd7, 0x50, 0xc6, 0x14, 0xac, 0xdb, 0x71, 0xaf, 0xb5, 0x9e, 0x59, 0xfb, 0x4b, 0x40, 0x7e,
	0x1d, 0x77, 0x78, 0x06, 0x99, 0xdf, 0x73, 0xbd, 0x95, 0xce, 0xfc, 0x9e, 0x1a, 0x37, 0x21, 0xf8,
	0x30, 0x37, 0x3b, 0x11, 0xbc, 0xa4, 0x29, 0x3d, 0xed, 0xbd, 0xe2, 0xbd, 0x9b, 0x69, 0x6f, 0x7e,
	0x76, 0x7a, 0xb6, 0xf0, 0x2d, 0x14, 0x3f, 0xe9, 0x3f, 0xb9, 0xe2, 0xd3, 0x19, 0x8e, 0xa1, 0x27,
	0x87, 0xe6, 0x18, 0xba, 0xec, 0x28, 0xcb, 0xe5, 0x1c, 0x7e, 0x00, 0x7a, 0xb6, 0xda, 0x2b, 0xa8,
	0x4f, 0x85, 0xd2, 0x3d, 0xec, 0xcd, 0xd3, 0x1c, 0x9e, 0x20, 0x25, 0x3c, 0xf6, 0x0f, 0x87, 0xd3,
	0x9b, 0x60, 0x72, 0x57, 0xf2, 0x27, 0x71, 0xf5, 0x37, 0x00, 0x00, 0xff, 0xff, 0xb5, 0x78, 0xb6,
	0x4f, 0x23, 0x03, 0x00, 0x00,
}