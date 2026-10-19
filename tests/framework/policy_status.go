package framework

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
)

// Policy is any NGF policy that reports Gateway API ancestor status.
type Policy interface {
	client.Object
	GetTargetRefs() []gatewayv1.LocalPolicyTargetReference
	GetPolicyStatus() gatewayv1.PolicyStatus
}

// PolicyCondition is the Accepted condition a policy is expected to reach on every ancestor.
type PolicyCondition struct {
	Status metav1.ConditionStatus
	Reason gatewayv1.PolicyConditionReason
}

// PolicyAccepted is the Accepted=True/Accepted condition.
var PolicyAccepted = PolicyCondition{Status: metav1.ConditionTrue, Reason: gatewayv1.PolicyReasonAccepted}

// AncestorValidator checks that a policy's ancestors refer to the expected objects.
type AncestorValidator func(policy Policy) error

// AncestorsMatchTargetRefs expects one ancestor per targetRef, as for policies targeting Gateways or Routes.
func AncestorsMatchTargetRefs(policy Policy) error {
	ancestors := policy.GetPolicyStatus().Ancestors
	targetRefs := policy.GetTargetRefs()
	if len(ancestors) != len(targetRefs) {
		return fmt.Errorf("policy has %d ancestors, expected %d", len(ancestors), len(targetRefs))
	}

	for _, ancestor := range ancestors {
		targetRef, ok := findTargetRefForAncestor(ancestor, targetRefs)
		if !ok {
			return fmt.Errorf("no targetRef matches ancestor %q", ancestor.AncestorRef.Name)
		}
		if err := ancestorMustEqualTargetRef(ancestor, targetRef, policy.GetNamespace()); err != nil {
			return err
		}
	}

	return nil
}

// AncestorIsGateway expects a single Gateway ancestor, as for policies targeting Services.
func AncestorIsGateway(gatewayName string) AncestorValidator {
	return func(policy Policy) error {
		ancestors := policy.GetPolicyStatus().Ancestors
		if len(ancestors) != 1 {
			return fmt.Errorf("policy has %d ancestors, expected 1", len(ancestors))
		}

		return ancestorMustEqualGatewayRef(ancestors[0], gatewayName, policy.GetNamespace())
	}
}

// WaitForPolicyStatus polls until every ancestor of the policy has the expected Accepted condition
// and the ancestors pass validate. policy is the empty object the status is read into.
func (rm *ResourceManager) WaitForPolicyStatus(
	nsName types.NamespacedName,
	policy Policy,
	expected PolicyCondition,
	validate AncestorValidator,
) error {
	GinkgoWriter.Printf(
		"Waiting for policy %q to have condition Accepted/%s/%s\n",
		nsName,
		expected.Status,
		expected.Reason,
	)

	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.GetStatusTimeout)
	defer cancel()

	var lastErr error
	err := wait.PollUntilContextCancel(ctx, 500*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		if err := rm.Get(ctx, nsName, policy); err != nil {
			lastErr = err
			return false, nil //nolint:nilerr // keep polling; reported on timeout
		}

		lastErr = checkPolicyStatus(policy, expected, validate)
		return lastErr == nil, nil
	})
	if err != nil && lastErr != nil {
		return fmt.Errorf("policy %q: %w (last status error: %w)", nsName, err, lastErr)
	}

	return err
}

// checkPolicyStatus verifies that the policy has ancestor statuses and that each ancestor has the
// expected Accepted condition. It also runs the provided AncestorValidator on the policy.
func checkPolicyStatus(policy Policy, expected PolicyCondition, validate AncestorValidator) error {
	ancestors := policy.GetPolicyStatus().Ancestors
	if len(ancestors) == 0 {
		return errors.New("policy has no ancestor status yet")
	}

	if err := validate(policy); err != nil {
		return err
	}

	for _, ancestor := range ancestors {
		if err := ancestorStatusMustHaveAcceptedCondition(ancestor, expected.Status, expected.Reason); err != nil {
			return fmt.Errorf("ancestor %q: %w", ancestor.AncestorRef.Name, err)
		}
	}

	return nil
}

// WaitForGatewayPolicyAffected polls until the Gateway has the given policy-affected
// condition set to True.
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

// WaitForAccessPolicyPartiallyProgrammed polls until the AccessPolicy reports
// Programmed=True with reason PartiallyProgrammed on at least one ancestor.
func (rm *ResourceManager) WaitForAccessPolicyPartiallyProgrammed(
	nsName types.NamespacedName, timeout time.Duration,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return wait.PollUntilContextCancel(ctx, 500*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		var ap ngfAPI.AccessPolicy
		if err := rm.Get(ctx, nsName, &ap); err != nil {
			return false, err
		}
		for _, ancestor := range ap.Status.Ancestors {
			for _, cond := range ancestor.Conditions {
				if cond.Type == string(conditions.PolicyConditionProgrammed) &&
					cond.Status == metav1.ConditionTrue &&
					cond.Reason == string(conditions.PolicyReasonPartiallyProgrammed) {
					return true, nil
				}
			}
		}
		return false, nil
	})
}

