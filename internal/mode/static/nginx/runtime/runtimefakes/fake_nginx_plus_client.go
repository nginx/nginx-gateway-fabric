// Code generated by counterfeiter. DO NOT EDIT.
package runtimefakes

import (
	"sync"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/runtime"
	"github.com/nginxinc/nginx-plus-go-client/client"
)

type FakeNginxPlusClient struct {
	GetStreamUpstreamsStub        func() (*client.StreamUpstreams, error)
	getStreamUpstreamsMutex       sync.RWMutex
	getStreamUpstreamsArgsForCall []struct {
	}
	getStreamUpstreamsReturns struct {
		result1 *client.StreamUpstreams
		result2 error
	}
	getStreamUpstreamsReturnsOnCall map[int]struct {
		result1 *client.StreamUpstreams
		result2 error
	}
	GetUpstreamsStub        func() (*client.Upstreams, error)
	getUpstreamsMutex       sync.RWMutex
	getUpstreamsArgsForCall []struct {
	}
	getUpstreamsReturns struct {
		result1 *client.Upstreams
		result2 error
	}
	getUpstreamsReturnsOnCall map[int]struct {
		result1 *client.Upstreams
		result2 error
	}
	UpdateHTTPServersStub        func(string, []client.UpstreamServer) ([]client.UpstreamServer, []client.UpstreamServer, []client.UpstreamServer, error)
	updateHTTPServersMutex       sync.RWMutex
	updateHTTPServersArgsForCall []struct {
		arg1 string
		arg2 []client.UpstreamServer
	}
	updateHTTPServersReturns struct {
		result1 []client.UpstreamServer
		result2 []client.UpstreamServer
		result3 []client.UpstreamServer
		result4 error
	}
	updateHTTPServersReturnsOnCall map[int]struct {
		result1 []client.UpstreamServer
		result2 []client.UpstreamServer
		result3 []client.UpstreamServer
		result4 error
	}
	UpdateStreamServersStub        func(string, []client.StreamUpstreamServer) ([]client.StreamUpstreamServer, []client.StreamUpstreamServer, []client.StreamUpstreamServer, error)
	updateStreamServersMutex       sync.RWMutex
	updateStreamServersArgsForCall []struct {
		arg1 string
		arg2 []client.StreamUpstreamServer
	}
	updateStreamServersReturns struct {
		result1 []client.StreamUpstreamServer
		result2 []client.StreamUpstreamServer
		result3 []client.StreamUpstreamServer
		result4 error
	}
	updateStreamServersReturnsOnCall map[int]struct {
		result1 []client.StreamUpstreamServer
		result2 []client.StreamUpstreamServer
		result3 []client.StreamUpstreamServer
		result4 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeNginxPlusClient) GetStreamUpstreams() (*client.StreamUpstreams, error) {
	fake.getStreamUpstreamsMutex.Lock()
	ret, specificReturn := fake.getStreamUpstreamsReturnsOnCall[len(fake.getStreamUpstreamsArgsForCall)]
	fake.getStreamUpstreamsArgsForCall = append(fake.getStreamUpstreamsArgsForCall, struct {
	}{})
	stub := fake.GetStreamUpstreamsStub
	fakeReturns := fake.getStreamUpstreamsReturns
	fake.recordInvocation("GetStreamUpstreams", []interface{}{})
	fake.getStreamUpstreamsMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeNginxPlusClient) GetStreamUpstreamsCallCount() int {
	fake.getStreamUpstreamsMutex.RLock()
	defer fake.getStreamUpstreamsMutex.RUnlock()
	return len(fake.getStreamUpstreamsArgsForCall)
}

func (fake *FakeNginxPlusClient) GetStreamUpstreamsCalls(stub func() (*client.StreamUpstreams, error)) {
	fake.getStreamUpstreamsMutex.Lock()
	defer fake.getStreamUpstreamsMutex.Unlock()
	fake.GetStreamUpstreamsStub = stub
}

func (fake *FakeNginxPlusClient) GetStreamUpstreamsReturns(result1 *client.StreamUpstreams, result2 error) {
	fake.getStreamUpstreamsMutex.Lock()
	defer fake.getStreamUpstreamsMutex.Unlock()
	fake.GetStreamUpstreamsStub = nil
	fake.getStreamUpstreamsReturns = struct {
		result1 *client.StreamUpstreams
		result2 error
	}{result1, result2}
}

func (fake *FakeNginxPlusClient) GetStreamUpstreamsReturnsOnCall(i int, result1 *client.StreamUpstreams, result2 error) {
	fake.getStreamUpstreamsMutex.Lock()
	defer fake.getStreamUpstreamsMutex.Unlock()
	fake.GetStreamUpstreamsStub = nil
	if fake.getStreamUpstreamsReturnsOnCall == nil {
		fake.getStreamUpstreamsReturnsOnCall = make(map[int]struct {
			result1 *client.StreamUpstreams
			result2 error
		})
	}
	fake.getStreamUpstreamsReturnsOnCall[i] = struct {
		result1 *client.StreamUpstreams
		result2 error
	}{result1, result2}
}

func (fake *FakeNginxPlusClient) GetUpstreams() (*client.Upstreams, error) {
	fake.getUpstreamsMutex.Lock()
	ret, specificReturn := fake.getUpstreamsReturnsOnCall[len(fake.getUpstreamsArgsForCall)]
	fake.getUpstreamsArgsForCall = append(fake.getUpstreamsArgsForCall, struct {
	}{})
	stub := fake.GetUpstreamsStub
	fakeReturns := fake.getUpstreamsReturns
	fake.recordInvocation("GetUpstreams", []interface{}{})
	fake.getUpstreamsMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeNginxPlusClient) GetUpstreamsCallCount() int {
	fake.getUpstreamsMutex.RLock()
	defer fake.getUpstreamsMutex.RUnlock()
	return len(fake.getUpstreamsArgsForCall)
}

func (fake *FakeNginxPlusClient) GetUpstreamsCalls(stub func() (*client.Upstreams, error)) {
	fake.getUpstreamsMutex.Lock()
	defer fake.getUpstreamsMutex.Unlock()
	fake.GetUpstreamsStub = stub
}

func (fake *FakeNginxPlusClient) GetUpstreamsReturns(result1 *client.Upstreams, result2 error) {
	fake.getUpstreamsMutex.Lock()
	defer fake.getUpstreamsMutex.Unlock()
	fake.GetUpstreamsStub = nil
	fake.getUpstreamsReturns = struct {
		result1 *client.Upstreams
		result2 error
	}{result1, result2}
}

func (fake *FakeNginxPlusClient) GetUpstreamsReturnsOnCall(i int, result1 *client.Upstreams, result2 error) {
	fake.getUpstreamsMutex.Lock()
	defer fake.getUpstreamsMutex.Unlock()
	fake.GetUpstreamsStub = nil
	if fake.getUpstreamsReturnsOnCall == nil {
		fake.getUpstreamsReturnsOnCall = make(map[int]struct {
			result1 *client.Upstreams
			result2 error
		})
	}
	fake.getUpstreamsReturnsOnCall[i] = struct {
		result1 *client.Upstreams
		result2 error
	}{result1, result2}
}

func (fake *FakeNginxPlusClient) UpdateHTTPServers(arg1 string, arg2 []client.UpstreamServer) ([]client.UpstreamServer, []client.UpstreamServer, []client.UpstreamServer, error) {
	var arg2Copy []client.UpstreamServer
	if arg2 != nil {
		arg2Copy = make([]client.UpstreamServer, len(arg2))
		copy(arg2Copy, arg2)
	}
	fake.updateHTTPServersMutex.Lock()
	ret, specificReturn := fake.updateHTTPServersReturnsOnCall[len(fake.updateHTTPServersArgsForCall)]
	fake.updateHTTPServersArgsForCall = append(fake.updateHTTPServersArgsForCall, struct {
		arg1 string
		arg2 []client.UpstreamServer
	}{arg1, arg2Copy})
	stub := fake.UpdateHTTPServersStub
	fakeReturns := fake.updateHTTPServersReturns
	fake.recordInvocation("UpdateHTTPServers", []interface{}{arg1, arg2Copy})
	fake.updateHTTPServersMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2, ret.result3, ret.result4
	}
	return fakeReturns.result1, fakeReturns.result2, fakeReturns.result3, fakeReturns.result4
}

