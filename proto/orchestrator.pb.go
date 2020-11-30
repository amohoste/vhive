// MIT License
//
// Copyright (c) 2020 Dmitrii Ustiugov and EASE lab
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Code generated by protoc-gen-go. DO NOT EDIT.
// source: orchestrator.proto

package proto

import (
	context "context"
	fmt "fmt"
	math "math"

	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
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

type StartVMReq struct {
	Image                string   `protobuf:"bytes,1,opt,name=image,proto3" json:"image,omitempty"`
	Id                   string   `protobuf:"bytes,2,opt,name=id,proto3" json:"id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *StartVMReq) Reset()         { *m = StartVMReq{} }
func (m *StartVMReq) String() string { return proto.CompactTextString(m) }
func (*StartVMReq) ProtoMessage()    {}
func (*StartVMReq) Descriptor() ([]byte, []int) {
	return fileDescriptor_96b6e6782baaa298, []int{0}
}

func (m *StartVMReq) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StartVMReq.Unmarshal(m, b)
}
func (m *StartVMReq) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StartVMReq.Marshal(b, m, deterministic)
}
func (m *StartVMReq) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StartVMReq.Merge(m, src)
}
func (m *StartVMReq) XXX_Size() int {
	return xxx_messageInfo_StartVMReq.Size(m)
}
func (m *StartVMReq) XXX_DiscardUnknown() {
	xxx_messageInfo_StartVMReq.DiscardUnknown(m)
}

var xxx_messageInfo_StartVMReq proto.InternalMessageInfo

func (m *StartVMReq) GetImage() string {
	if m != nil {
		return m.Image
	}
	return ""
}

func (m *StartVMReq) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

type StopVMsReq struct {
	AllVms               bool     `protobuf:"varint,1,opt,name=all_vms,json=allVms,proto3" json:"all_vms,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *StopVMsReq) Reset()         { *m = StopVMsReq{} }
func (m *StopVMsReq) String() string { return proto.CompactTextString(m) }
func (*StopVMsReq) ProtoMessage()    {}
func (*StopVMsReq) Descriptor() ([]byte, []int) {
	return fileDescriptor_96b6e6782baaa298, []int{1}
}

func (m *StopVMsReq) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StopVMsReq.Unmarshal(m, b)
}
func (m *StopVMsReq) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StopVMsReq.Marshal(b, m, deterministic)
}
func (m *StopVMsReq) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StopVMsReq.Merge(m, src)
}
func (m *StopVMsReq) XXX_Size() int {
	return xxx_messageInfo_StopVMsReq.Size(m)
}
func (m *StopVMsReq) XXX_DiscardUnknown() {
	xxx_messageInfo_StopVMsReq.DiscardUnknown(m)
}

var xxx_messageInfo_StopVMsReq proto.InternalMessageInfo

func (m *StopVMsReq) GetAllVms() bool {
	if m != nil {
		return m.AllVms
	}
	return false
}

type StopSingleVMReq struct {
	Id                   string   `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *StopSingleVMReq) Reset()         { *m = StopSingleVMReq{} }
func (m *StopSingleVMReq) String() string { return proto.CompactTextString(m) }
func (*StopSingleVMReq) ProtoMessage()    {}
func (*StopSingleVMReq) Descriptor() ([]byte, []int) {
	return fileDescriptor_96b6e6782baaa298, []int{2}
}

func (m *StopSingleVMReq) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StopSingleVMReq.Unmarshal(m, b)
}
func (m *StopSingleVMReq) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StopSingleVMReq.Marshal(b, m, deterministic)
}
func (m *StopSingleVMReq) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StopSingleVMReq.Merge(m, src)
}
func (m *StopSingleVMReq) XXX_Size() int {
	return xxx_messageInfo_StopSingleVMReq.Size(m)
}
func (m *StopSingleVMReq) XXX_DiscardUnknown() {
	xxx_messageInfo_StopSingleVMReq.DiscardUnknown(m)
}

var xxx_messageInfo_StopSingleVMReq proto.InternalMessageInfo

func (m *StopSingleVMReq) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

type Status struct {
	Message              string   `protobuf:"bytes,1,opt,name=message,proto3" json:"message,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Status) Reset()         { *m = Status{} }
func (m *Status) String() string { return proto.CompactTextString(m) }
func (*Status) ProtoMessage()    {}
func (*Status) Descriptor() ([]byte, []int) {
	return fileDescriptor_96b6e6782baaa298, []int{3}
}

func (m *Status) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Status.Unmarshal(m, b)
}
func (m *Status) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Status.Marshal(b, m, deterministic)
}
func (m *Status) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Status.Merge(m, src)
}
func (m *Status) XXX_Size() int {
	return xxx_messageInfo_Status.Size(m)
}
func (m *Status) XXX_DiscardUnknown() {
	xxx_messageInfo_Status.DiscardUnknown(m)
}