// WaitForAccessPolicyNotProgrammed polls until the AccessPolicy reports
// Programmed=False with reason Overridden on at least one ancestor.
// This is set when the route Allow has no overlap with the gateway's permitted range.
func (rm *ResourceManager) WaitForAccessPolicyNotProgrammed(nsName types.NamespacedName, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return wait.PollUntilContextCancel(ctx, 500*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		var ap ngfAPI.AccessPolicy
		if err := rm.Get(ctx, nsName, &ap); err != nil {
			return false, err
		}
		for _, ancestor := range ap.Status.Ancestors {
			for _, cond := range ancestor.Conditions {
				if cond.Type == string(conditions.PolicyConditionProgrammed) &&
					cond.Status == metav1.ConditionFalse &&
					cond.Reason == string(conditions.PolicyReasonOverridden) {
					return true, nil
				}
			}
		}
		return false, nil
	})
}

// ancestorStatusMustHaveAcceptedCondition checks that the given ancestor has an Accepted
// condition with the expected status and reason.
func ancestorStatusMustHaveAcceptedCondition(
	status gatewayv1.PolicyAncestorStatus,
	condStatus metav1.ConditionStatus,
	condReason gatewayv1.PolicyConditionReason,
) error {
	for _, cond := range status.Conditions {
		if cond.Type != string(gatewayv1.PolicyConditionAccepted) {
			continue
		}
		if cond.Status != condStatus {
			return fmt.Errorf("expected condition status to be %s, got %s", condStatus, cond.Status)
		}
		if cond.Reason != string(condReason) {
			return fmt.Errorf("expected condition reason to be %s, got %s", condReason, cond.Reason)
		}
		return nil
	}

	return fmt.Errorf("status has no Accepted condition, got %d conditions", len(status.Conditions))
}

// ancestorMustEqualTargetRef verifies that the ancestor's ref matches the given LocalPolicyTargetReference.
func ancestorMustEqualTargetRef(
	ancestor gatewayv1.PolicyAncestorStatus,
	targetRef gatewayv1.LocalPolicyTargetReference,
	namespace string,
) error {
	ref := ancestor.AncestorRef
	if ancestor.ControllerName != NgfControllerName {
		return fmt.Errorf("expected ancestor controller name to be %s, got %s", NgfControllerName, ancestor.ControllerName)
	}
	if ref.Namespace == nil {
		return fmt.Errorf("expected ancestor namespace to be %s, got nil", namespace)
	}
	if string(*ref.Namespace) != namespace {
		return fmt.Errorf("expected ancestor namespace to be %s, got %s", namespace, *ref.Namespace)
	}
	if ref.Name != targetRef.Name {
		return fmt.Errorf("expected ancestorRef to have name %s, got %s", targetRef.Name, ref.Name)
	}
	if ref.Group == nil {
		return fmt.Errorf("expected ancestorRef to have group %s, got nil", targetRef.Group)
	}
	if *ref.Group != targetRef.Group {
		return fmt.Errorf("expected ancestorRef to have group %s, got %s", targetRef.Group, *ref.Group)
	}
	if ref.Kind == nil {
		return fmt.Errorf("expected ancestorRef to have kind %s, got nil", targetRef.Kind)
	}
	if *ref.Kind != targetRef.Kind {
		return fmt.Errorf("expected ancestorRef to have kind %s, got %s", targetRef.Kind, *ref.Kind)
	}

	return nil
}

// ancestorMustEqualGatewayRef verifies that the ancestor's ref matches the given Gateway name.
func ancestorMustEqualGatewayRef(
	ancestor gatewayv1.PolicyAncestorStatus,
	gatewayName string,
	namespace string,
) error {
	if ancestor.ControllerName != NgfControllerName {
		return fmt.Errorf("expected ancestor controller name to be %s, got %s", NgfControllerName, ancestor.ControllerName)
	}
	if ancestor.AncestorRef.Namespace == nil {
		return fmt.Errorf("expected ancestor namespace to be %s, got nil", namespace)
	}
	if string(*ancestor.AncestorRef.Namespace) != namespace {
		return fmt.Errorf("expected ancestor namespace to be %s, got %s", namespace, string(*ancestor.AncestorRef.Namespace))
	}
	if string(ancestor.AncestorRef.Name) != gatewayName {
		return fmt.Errorf("expected ancestorRef to have name %s, got %s", gatewayName, ancestor.AncestorRef.Name)
	}
	if ancestor.AncestorRef.Kind == nil {
		return errors.New("expected ancestorRef to have kind Gateway, got nil")
	}
	if *ancestor.AncestorRef.Kind != gatewayv1.Kind("Gateway") {
		return fmt.Errorf("expected ancestorRef to have kind %s, got %s", "Gateway", string(*ancestor.AncestorRef.Kind))
	}
	return nil
}

// findTargetRefForAncestor finds the LocalPolicyTargetReference in the list that matches the given ancestor.
func findTargetRefForAncestor(
	ancestor gatewayv1.PolicyAncestorStatus,
	targets []gatewayv1.LocalPolicyTargetReference,
) (gatewayv1.LocalPolicyTargetReference, bool) {
	if ancestor.AncestorRef.Group == nil || ancestor.AncestorRef.Kind == nil {
		return gatewayv1.LocalPolicyTargetReference{}, false
	}
	for _, tr := range targets {
		if tr.Name == ancestor.AncestorRef.Name &&
			tr.Kind == *ancestor.AncestorRef.Kind &&
			tr.Group == *ancestor.AncestorRef.Group {
			return tr, true
		}
	}
	return gatewayv1.LocalPolicyTargetReference{}, false
}