func (fake *FakeNginxPlusClient) UpdateHTTPServersCallCount() int {
	fake.updateHTTPServersMutex.RLock()
	defer fake.updateHTTPServersMutex.RUnlock()
	return len(fake.updateHTTPServersArgsForCall)
}

func (fake *FakeNginxPlusClient) UpdateHTTPServersCalls(stub func(string, []client.UpstreamServer) ([]client.UpstreamServer, []client.UpstreamServer, []client.UpstreamServer, error)) {
	fake.updateHTTPServersMutex.Lock()
	defer fake.updateHTTPServersMutex.Unlock()
	fake.UpdateHTTPServersStub = stub
}

func (fake *FakeNginxPlusClient) UpdateHTTPServersArgsForCall(i int) (string, []client.UpstreamServer) {
	fake.updateHTTPServersMutex.RLock()
	defer fake.updateHTTPServersMutex.RUnlock()
	argsForCall := fake.updateHTTPServersArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeNginxPlusClient) UpdateHTTPServersReturns(result1 []client.UpstreamServer, result2 []client.UpstreamServer, result3 []client.UpstreamServer, result4 error) {
	fake.updateHTTPServersMutex.Lock()
	defer fake.updateHTTPServersMutex.Unlock()
	fake.UpdateHTTPServersStub = nil
	fake.updateHTTPServersReturns = struct {
		result1 []client.UpstreamServer
		result2 []client.UpstreamServer
		result3 []client.UpstreamServer
		result4 error
	}{result1, result2, result3, result4}
}