var xxx_messageInfo_Status proto.InternalMessageInfo

func (m *Status) GetMessage() string {
	if m != nil {
		return m.Message
	}
	return ""
}

type StartVMResp struct {
	Message              string   `protobuf:"bytes,1,opt,name=message,proto3" json:"message,omitempty"`
	Profile              string   `protobuf:"bytes,2,opt,name=profile,proto3" json:"profile,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *StartVMResp) Reset()         { *m = StartVMResp{} }
func (m *StartVMResp) String() string { return proto.CompactTextString(m) }
func (*StartVMResp) ProtoMessage()    {}
func (*StartVMResp) Descriptor() ([]byte, []int) {
	return fileDescriptor_96b6e6782baaa298, []int{4}
}

func (m *StartVMResp) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StartVMResp.Unmarshal(m, b)
}
func (m *StartVMResp) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StartVMResp.Marshal(b, m, deterministic)
}
func (m *StartVMResp) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StartVMResp.Merge(m, src)
}
func (m *StartVMResp) XXX_Size() int {
	return xxx_messageInfo_StartVMResp.Size(m)
}
func (m *StartVMResp) XXX_DiscardUnknown() {
	xxx_messageInfo_StartVMResp.DiscardUnknown(m)
}

var xxx_messageInfo_StartVMResp proto.InternalMessageInfo

func (m *StartVMResp) GetMessage() string {
	if m != nil {
		return m.Message
	}
	return ""
}

func (m *StartVMResp) GetProfile() string {
	if m != nil {
		return m.Profile
	}
	return ""
}

func init() {
	proto.RegisterType((*StartVMReq)(nil), "proto.StartVMReq")
	proto.RegisterType((*StopVMsReq)(nil), "proto.StopVMsReq")
	proto.RegisterType((*StopSingleVMReq)(nil), "proto.StopSingleVMReq")
	proto.RegisterType((*Status)(nil), "proto.Status")
	proto.RegisterType((*StartVMResp)(nil), "proto.StartVMResp")
}

func init() { proto.RegisterFile("orchestrator.proto", fileDescriptor_96b6e6782baaa298) }

var fileDescriptor_96b6e6782baaa298 = []byte{
	// 281 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x74, 0x91, 0x51, 0x4b, 0xfb, 0x30,
	0x14, 0xc5, 0xd7, 0xc1, 0xda, 0xff, 0xff, 0x3a, 0x95, 0x5d, 0x44, 0xcb, 0x40, 0xd0, 0x80, 0xe0,
	0x8b, 0x7d, 0xa8, 0x82, 0xcf, 0xee, 0x7d, 0x38, 0x5a, 0xe8, 0xab, 0xc4, 0x2d, 0xd6, 0x40, 0x6a,
	0x62, 0x6e, 0x26, 0x7e, 0x26, 0x3f, 0xa5, 0xa4, 0x5d, 0xd7, 0xa0, 0xf8, 0x14, 0xee, 0xcd, 0xfd,
	0x9d, 0xdc, 0x73, 0x02, 0xa8, 0xed, 0xfa, 0x55, 0x90, 0xb3, 0xdc, 0x69, 0x9b, 0x19, 0xab, 0x9d,
	0xc6, 0x49, 0x7b, 0xb0, 0x1c, 0xa0, 0x74, 0xdc, 0xba, 0x6a, 0x59, 0x88, 0x77, 0x3c, 0x81, 0x89,
	0x6c, 0x78, 0x2d, 0xd2, 0xe8, 0x22, 0xba, 0xfe, 0x5f, 0x74, 0x05, 0x1e, 0xc1, 0x58, 0x6e, 0xd2,
	0x71, 0xdb, 0x1a, 0xcb, 0x0d, 0xbb, 0xf2, 0x8c, 0x36, 0xd5, 0x92, 0x3c, 0x73, 0x06, 0x09, 0x57,
	0xea, 0xe9, 0xa3, 0xa1, 0x96, 0xfa, 0x57, 0xc4, 0x5c, 0xa9, 0xaa, 0x21, 0x76, 0x09, 0xc7, 0x7e,
	0xac, 0x94, 0x6f, 0xb5, 0x12, 0x9d, 0x7e, 0xa7, 0x14, 0xed, 0x95, 0x18, 0xc4, 0xa5, 0xe3, 0x6e,
	0x4b, 0x98, 0x42, 0xd2, 0x08, 0xa2, 0xe1, 0xed, 0xbe, 0x64, 0x0f, 0x70, 0xb0, 0xdf, 0x90, 0xcc,
	0xdf, 0x83, 0xfe, 0xc6, 0x58, 0xfd, 0x22, 0x95, 0xd8, 0xed, 0xda, 0x97, 0xf9, 0x57, 0x04, 0xd3,
	0xc7, 0x20, 0x02, 0xcc, 0x21, 0xd9, 0x69, 0xe2, 0xac, 0xcb, 0x23, 0x1b, 0x52, 0x98, 0xe3, 0xcf,
	0x16, 0x19, 0x36, 0xc2, 0x1b, 0xcf, 0xb4, 0xae, 0x03, 0xa6, 0x4f, 0x61, 0x7e, 0x38, 0x30, 0x6e,
	0x4b, 0x6c, 0x84, 0xf7, 0x30, 0x0d, 0xdd, 0xe3, 0x69, 0xc0, 0x04, 0x91, 0xfc, 0x02, 0x17, 0x77,
	0x70, 0x2e, 0x75, 0x56, 0x5b, 0xb3, 0xce, 0xc4, 0x27, 0x6f, 0x8c, 0x12, 0x94, 0x85, 0xff, 0xb7,
	0x98, 0x85, 0x56, 0x56, 0x1e, 0x5e, 0x45, 0xcf, 0x71, 0xab, 0x72, 0xfb, 0x1d, 0x00, 0x00, 0xff,
	0xff, 0x9e, 0x34, 0x34, 0xa7, 0xeb, 0x01, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// OrchestratorClient is the client API for Orchestrator service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type OrchestratorClient interface {
	StartVM(ctx context.Context, in *StartVMReq, opts ...grpc.CallOption) (*StartVMResp, error)
	StopVMs(ctx context.Context, in *StopVMsReq, opts ...grpc.CallOption) (*Status, error)
	StopSingleVM(ctx context.Context, in *StopSingleVMReq, opts ...grpc.CallOption) (*Status, error)
}

type orchestratorClient struct {
	cc grpc.ClientConnInterface
}

func NewOrchestratorClient(cc grpc.ClientConnInterface) OrchestratorClient {
	return &orchestratorClient{cc}
}

func (c *orchestratorClient) StartVM(ctx context.Context, in *StartVMReq, opts ...grpc.CallOption) (*StartVMResp, error) {
	out := new(StartVMResp)
	err := c.cc.Invoke(ctx, "/proto.Orchestrator/StartVM", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orchestratorClient) StopVMs(ctx context.Context, in *StopVMsReq, opts ...grpc.CallOption) (*Status, error) {
	out := new(Status)
	err := c.cc.Invoke(ctx, "/proto.Orchestrator/StopVMs", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orchestratorClient) StopSingleVM(ctx context.Context, in *StopSingleVMReq, opts ...grpc.CallOption) (*Status, error) {
	out := new(Status)
	err := c.cc.Invoke(ctx, "/proto.Orchestrator/StopSingleVM", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// OrchestratorServer is the server API for Orchestrator service.
type OrchestratorServer interface {
	StartVM(context.Context, *StartVMReq) (*StartVMResp, error)
	StopVMs(context.Context, *StopVMsReq) (*Status, error)
	StopSingleVM(context.Context, *StopSingleVMReq) (*Status, error)
}

// UnimplementedOrchestratorServer can be embedded to have forward compatible implementations.
type UnimplementedOrchestratorServer struct {
}

func (*UnimplementedOrchestratorServer) StartVM(ctx context.Context, req *StartVMReq) (*StartVMResp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StartVM not implemented")
}
func (*UnimplementedOrchestratorServer) StopVMs(ctx context.Context, req *StopVMsReq) (*Status, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StopVMs not implemented")
}
func (*UnimplementedOrchestratorServer) StopSingleVM(ctx context.Context, req *StopSingleVMReq) (*Status, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StopSingleVM not implemented")
}

func RegisterOrchestratorServer(s *grpc.Server, srv OrchestratorServer) {
	s.RegisterService(&_Orchestrator_serviceDesc, srv)
}

func _Orchestrator_StartVM_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StartVMReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrchestratorServer).StartVM(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.Orchestrator/StartVM",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrchestratorServer).StartVM(ctx, req.(*StartVMReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orchestrator_StopVMs_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StopVMsReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrchestratorServer).StopVMs(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.Orchestrator/StopVMs",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrchestratorServer).StopVMs(ctx, req.(*StopVMsReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orchestrator_StopSingleVM_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StopSingleVMReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrchestratorServer).StopSingleVM(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.Orchestrator/StopSingleVM",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrchestratorServer).StopSingleVM(ctx, req.(*StopSingleVMReq))
	}
	return interceptor(ctx, in, info, handler)
}

var _Orchestrator_serviceDesc = grpc.ServiceDesc{
	ServiceName: "proto.Orchestrator",
	HandlerType: (*OrchestratorServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "StartVM",
			Handler:    _Orchestrator_StartVM_Handler,
		},
		{
			MethodName: "StopVMs",
			Handler:    _Orchestrator_StopVMs_Handler,
		},
		{
			MethodName: "StopSingleVM",
			Handler:    _Orchestrator_StopSingleVM_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "orchestrator.proto",
}
