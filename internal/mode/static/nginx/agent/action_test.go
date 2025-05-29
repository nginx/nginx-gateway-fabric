package agent

import (
	"testing"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestActionsEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		actionA  []*pb.NGINXPlusAction
		actionB  []*pb.NGINXPlusAction
		expected bool
	}{
		{
			name: "Actions are equal",
			actionA: []*pb.NGINXPlusAction{
				{
					Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{
						UpdateHttpUpstreamServers: &pb.UpdateHTTPUpstreamServers{
							HttpUpstreamName: "upstream1",
							Servers: []*structpb.Struct{
								{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value"}}}},
							},
						},
					},
				},
			},
			actionB: []*pb.NGINXPlusAction{
				{
					Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{
						UpdateHttpUpstreamServers: &pb.UpdateHTTPUpstreamServers{
							HttpUpstreamName: "upstream1",
							Servers: []*structpb.Struct{
								{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value"}}}},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Actions have different types",
			actionA: []*pb.NGINXPlusAction{
				{
					Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{
						UpdateHttpUpstreamServers: &pb.UpdateHTTPUpstreamServers{
							HttpUpstreamName: "upstream1",
						},
					},
				},
			},
			actionB: []*pb.NGINXPlusAction{
				{
					Action: &pb.NGINXPlusAction_UpdateStreamServers{
						UpdateStreamServers: &pb.UpdateStreamServers{
							UpstreamStreamName: "upstream1",
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Actions have different values",
			actionA: []*pb.NGINXPlusAction{
				{
					Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{
						UpdateHttpUpstreamServers: &pb.UpdateHTTPUpstreamServers{
							HttpUpstreamName: "upstream1",
							Servers: []*structpb.Struct{
								{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value1"}}}},
							},
						},
					},
				},
			},
			actionB: []*pb.NGINXPlusAction{
				{
					Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{
						UpdateHttpUpstreamServers: &pb.UpdateHTTPUpstreamServers{
							HttpUpstreamName: "upstream1",
							Servers: []*structpb.Struct{
								{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value2"}}}},
							},
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(actionsEqual(tt.actionA, tt.actionB)).To(Equal(tt.expected))
		})
	}
}

func TestHttpUpstreamsEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		upstreamA *pb.UpdateHTTPUpstreamServers
		upstreamB *pb.UpdateHTTPUpstreamServers
		name      string
		expected  bool
	}{
		{
			name: "HTTP upstreams are equal",
			upstreamA: &pb.UpdateHTTPUpstreamServers{
				HttpUpstreamName: "upstream1",
				Servers: []*structpb.Struct{
					{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value"}}}},
				},
			},
			upstreamB: &pb.UpdateHTTPUpstreamServers{
				HttpUpstreamName: "upstream1",
				Servers: []*structpb.Struct{
					{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value"}}}},
				},
			},
			expected: true,
		},
		{
			name: "HTTP upstreams have different upstream names",
			upstreamA: &pb.UpdateHTTPUpstreamServers{
				HttpUpstreamName: "upstream1",
			},
			upstreamB: &pb.UpdateHTTPUpstreamServers{
				HttpUpstreamName: "upstream2",
			},
			expected: false,
		},
		{
			name: "HTTP upstreams have different server lengths",
			upstreamA: &pb.UpdateHTTPUpstreamServers{
				HttpUpstreamName: "upstream1",
				Servers: []*structpb.Struct{
					{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value"}}}},
				},
			},
			upstreamB: &pb.UpdateHTTPUpstreamServers{
				HttpUpstreamName: "upstream1",
				Servers: []*structpb.Struct{
					{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value"}}}},
					{Fields: map[string]*structpb.Value{"key2": {Kind: &structpb.Value_StringValue{StringValue: "value2"}}}},
				},
			},
			expected: false,
		},
		{
			name: "HTTP upstreams have different server contents",
			upstreamA: &pb.UpdateHTTPUpstreamServers{
				HttpUpstreamName: "upstream1",
				Servers: []*structpb.Struct{
					{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value1"}}}},
				},
			},
			upstreamB: &pb.UpdateHTTPUpstreamServers{
				HttpUpstreamName: "upstream1",
				Servers: []*structpb.Struct{
					{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value2"}}}},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(httpUpstreamsEqual(tt.upstreamA, tt.upstreamB)).To(Equal(tt.expected))
		})
	}
}

func TestStreamUpstreamsEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		upstreamA *pb.UpdateStreamServers
		upstreamB *pb.UpdateStreamServers
		name      string
		expected  bool
	}{
		{
			name: "Stream upstreams are equal",
			upstreamA: &pb.UpdateStreamServers{
				UpstreamStreamName: "stream1",
				Servers: []*structpb.Struct{
					{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value"}}}},
				},
			},
			upstreamB: &pb.UpdateStreamServers{
				UpstreamStreamName: "stream1",
				Servers: []*structpb.Struct{
					{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value"}}}},
				},
			},
			expected: true,
		},
		{
			name: "Stream have different upstream names",
			upstreamA: &pb.UpdateStreamServers{
				UpstreamStreamName: "stream1",
			},
			upstreamB: &pb.UpdateStreamServers{
				UpstreamStreamName: "stream2",
			},
			expected: false,
		},
		{
			name: "Stream upstreams have different server lengths",
			upstreamA: &pb.UpdateStreamServers{
				UpstreamStreamName: "stream1",
				Servers: []*structpb.Struct{
					{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value"}}}},
				},
			},
			upstreamB: &pb.UpdateStreamServers{
				UpstreamStreamName: "stream1",
				Servers: []*structpb.Struct{
					{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value"}}}},
					{Fields: map[string]*structpb.Value{"key2": {Kind: &structpb.Value_StringValue{StringValue: "value2"}}}},
				},
			},
			expected: false,
		},
		{
			name: "Stream upstreams have different server contents",
			upstreamA: &pb.UpdateStreamServers{
				UpstreamStreamName: "stream1",
				Servers: []*structpb.Struct{
					{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value1"}}}},
				},
			},
			upstreamB: &pb.UpdateStreamServers{
				UpstreamStreamName: "stream1",
				Servers: []*structpb.Struct{
					{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value2"}}}},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(streamUpstreamsEqual(tt.upstreamA, tt.upstreamB)).To(Equal(tt.expected))
		})
	}
}

func TestStructsEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		structA  *structpb.Struct
		structB  *structpb.Struct
		name     string
		expected bool
	}{
		{
			name: "Structs are equal",
			structA: &structpb.Struct{
				Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value"}}},
			},
			structB: &structpb.Struct{
				Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value"}}},
			},
			expected: true,
		},
		{
			name: "Structs have different values",
			structA: &structpb.Struct{
				Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value"}}},
			},
			structB: &structpb.Struct{
				Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "different"}}},
			},
			expected: false,
		},
		{
			name: "Structs have different keys",
			structA: &structpb.Struct{
				Fields: map[string]*structpb.Value{"key1": {Kind: &structpb.Value_StringValue{StringValue: "value"}}},
			},
			structB: &structpb.Struct{
				Fields: map[string]*structpb.Value{"key2": {Kind: &structpb.Value_StringValue{StringValue: "value"}}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(structsEqual(tt.structA, tt.structB)).To(Equal(tt.expected))
		})
	}
}

func TestValuesEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		valueA   *structpb.Value
		valueB   *structpb.Value
		name     string
		expected bool
	}{
		{
			name:     "Values are equal",
			valueA:   &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: "value"}},
			valueB:   &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: "value"}},
			expected: true,
		},
		{
			name:     "Values are not equal",
			valueA:   &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: "value"}},
			valueB:   &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: "different"}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(valuesEqual(tt.valueA, tt.valueB)).To(Equal(tt.expected))
		})
	}
}
