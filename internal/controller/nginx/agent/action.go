package agent

import (
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

func actionsEqual(a, b []*pb.NGINXPlusAction) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		switch actionA := a[i].Action.(type) {
		case *pb.NGINXPlusAction_UpdateHttpUpstreamServers:
			actionB, ok := b[i].Action.(*pb.NGINXPlusAction_UpdateHttpUpstreamServers)
			if !ok || !httpUpstreamsEqual(actionA.UpdateHttpUpstreamServers, actionB.UpdateHttpUpstreamServers) {
				return false
			}
		case *pb.NGINXPlusAction_UpdateStreamServers:
			actionB, ok := b[i].Action.(*pb.NGINXPlusAction_UpdateStreamServers)
			if !ok || !streamUpstreamsEqual(actionA.UpdateStreamServers, actionB.UpdateStreamServers) {
				return false
			}
		default:
			return false
		}
	}

	return true
}

func httpUpstreamsEqual(a, b *pb.UpdateHTTPUpstreamServers) bool {
	if a.HttpUpstreamName != b.HttpUpstreamName {
		return false
	}

	if len(a.Servers) != len(b.Servers) {
		return false
	}

	for i := range a.Servers {
		if !structsEqual(a.Servers[i], b.Servers[i]) {
			return false
		}
	}

	return true
}

func streamUpstreamsEqual(a, b *pb.UpdateStreamServers) bool {
	if a.UpstreamStreamName != b.UpstreamStreamName {
		return false
	}

	if len(a.Servers) != len(b.Servers) {
		return false
	}

	for i := range a.Servers {
		if !structsEqual(a.Servers[i], b.Servers[i]) {
			return false
		}
	}

	return true
}

func structsEqual(a, b *structpb.Struct) bool {
	if len(a.Fields) != len(b.Fields) {
		return false
	}

	for key, valueA := range a.Fields {
		valueB, exists := b.Fields[key]
		if !exists || !valuesEqual(valueA, valueB) {
			return false
		}
	}

	return true
}

func valuesEqual(a, b *structpb.Value) bool {
	switch valueA := a.Kind.(type) {
	case *structpb.Value_StringValue:
		valueB, ok := b.Kind.(*structpb.Value_StringValue)
		return ok && valueA.StringValue == valueB.StringValue
	default:
		return false
	}
}
