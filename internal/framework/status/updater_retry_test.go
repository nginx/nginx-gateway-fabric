package status_test

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller/controllerfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/status"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/status/statusfakes"
)

func TestNewRetryUpdateFunc(t *testing.T) {
	t.Parallel()
	tests := []struct {
		getReturns          error
		updateReturns       error
		name                string
		expUpdateCallCount  int
		statusSetterReturns bool
		expConditionPassed  bool
	}{
		{
			getReturns:          errors.New("failed to get resource"),
			updateReturns:       nil,
			statusSetterReturns: true,
			expUpdateCallCount:  0,
			name:                "get fails",
			expConditionPassed:  false,
		},
		{
			getReturns:          apierrors.NewNotFound(schema.GroupResource{}, "not found"),
			updateReturns:       nil,
			statusSetterReturns: true,
			expUpdateCallCount:  0,
			name:                "get fails and apierrors is not found",
			expConditionPassed:  true,
		},
		{
			getReturns:          nil,
			updateReturns:       errors.New("failed to update resource"),
			statusSetterReturns: true,
			expUpdateCallCount:  1,
			name:                "update fails",
			expConditionPassed:  false,
		},
		{
			getReturns:          nil,
			updateReturns:       nil,
			statusSetterReturns: false,
			expUpdateCallCount:  0,
			name:                "status not set",
			expConditionPassed:  true,
		},
		{
			getReturns:          nil,
			updateReturns:       nil,
			statusSetterReturns: true,
			expUpdateCallCount:  1,
			name:                "nothing fails",
			expConditionPassed:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			fakeStatusUpdater := &statusfakes.FakeK8sUpdater{}
			fakeGetter := &controllerfakes.FakeGetter{}

			fakeStatusUpdater.UpdateReturns(test.updateReturns)
			fakeGetter.GetReturns(test.getReturns)

			f := status.NewRetryUpdateFunc(
				fakeGetter,
				fakeStatusUpdater,
				types.NamespacedName{},
				&v1.GatewayClass{},
				zap.New(),
				func(client.Object) bool { return test.statusSetterReturns },
			)

			conditionPassed, err := f(context.Background())

			// The function should always return nil.
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(conditionPassed).To(Equal(test.expConditionPassed))
			g.Expect(fakeStatusUpdater.UpdateCallCount()).To(Equal(test.expUpdateCallCount))
		})
	}
}
