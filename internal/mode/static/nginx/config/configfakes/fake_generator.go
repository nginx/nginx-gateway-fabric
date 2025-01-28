// Code generated by counterfeiter. DO NOT EDIT.
package configfakes

import (
	"sync"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

type FakeGenerator struct {
	GenerateStub        func(dataplane.Configuration) []agent.File
	generateMutex       sync.RWMutex
	generateArgsForCall []struct {
		arg1 dataplane.Configuration
	}
	generateReturns struct {
		result1 []agent.File
	}
	generateReturnsOnCall map[int]struct {
		result1 []agent.File
	}
	GenerateDeploymentContextStub        func(dataplane.DeploymentContext) (agent.File, error)
	generateDeploymentContextMutex       sync.RWMutex
	generateDeploymentContextArgsForCall []struct {
		arg1 dataplane.DeploymentContext
	}
	generateDeploymentContextReturns struct {
		result1 agent.File
		result2 error
	}
	generateDeploymentContextReturnsOnCall map[int]struct {
		result1 agent.File
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeGenerator) Generate(arg1 dataplane.Configuration) []agent.File {
	fake.generateMutex.Lock()
	ret, specificReturn := fake.generateReturnsOnCall[len(fake.generateArgsForCall)]
	fake.generateArgsForCall = append(fake.generateArgsForCall, struct {
		arg1 dataplane.Configuration
	}{arg1})
	stub := fake.GenerateStub
	fakeReturns := fake.generateReturns
	fake.recordInvocation("Generate", []interface{}{arg1})
	fake.generateMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeGenerator) GenerateCallCount() int {
	fake.generateMutex.RLock()
	defer fake.generateMutex.RUnlock()
	return len(fake.generateArgsForCall)
}

func (fake *FakeGenerator) GenerateCalls(stub func(dataplane.Configuration) []agent.File) {
	fake.generateMutex.Lock()
	defer fake.generateMutex.Unlock()
	fake.GenerateStub = stub
}

func (fake *FakeGenerator) GenerateArgsForCall(i int) dataplane.Configuration {
	fake.generateMutex.RLock()
	defer fake.generateMutex.RUnlock()
	argsForCall := fake.generateArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeGenerator) GenerateReturns(result1 []agent.File) {
	fake.generateMutex.Lock()
	defer fake.generateMutex.Unlock()
	fake.GenerateStub = nil
	fake.generateReturns = struct {
		result1 []agent.File
	}{result1}
}

func (fake *FakeGenerator) GenerateReturnsOnCall(i int, result1 []agent.File) {
	fake.generateMutex.Lock()
	defer fake.generateMutex.Unlock()
	fake.GenerateStub = nil
	if fake.generateReturnsOnCall == nil {
		fake.generateReturnsOnCall = make(map[int]struct {
			result1 []agent.File
		})
	}
	fake.generateReturnsOnCall[i] = struct {
		result1 []agent.File
	}{result1}
}

func (fake *FakeGenerator) GenerateDeploymentContext(arg1 dataplane.DeploymentContext) (agent.File, error) {
	fake.generateDeploymentContextMutex.Lock()
	ret, specificReturn := fake.generateDeploymentContextReturnsOnCall[len(fake.generateDeploymentContextArgsForCall)]
	fake.generateDeploymentContextArgsForCall = append(fake.generateDeploymentContextArgsForCall, struct {
		arg1 dataplane.DeploymentContext
	}{arg1})
	stub := fake.GenerateDeploymentContextStub
	fakeReturns := fake.generateDeploymentContextReturns
	fake.recordInvocation("GenerateDeploymentContext", []interface{}{arg1})
	fake.generateDeploymentContextMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeGenerator) GenerateDeploymentContextCallCount() int {
	fake.generateDeploymentContextMutex.RLock()
	defer fake.generateDeploymentContextMutex.RUnlock()
	return len(fake.generateDeploymentContextArgsForCall)
}

func (fake *FakeGenerator) GenerateDeploymentContextCalls(stub func(dataplane.DeploymentContext) (agent.File, error)) {
	fake.generateDeploymentContextMutex.Lock()
	defer fake.generateDeploymentContextMutex.Unlock()
	fake.GenerateDeploymentContextStub = stub
}

func (fake *FakeGenerator) GenerateDeploymentContextArgsForCall(i int) dataplane.DeploymentContext {
	fake.generateDeploymentContextMutex.RLock()
	defer fake.generateDeploymentContextMutex.RUnlock()
	argsForCall := fake.generateDeploymentContextArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeGenerator) GenerateDeploymentContextReturns(result1 agent.File, result2 error) {
	fake.generateDeploymentContextMutex.Lock()
	defer fake.generateDeploymentContextMutex.Unlock()
	fake.GenerateDeploymentContextStub = nil
	fake.generateDeploymentContextReturns = struct {
		result1 agent.File
		result2 error
	}{result1, result2}
}

func (fake *FakeGenerator) GenerateDeploymentContextReturnsOnCall(i int, result1 agent.File, result2 error) {
	fake.generateDeploymentContextMutex.Lock()
	defer fake.generateDeploymentContextMutex.Unlock()
	fake.GenerateDeploymentContextStub = nil
	if fake.generateDeploymentContextReturnsOnCall == nil {
		fake.generateDeploymentContextReturnsOnCall = make(map[int]struct {
			result1 agent.File
			result2 error
		})
	}
	fake.generateDeploymentContextReturnsOnCall[i] = struct {
		result1 agent.File
		result2 error
	}{result1, result2}
}

func (fake *FakeGenerator) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.generateMutex.RLock()
	defer fake.generateMutex.RUnlock()
	fake.generateDeploymentContextMutex.RLock()
	defer fake.generateDeploymentContextMutex.RUnlock()
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

var _ config.Generator = new(FakeGenerator)
