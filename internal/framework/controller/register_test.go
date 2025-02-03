package controller_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gcustom"
	gtypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller/controllerfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller/index"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller/predicate"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	ngftypes "github.com/nginx/nginx-gateway-fabric/internal/framework/types"
)

func TestRegister(t *testing.T) {
	t.Parallel()
	type fakes struct {
		mgr     *controllerfakes.FakeManager
		indexer *controllerfakes.FakeFieldIndexer
	}

	getDefaultFakes := func() fakes {
		scheme := runtime.NewScheme()
		utilruntime.Must(v1.Install(scheme))
		utilruntime.Must(v1beta1.Install(scheme))

		indexer := &controllerfakes.FakeFieldIndexer{}

		mgr := &controllerfakes.FakeManager{}
		mgr.GetClientReturns(fake.NewClientBuilder().Build())
		mgr.GetSchemeReturns(scheme)
		mgr.GetLoggerReturns(zap.New())
		mgr.GetFieldIndexerReturns(indexer)

		return fakes{
			mgr:     mgr,
			indexer: indexer,
		}
	}

	testError := errors.New("test error")

	objectTypeWithGVK := &v1.HTTPRoute{}
	objectTypeWithGVK.SetGroupVersionKind(
		schema.GroupVersionKind{Group: v1.GroupName, Version: "v1", Kind: kinds.HTTPRoute},
	)

	objectTypeNoGVK := &v1.HTTPRoute{}

	tests := []struct {
		fakes                   fakes
		objectType              ngftypes.ObjectType
		expectedErr             error
		msg                     string
		expectedMgrAddCallCount int
		expectPanic             bool
	}{
		{
			fakes:                   getDefaultFakes(),
			objectType:              objectTypeWithGVK,
			expectedErr:             nil,
			expectedMgrAddCallCount: 1,
			msg:                     "normal case",
		},
		{
			fakes: func(f fakes) fakes {
				f.indexer.IndexFieldReturns(testError)
				return f
			}(getDefaultFakes()),
			objectType:              objectTypeWithGVK,
			expectedErr:             testError,
			expectedMgrAddCallCount: 0,
			msg:                     "preparing index fails",
		},
		{
			fakes: func(f fakes) fakes {
				f.mgr.AddReturns(testError)
				return f
			}(getDefaultFakes()),
			objectType:              objectTypeWithGVK,
			expectedErr:             testError,
			expectedMgrAddCallCount: 1,
			msg:                     "building controller fails",
		},
		{
			fakes:                   getDefaultFakes(),
			objectType:              objectTypeNoGVK,
			expectPanic:             true,
			expectedMgrAddCallCount: 0,
			msg:                     "adding OnlyMetadata option panics",
		},
	}

	nsNameFilter := func(_ types.NamespacedName) (bool, string) {
		return true, ""
	}

	fieldIndexes := index.CreateEndpointSliceFieldIndices()

	eventCh := make(chan<- interface{})

	beSameFunctionPointer := func(expected interface{}) gtypes.GomegaMatcher {
		return gcustom.MakeMatcher(func(f interface{}) (bool, error) {
			// comparing functions is not allowed in Go, so we're comparing the pointers
			return reflect.ValueOf(expected).Pointer() == reflect.ValueOf(f).Pointer(), nil
		})
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			newReconciler := func(c controller.ReconcilerConfig) *controller.Reconciler {
				g.Expect(c.Getter).To(BeIdenticalTo(test.fakes.mgr.GetClient()))
				g.Expect(c.ObjectType).To(BeIdenticalTo(test.objectType))
				g.Expect(c.EventCh).To(BeIdenticalTo(eventCh))
				g.Expect(c.NamespacedNameFilter).Should(beSameFunctionPointer(nsNameFilter))

				return controller.NewReconciler(c)
			}

			register := func() error {
				return controller.Register(
					context.Background(),
					test.objectType,
					test.msg, // unique controller name for each loop iteration
					test.fakes.mgr,
					eventCh,
					controller.WithNamespacedNameFilter(nsNameFilter),
					controller.WithK8sPredicate(predicate.ServicePortsChangedPredicate{}),
					controller.WithFieldIndices(fieldIndexes),
					controller.WithNewReconciler(newReconciler),
					controller.WithOnlyMetadata(),
				)
			}

			if test.expectPanic {
				g.Expect(func() { _ = register() }).To(Panic())
			} else {
				err := register()
				if test.expectedErr == nil {
					g.Expect(err).ToNot(HaveOccurred())
				} else {
					g.Expect(err).To(MatchError(test.expectedErr))
				}
			}

			indexCallCount := test.fakes.indexer.IndexFieldCallCount()

			g.Expect(indexCallCount).To(Equal(1))

			_, objType, field, indexFunc := test.fakes.indexer.IndexFieldArgsForCall(0)

			g.Expect(objType).To(BeIdenticalTo(test.objectType))
			g.Expect(field).To(BeIdenticalTo(index.KubernetesServiceNameIndexField))

			expectedIndexFunc := fieldIndexes[index.KubernetesServiceNameIndexField]
			g.Expect(indexFunc).To(beSameFunctionPointer(expectedIndexFunc))

			addCallCount := test.fakes.mgr.AddCallCount()
			g.Expect(addCallCount).To(Equal(test.expectedMgrAddCallCount))
		})
	}
}
