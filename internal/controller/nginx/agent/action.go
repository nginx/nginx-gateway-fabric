package agent

import (
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

func actionsEqual(a, b []*pb.NGINXPlusAction) bool {
	if len(a) != len(b) {
		return false
	}

	aHTTP, aStream := indexActionsByName(a)
	bHTTP, bStream := indexActionsByName(b)

	if len(aHTTP) != len(bHTTP) || len(aStream) != len(bStream) {
		return false
	}

	for name, actionA := range aHTTP {
		actionB, ok := bHTTP[name]
		if !ok || !httpUpstreamsEqual(actionA, actionB) {
			return false
		}
	}

	for name, actionA := range aStream {
		actionB, ok := bStream[name]
		if !ok || !streamUpstreamsEqual(actionA, actionB) {
			return false
		}
	}

	return true
}

// indexActionsByName splits a list of NGINXPlusActions into two maps, keyed by upstream name:
// one for HTTP upstream actions and one for stream upstream actions.
func indexActionsByName(
	actions []*pb.NGINXPlusAction,
) (map[string]*pb.UpdateHTTPUpstreamServers, map[string]*pb.UpdateStreamServers) {
	httpActions := make(map[string]*pb.UpdateHTTPUpstreamServers, len(actions))
	streamActions := make(map[string]*pb.UpdateStreamServers, len(actions))

	for _, action := range actions {
		switch a := action.Action.(type) {
		case *pb.NGINXPlusAction_UpdateHttpUpstreamServers:
			httpActions[a.UpdateHttpUpstreamServers.GetHttpUpstreamName()] = a.UpdateHttpUpstreamServers
		case *pb.NGINXPlusAction_UpdateStreamServers:
			streamActions[a.UpdateStreamServers.GetUpstreamStreamName()] = a.UpdateStreamServers
		}
	}

	return httpActions, streamActions
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