func (fake *FakeNginxPlusClient) UpdateHTTPServersReturnsOnCall(i int, result1 []client.UpstreamServer, result2 []client.UpstreamServer, result3 []client.UpstreamServer, result4 error) {
	fake.updateHTTPServersMutex.Lock()
	defer fake.updateHTTPServersMutex.Unlock()
	fake.UpdateHTTPServersStub = nil
	if fake.updateHTTPServersReturnsOnCall == nil {
		fake.updateHTTPServersReturnsOnCall = make(map[int]struct {
			result1 []client.UpstreamServer
			result2 []client.UpstreamServer
			result3 []client.UpstreamServer
			result4 error
		})
	}
	fake.updateHTTPServersReturnsOnCall[i] = struct {
		result1 []client.UpstreamServer
		result2 []client.UpstreamServer
		result3 []client.UpstreamServer
		result4 error
	}{result1, result2, result3, result4}
}

func (fake *FakeNginxPlusClient) UpdateStreamServers(arg1 string, arg2 []client.StreamUpstreamServer) ([]client.StreamUpstreamServer, []client.StreamUpstreamServer, []client.StreamUpstreamServer, error) {
	var arg2Copy []client.StreamUpstreamServer
	if arg2 != nil {
		arg2Copy = make([]client.StreamUpstreamServer, len(arg2))
		copy(arg2Copy, arg2)
	}
	fake.updateStreamServersMutex.Lock()
	ret, specificReturn := fake.updateStreamServersReturnsOnCall[len(fake.updateStreamServersArgsForCall)]
	fake.updateStreamServersArgsForCall = append(fake.updateStreamServersArgsForCall, struct {
		arg1 string
		arg2 []client.StreamUpstreamServer
	}{arg1, arg2Copy})
	stub := fake.UpdateStreamServersStub
	fakeReturns := fake.updateStreamServersReturns
	fake.recordInvocation("UpdateStreamServers", []interface{}{arg1, arg2Copy})
	fake.updateStreamServersMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2, ret.result3, ret.result4
	}
	return fakeReturns.result1, fakeReturns.result2, fakeReturns.result3, fakeReturns.result4
}

func (fake *FakeNginxPlusClient) UpdateStreamServersCallCount() int {
	fake.updateStreamServersMutex.RLock()
	defer fake.updateStreamServersMutex.RUnlock()
	return len(fake.updateStreamServersArgsForCall)
}

func (fake *FakeNginxPlusClient) UpdateStreamServersCalls(stub func(string, []client.StreamUpstreamServer) ([]client.StreamUpstreamServer, []client.StreamUpstreamServer, []client.StreamUpstreamServer, error)) {
	fake.updateStreamServersMutex.Lock()
	defer fake.updateStreamServersMutex.Unlock()
	fake.UpdateStreamServersStub = stub
}

func (fake *FakeNginxPlusClient) UpdateStreamServersArgsForCall(i int) (string, []client.StreamUpstreamServer) {
	fake.updateStreamServersMutex.RLock()
	defer fake.updateStreamServersMutex.RUnlock()
	argsForCall := fake.updateStreamServersArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeNginxPlusClient) UpdateStreamServersReturns(result1 []client.StreamUpstreamServer, result2 []client.StreamUpstreamServer, result3 []client.StreamUpstreamServer, result4 error) {
	fake.updateStreamServersMutex.Lock()
	defer fake.updateStreamServersMutex.Unlock()
	fake.UpdateStreamServersStub = nil
	fake.updateStreamServersReturns = struct {
		result1 []client.StreamUpstreamServer
		result2 []client.StreamUpstreamServer
		result3 []client.StreamUpstreamServer
		result4 error
	}{result1, result2, result3, result4}
}

func (fake *FakeNginxPlusClient) UpdateStreamServersReturnsOnCall(i int, result1 []client.StreamUpstreamServer, result2 []client.StreamUpstreamServer, result3 []client.StreamUpstreamServer, result4 error) {
	fake.updateStreamServersMutex.Lock()
	defer fake.updateStreamServersMutex.Unlock()
	fake.UpdateStreamServersStub = nil
	if fake.updateStreamServersReturnsOnCall == nil {
		fake.updateStreamServersReturnsOnCall = make(map[int]struct {
			result1 []client.StreamUpstreamServer
			result2 []client.StreamUpstreamServer
			result3 []client.StreamUpstreamServer
			result4 error
		})
	}
	fake.updateStreamServersReturnsOnCall[i] = struct {
		result1 []client.StreamUpstreamServer
		result2 []client.StreamUpstreamServer
		result3 []client.StreamUpstreamServer
		result4 error
	}{result1, result2, result3, result4}
}

func (fake *FakeNginxPlusClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.getStreamUpstreamsMutex.RLock()
	defer fake.getStreamUpstreamsMutex.RUnlock()
	fake.getUpstreamsMutex.RLock()
	defer fake.getUpstreamsMutex.RUnlock()
	fake.updateHTTPServersMutex.RLock()
	defer fake.updateHTTPServersMutex.RUnlock()
	fake.updateStreamServersMutex.RLock()
	defer fake.updateStreamServersMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeNginxPlusClient) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ runtime.NginxPlusClient = new(FakeNginxPlusClient)
