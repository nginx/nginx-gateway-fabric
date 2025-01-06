// Code generated by counterfeiter. DO NOT EDIT.
package policiesfakes

import (
	"sync"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
)

type FakeGenerator struct {
	GenerateForInternalLocationStub        func([]policies.Policy) policies.GenerateResultFiles
	generateForInternalLocationMutex       sync.RWMutex
	generateForInternalLocationArgsForCall []struct {
		arg1 []policies.Policy
	}
	generateForInternalLocationReturns struct {
		result1 policies.GenerateResultFiles
	}
	generateForInternalLocationReturnsOnCall map[int]struct {
		result1 policies.GenerateResultFiles
	}
	GenerateForLocationStub        func([]policies.Policy, http.Location) policies.GenerateResultFiles
	generateForLocationMutex       sync.RWMutex
	generateForLocationArgsForCall []struct {
		arg1 []policies.Policy
		arg2 http.Location
	}
	generateForLocationReturns struct {
		result1 policies.GenerateResultFiles
	}
	generateForLocationReturnsOnCall map[int]struct {
		result1 policies.GenerateResultFiles
	}
	GenerateForServerStub        func([]policies.Policy, http.Server) policies.GenerateResultFiles
	generateForServerMutex       sync.RWMutex
	generateForServerArgsForCall []struct {
		arg1 []policies.Policy
		arg2 http.Server
	}
	generateForServerReturns struct {
		result1 policies.GenerateResultFiles
	}
	generateForServerReturnsOnCall map[int]struct {
		result1 policies.GenerateResultFiles
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeGenerator) GenerateForInternalLocation(arg1 []policies.Policy) policies.GenerateResultFiles {
	var arg1Copy []policies.Policy
	if arg1 != nil {
		arg1Copy = make([]policies.Policy, len(arg1))
		copy(arg1Copy, arg1)
	}
	fake.generateForInternalLocationMutex.Lock()
	ret, specificReturn := fake.generateForInternalLocationReturnsOnCall[len(fake.generateForInternalLocationArgsForCall)]
	fake.generateForInternalLocationArgsForCall = append(fake.generateForInternalLocationArgsForCall, struct {
		arg1 []policies.Policy
	}{arg1Copy})
	stub := fake.GenerateForInternalLocationStub
	fakeReturns := fake.generateForInternalLocationReturns
	fake.recordInvocation("GenerateForInternalLocation", []interface{}{arg1Copy})
	fake.generateForInternalLocationMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeGenerator) GenerateForInternalLocationCallCount() int {
	fake.generateForInternalLocationMutex.RLock()
	defer fake.generateForInternalLocationMutex.RUnlock()
	return len(fake.generateForInternalLocationArgsForCall)
}

func (fake *FakeGenerator) GenerateForInternalLocationCalls(stub func([]policies.Policy) policies.GenerateResultFiles) {
	fake.generateForInternalLocationMutex.Lock()
	defer fake.generateForInternalLocationMutex.Unlock()
	fake.GenerateForInternalLocationStub = stub
}

func (fake *FakeGenerator) GenerateForInternalLocationArgsForCall(i int) []policies.Policy {
	fake.generateForInternalLocationMutex.RLock()
	defer fake.generateForInternalLocationMutex.RUnlock()
	argsForCall := fake.generateForInternalLocationArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeGenerator) GenerateForInternalLocationReturns(result1 policies.GenerateResultFiles) {
	fake.generateForInternalLocationMutex.Lock()
	defer fake.generateForInternalLocationMutex.Unlock()
	fake.GenerateForInternalLocationStub = nil
	fake.generateForInternalLocationReturns = struct {
		result1 policies.GenerateResultFiles
	}{result1}
}

func (fake *FakeGenerator) GenerateForInternalLocationReturnsOnCall(i int, result1 policies.GenerateResultFiles) {
	fake.generateForInternalLocationMutex.Lock()
	defer fake.generateForInternalLocationMutex.Unlock()
	fake.GenerateForInternalLocationStub = nil
	if fake.generateForInternalLocationReturnsOnCall == nil {
		fake.generateForInternalLocationReturnsOnCall = make(map[int]struct {
			result1 policies.GenerateResultFiles
		})
	}
	fake.generateForInternalLocationReturnsOnCall[i] = struct {
		result1 policies.GenerateResultFiles
	}{result1}
}

func (fake *FakeGenerator) GenerateForLocation(arg1 []policies.Policy, arg2 http.Location) policies.GenerateResultFiles {
	var arg1Copy []policies.Policy
	if arg1 != nil {
		arg1Copy = make([]policies.Policy, len(arg1))
		copy(arg1Copy, arg1)
	}
	fake.generateForLocationMutex.Lock()
	ret, specificReturn := fake.generateForLocationReturnsOnCall[len(fake.generateForLocationArgsForCall)]
	fake.generateForLocationArgsForCall = append(fake.generateForLocationArgsForCall, struct {
		arg1 []policies.Policy
		arg2 http.Location
	}{arg1Copy, arg2})
	stub := fake.GenerateForLocationStub
	fakeReturns := fake.generateForLocationReturns
	fake.recordInvocation("GenerateForLocation", []interface{}{arg1Copy, arg2})
	fake.generateForLocationMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeGenerator) GenerateForLocationCallCount() int {
	fake.generateForLocationMutex.RLock()
	defer fake.generateForLocationMutex.RUnlock()
	return len(fake.generateForLocationArgsForCall)
}

func (fake *FakeGenerator) GenerateForLocationCalls(stub func([]policies.Policy, http.Location) policies.GenerateResultFiles) {
	fake.generateForLocationMutex.Lock()
	defer fake.generateForLocationMutex.Unlock()
	fake.GenerateForLocationStub = stub
}

func (fake *FakeGenerator) GenerateForLocationArgsForCall(i int) ([]policies.Policy, http.Location) {
	fake.generateForLocationMutex.RLock()
	defer fake.generateForLocationMutex.RUnlock()
	argsForCall := fake.generateForLocationArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeGenerator) GenerateForLocationReturns(result1 policies.GenerateResultFiles) {
	fake.generateForLocationMutex.Lock()
	defer fake.generateForLocationMutex.Unlock()
	fake.GenerateForLocationStub = nil
	fake.generateForLocationReturns = struct {
		result1 policies.GenerateResultFiles
	}{result1}
}

func (fake *FakeGenerator) GenerateForLocationReturnsOnCall(i int, result1 policies.GenerateResultFiles) {
	fake.generateForLocationMutex.Lock()
	defer fake.generateForLocationMutex.Unlock()
	fake.GenerateForLocationStub = nil
	if fake.generateForLocationReturnsOnCall == nil {
		fake.generateForLocationReturnsOnCall = make(map[int]struct {
			result1 policies.GenerateResultFiles
		})
	}
	fake.generateForLocationReturnsOnCall[i] = struct {
		result1 policies.GenerateResultFiles
	}{result1}
}

func (fake *FakeGenerator) GenerateForServer(arg1 []policies.Policy, arg2 http.Server) policies.GenerateResultFiles {
	var arg1Copy []policies.Policy
	if arg1 != nil {
		arg1Copy = make([]policies.Policy, len(arg1))
		copy(arg1Copy, arg1)
	}
	fake.generateForServerMutex.Lock()
	ret, specificReturn := fake.generateForServerReturnsOnCall[len(fake.generateForServerArgsForCall)]
	fake.generateForServerArgsForCall = append(fake.generateForServerArgsForCall, struct {
		arg1 []policies.Policy
		arg2 http.Server
	}{arg1Copy, arg2})
	stub := fake.GenerateForServerStub
	fakeReturns := fake.generateForServerReturns
	fake.recordInvocation("GenerateForServer", []interface{}{arg1Copy, arg2})
	fake.generateForServerMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeGenerator) GenerateForServerCallCount() int {
	fake.generateForServerMutex.RLock()
	defer fake.generateForServerMutex.RUnlock()
	return len(fake.generateForServerArgsForCall)
}

func (fake *FakeGenerator) GenerateForServerCalls(stub func([]policies.Policy, http.Server) policies.GenerateResultFiles) {
	fake.generateForServerMutex.Lock()
	defer fake.generateForServerMutex.Unlock()
	fake.GenerateForServerStub = stub
}

func (fake *FakeGenerator) GenerateForServerArgsForCall(i int) ([]policies.Policy, http.Server) {
	fake.generateForServerMutex.RLock()
	defer fake.generateForServerMutex.RUnlock()
	argsForCall := fake.generateForServerArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeGenerator) GenerateForServerReturns(result1 policies.GenerateResultFiles) {
	fake.generateForServerMutex.Lock()
	defer fake.generateForServerMutex.Unlock()
	fake.GenerateForServerStub = nil
	fake.generateForServerReturns = struct {
		result1 policies.GenerateResultFiles
	}{result1}
}

func (fake *FakeGenerator) GenerateForServerReturnsOnCall(i int, result1 policies.GenerateResultFiles) {
	fake.generateForServerMutex.Lock()
	defer fake.generateForServerMutex.Unlock()
	fake.GenerateForServerStub = nil
	if fake.generateForServerReturnsOnCall == nil {
		fake.generateForServerReturnsOnCall = make(map[int]struct {
			result1 policies.GenerateResultFiles
		})
	}
	fake.generateForServerReturnsOnCall[i] = struct {
		result1 policies.GenerateResultFiles
	}{result1}
}

func (fake *FakeGenerator) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.generateForInternalLocationMutex.RLock()
	defer fake.generateForInternalLocationMutex.RUnlock()
	fake.generateForLocationMutex.RLock()
	defer fake.generateForLocationMutex.RUnlock()
	fake.generateForServerMutex.RLock()
	defer fake.generateForServerMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeGenerator) recordInvocation(key string, args []interface{}) {
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

var _ policies.Generator = new(FakeGenerator)
