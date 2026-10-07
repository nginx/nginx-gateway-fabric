package framework

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// WaitForGatewayPolicyAffected polls until the Gateway has the given policy-affected
// condition set to True. condType is the condition type string for the specific policy.
func (rm *ResourceManager) WaitForGatewayPolicyAffected(
	nsName types.NamespacedName,
	condType string,
	timeout time.Duration,
) error {
	GinkgoWriter.Printf("Waiting for Gateway %q to have condition %q=True\n", nsName, condType)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return wait.PollUntilContextCancel(ctx, 500*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		var gw gatewayv1.Gateway
		if err := rm.Get(ctx, nsName, &gw); err != nil {
			return false, err
		}
		for _, cond := range gw.Status.Conditions {
			if cond.Type == condType && cond.Status == metav1.ConditionTrue {
				return true, nil
			}
		}
		GinkgoWriter.Printf("Gateway %q does not have condition %q=True yet\n", nsName, condType)
		return false, nil
	})
}

// WaitForHTTPRoutePolicyAffected polls until the HTTPRoute has the given policy-affected
// condition set to True across any of its parent statuses.
func (rm *ResourceManager) WaitForHTTPRoutePolicyAffected(
	nsName types.NamespacedName,
	condType string,
	timeout time.Duration,
) error {
	GinkgoWriter.Printf("Waiting for HTTPRoute %q to have condition %q=True\n", nsName, condType)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return wait.PollUntilContextCancel(ctx, 500*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		var route gatewayv1.HTTPRoute
		if err := rm.Get(ctx, nsName, &route); err != nil {
			return false, err
		}
		for _, parent := range route.Status.Parents {
			for _, cond := range parent.Conditions {
				if cond.Type == condType && cond.Status == metav1.ConditionTrue {
					return true, nil
				}
			}
		}
		GinkgoWriter.Printf("HTTPRoute %q does not have condition %q=True yet\n", nsName, condType)
		return false, nil
	})
}

// WaitForGRPCRoutePolicyAffected polls until the GRPCRoute has the given policy-affected
// condition set to True across any of its parent statuses.
func (rm *ResourceManager) WaitForGRPCRoutePolicyAffected(
	nsName types.NamespacedName,
	condType string,
	timeout time.Duration,
) error {
	GinkgoWriter.Printf("Waiting for GRPCRoute %q to have condition %q=True\n", nsName, condType)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return wait.PollUntilContextCancel(ctx, 500*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		var route gatewayv1.GRPCRoute
		if err := rm.Get(ctx, nsName, &route); err != nil {
			return false, err
		}
		for _, parent := range route.Status.Parents {
			for _, cond := range parent.Conditions {
				if cond.Type == condType && cond.Status == metav1.ConditionTrue {
					return true, nil
				}
			}
		}
		GinkgoWriter.Printf("GRPCRoute %q does not have condition %q=True yet\n", nsName, condType)
		return false, nil
	})
}

// WaitForHTTPRoutePolicyAffectedGone polls until the given policy-affected condition is
// no longer present on the HTTPRoute, indicating the last affecting policy was removed.
func (rm *ResourceManager) WaitForHTTPRoutePolicyAffectedGone(
	nsName types.NamespacedName,
	condType string,
	timeout time.Duration,
) error {
	GinkgoWriter.Printf("Waiting for condition %q to be removed from HTTPRoute %q\n", condType, nsName)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return wait.PollUntilContextCancel(ctx, 500*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		var route gatewayv1.HTTPRoute
		if err := rm.Get(ctx, nsName, &route); err != nil {
			return false, err
		}
		for _, parent := range route.Status.Parents {
			for _, cond := range parent.Conditions {
				if cond.Type == condType {
					GinkgoWriter.Printf("HTTPRoute %q still has condition %q\n", nsName, condType)
					return false, nil
				}
			}
		}
		return true, nil
	})
}
