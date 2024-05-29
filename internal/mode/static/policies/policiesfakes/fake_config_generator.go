// Code generated by counterfeiter. DO NOT EDIT.
package policiesfakes

import (
	"sync"

	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/policies"
)

type FakeConfigGenerator struct {
	GenerateStub        func(policies.Policy, *policies.GlobalSettings) []byte
	generateMutex       sync.RWMutex
	generateArgsForCall []struct {
		arg1 policies.Policy
		arg2 *policies.GlobalSettings
	}
	generateReturns struct {
		result1 []byte
	}
	generateReturnsOnCall map[int]struct {
		result1 []byte
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeConfigGenerator) Generate(arg1 policies.Policy, arg2 *policies.GlobalSettings) []byte {
	fake.generateMutex.Lock()
	ret, specificReturn := fake.generateReturnsOnCall[len(fake.generateArgsForCall)]
	fake.generateArgsForCall = append(fake.generateArgsForCall, struct {
		arg1 policies.Policy
		arg2 *policies.GlobalSettings
	}{arg1, arg2})
	stub := fake.GenerateStub
	fakeReturns := fake.generateReturns
	fake.recordInvocation("Generate", []interface{}{arg1, arg2})
	fake.generateMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeConfigGenerator) GenerateCallCount() int {
	fake.generateMutex.RLock()
	defer fake.generateMutex.RUnlock()
	return len(fake.generateArgsForCall)
}

func (fake *FakeConfigGenerator) GenerateCalls(stub func(policies.Policy, *policies.GlobalSettings) []byte) {
	fake.generateMutex.Lock()
	defer fake.generateMutex.Unlock()
	fake.GenerateStub = stub
}

func (fake *FakeConfigGenerator) GenerateArgsForCall(i int) (policies.Policy, *policies.GlobalSettings) {
	fake.generateMutex.RLock()
	defer fake.generateMutex.RUnlock()
	argsForCall := fake.generateArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeConfigGenerator) GenerateReturns(result1 []byte) {
	fake.generateMutex.Lock()
	defer fake.generateMutex.Unlock()
	fake.GenerateStub = nil
	fake.generateReturns = struct {
		result1 []byte
	}{result1}
}

func (fake *FakeConfigGenerator) GenerateReturnsOnCall(i int, result1 []byte) {
	fake.generateMutex.Lock()
	defer fake.generateMutex.Unlock()
	fake.GenerateStub = nil
	if fake.generateReturnsOnCall == nil {
		fake.generateReturnsOnCall = make(map[int]struct {
			result1 []byte
		})
	}
	fake.generateReturnsOnCall[i] = struct {
		result1 []byte
	}{result1}
}

func (fake *FakeConfigGenerator) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.generateMutex.RLock()
	defer fake.generateMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeConfigGenerator) recordInvocation(key string, args []interface{}) {
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

var _ policies.ConfigGenerator = new(FakeConfigGenerator)
