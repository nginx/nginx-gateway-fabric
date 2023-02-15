// Code generated by counterfeiter. DO NOT EDIT.
package statefakes

import (
	"context"
	"sync"

	"github.com/nginxinc/nginx-kubernetes-gateway/internal/state"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/state/dataplane"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FakeChangeProcessor struct {
	CaptureDeleteChangeStub        func(client.Object, types.NamespacedName)
	captureDeleteChangeMutex       sync.RWMutex
	captureDeleteChangeArgsForCall []struct {
		arg1 client.Object
		arg2 types.NamespacedName
	}
	CaptureUpsertChangeStub        func(client.Object)
	captureUpsertChangeMutex       sync.RWMutex
	captureUpsertChangeArgsForCall []struct {
		arg1 client.Object
	}
	ProcessStub        func(context.Context) (bool, dataplane.Configuration, state.Statuses)
	processMutex       sync.RWMutex
	processArgsForCall []struct {
		arg1 context.Context
	}
	processReturns struct {
		result1 bool
		result2 dataplane.Configuration
		result3 state.Statuses
	}
	processReturnsOnCall map[int]struct {
		result1 bool
		result2 dataplane.Configuration
		result3 state.Statuses
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeChangeProcessor) CaptureDeleteChange(arg1 client.Object, arg2 types.NamespacedName) {
	fake.captureDeleteChangeMutex.Lock()
	fake.captureDeleteChangeArgsForCall = append(fake.captureDeleteChangeArgsForCall, struct {
		arg1 client.Object
		arg2 types.NamespacedName
	}{arg1, arg2})
	stub := fake.CaptureDeleteChangeStub
	fake.recordInvocation("CaptureDeleteChange", []interface{}{arg1, arg2})
	fake.captureDeleteChangeMutex.Unlock()
	if stub != nil {
		fake.CaptureDeleteChangeStub(arg1, arg2)
	}
}

func (fake *FakeChangeProcessor) CaptureDeleteChangeCallCount() int {
	fake.captureDeleteChangeMutex.RLock()
	defer fake.captureDeleteChangeMutex.RUnlock()
	return len(fake.captureDeleteChangeArgsForCall)
}

func (fake *FakeChangeProcessor) CaptureDeleteChangeCalls(stub func(client.Object, types.NamespacedName)) {
	fake.captureDeleteChangeMutex.Lock()
	defer fake.captureDeleteChangeMutex.Unlock()
	fake.CaptureDeleteChangeStub = stub
}

func (fake *FakeChangeProcessor) CaptureDeleteChangeArgsForCall(i int) (client.Object, types.NamespacedName) {
	fake.captureDeleteChangeMutex.RLock()
	defer fake.captureDeleteChangeMutex.RUnlock()
	argsForCall := fake.captureDeleteChangeArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeChangeProcessor) CaptureUpsertChange(arg1 client.Object) {
	fake.captureUpsertChangeMutex.Lock()
	fake.captureUpsertChangeArgsForCall = append(fake.captureUpsertChangeArgsForCall, struct {
		arg1 client.Object
	}{arg1})
	stub := fake.CaptureUpsertChangeStub
	fake.recordInvocation("CaptureUpsertChange", []interface{}{arg1})
	fake.captureUpsertChangeMutex.Unlock()
	if stub != nil {
		fake.CaptureUpsertChangeStub(arg1)
	}
}

func (fake *FakeChangeProcessor) CaptureUpsertChangeCallCount() int {
	fake.captureUpsertChangeMutex.RLock()
	defer fake.captureUpsertChangeMutex.RUnlock()
	return len(fake.captureUpsertChangeArgsForCall)
}

func (fake *FakeChangeProcessor) CaptureUpsertChangeCalls(stub func(client.Object)) {
	fake.captureUpsertChangeMutex.Lock()
	defer fake.captureUpsertChangeMutex.Unlock()
	fake.CaptureUpsertChangeStub = stub
}

func (fake *FakeChangeProcessor) CaptureUpsertChangeArgsForCall(i int) client.Object {
	fake.captureUpsertChangeMutex.RLock()
	defer fake.captureUpsertChangeMutex.RUnlock()
	argsForCall := fake.captureUpsertChangeArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeChangeProcessor) Process(arg1 context.Context) (bool, dataplane.Configuration, state.Statuses) {
	fake.processMutex.Lock()
	ret, specificReturn := fake.processReturnsOnCall[len(fake.processArgsForCall)]
	fake.processArgsForCall = append(fake.processArgsForCall, struct {
		arg1 context.Context
	}{arg1})
	stub := fake.ProcessStub
	fakeReturns := fake.processReturns
	fake.recordInvocation("Process", []interface{}{arg1})
	fake.processMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2, ret.result3
	}
	return fakeReturns.result1, fakeReturns.result2, fakeReturns.result3
}

func (fake *FakeChangeProcessor) ProcessCallCount() int {
	fake.processMutex.RLock()
	defer fake.processMutex.RUnlock()
	return len(fake.processArgsForCall)
}

func (fake *FakeChangeProcessor) ProcessCalls(stub func(context.Context) (bool, dataplane.Configuration, state.Statuses)) {
	fake.processMutex.Lock()
	defer fake.processMutex.Unlock()
	fake.ProcessStub = stub
}

func (fake *FakeChangeProcessor) ProcessArgsForCall(i int) context.Context {
	fake.processMutex.RLock()
	defer fake.processMutex.RUnlock()
	argsForCall := fake.processArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeChangeProcessor) ProcessReturns(result1 bool, result2 dataplane.Configuration, result3 state.Statuses) {
	fake.processMutex.Lock()
	defer fake.processMutex.Unlock()
	fake.ProcessStub = nil
	fake.processReturns = struct {
		result1 bool
		result2 dataplane.Configuration
		result3 state.Statuses
	}{result1, result2, result3}
}

func (fake *FakeChangeProcessor) ProcessReturnsOnCall(i int, result1 bool, result2 dataplane.Configuration, result3 state.Statuses) {
	fake.processMutex.Lock()
	defer fake.processMutex.Unlock()
	fake.ProcessStub = nil
	if fake.processReturnsOnCall == nil {
		fake.processReturnsOnCall = make(map[int]struct {
			result1 bool
			result2 dataplane.Configuration
			result3 state.Statuses
		})
	}
	fake.processReturnsOnCall[i] = struct {
		result1 bool
		result2 dataplane.Configuration
		result3 state.Statuses
	}{result1, result2, result3}
}

func (fake *FakeChangeProcessor) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.captureDeleteChangeMutex.RLock()
	defer fake.captureDeleteChangeMutex.RUnlock()
	fake.captureUpsertChangeMutex.RLock()
	defer fake.captureUpsertChangeMutex.RUnlock()
	fake.processMutex.RLock()
	defer fake.processMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeChangeProcessor) recordInvocation(key string, args []interface{}) {
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

var _ state.ChangeProcessor = new(FakeChangeProcessor)
