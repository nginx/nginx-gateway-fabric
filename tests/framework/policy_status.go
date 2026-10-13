package framework

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
)

// WaitForPolicyToBeAccepted polls until all ancestors in the policy status show
// Accepted=True/Accepted.
func (rm *ResourceManager) WaitForPolicyToBeAccepted(
	nsName types.NamespacedName,
	timeout time.Duration,
	getAncestors func(ctx context.Context) ([]gatewayv1.PolicyAncestorStatus, error),
) error {
	GinkgoWriter.Printf("Waiting for policy %q to be accepted\n", nsName)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return wait.PollUntilContextCancel(ctx, 500*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		ancestors, err := getAncestors(ctx)
		if err != nil {
			return false, err
		}

		if len(ancestors) == 0 {
			GinkgoWriter.Printf("Policy %q has no ancestor status yet\n", nsName)
			return false, nil
		}

		for _, ancestor := range ancestors {
			if err := policyAncestorAccepted(ancestor); err != nil {
				GinkgoWriter.Printf("Policy %q is not accepted yet: %v\n", nsName, err)
				return false, nil
			}
		}

		return true, nil
	})
}

// policyAncestorAccepted returns nil if the ancestor has Accepted=True/Accepted.
func policyAncestorAccepted(ancestor gatewayv1.PolicyAncestorStatus) error {
	for _, cond := range ancestor.Conditions {
		if cond.Type != string(gatewayv1.PolicyConditionAccepted) {
			continue
		}
		if cond.Status != metav1.ConditionTrue {
			return fmt.Errorf("expected Accepted=True, got %s", cond.Status)
		}
		if cond.Reason != string(gatewayv1.PolicyReasonAccepted) {
			return fmt.Errorf("expected reason %s, got %s", gatewayv1.PolicyReasonAccepted, cond.Reason)
		}
		return nil
	}
	return fmt.Errorf("no Accepted condition found in ancestor conditions")
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

// AncestorStatusMustHaveAcceptedCondition checks that the given ancestor has an Accepted
// condition with the expected status and reason.
func AncestorStatusMustHaveAcceptedCondition(
	status gatewayv1.PolicyAncestorStatus,
	condStatus metav1.ConditionStatus,
	condReason gatewayv1.PolicyConditionReason,
) error {
	GinkgoWriter.Printf("Checking if ancestor status has accepted condition\n")
	for _, cond := range status.Conditions {
		if cond.Type != string(gatewayv1.PolicyConditionAccepted) {
			continue
		}
		if cond.Status != condStatus {
			wrongStatusErr := fmt.Errorf("expected condition status to be %s, got %s", condStatus, cond.Status)
			GinkgoWriter.Printf("ERROR: %v\n", wrongStatusErr)
			return wrongStatusErr
		}
		if cond.Reason != string(condReason) {
			wrongReasonErr := fmt.Errorf("expected condition reason to be %s, got %s", condReason, cond.Reason)
			GinkgoWriter.Printf("ERROR: %v\n", wrongReasonErr)
			return wrongReasonErr
		}
		return nil
	}
	missingAcceptedErr := fmt.Errorf("status has no Accepted condition, got %d conditions", len(status.Conditions))
	GinkgoWriter.Printf("ERROR: %v\n", missingAcceptedErr)
	return missingAcceptedErr
}

// AncestorMustEqualTargetRef verifies that the ancestor's ref matches the given LocalPolicyTargetReference.
func AncestorMustEqualTargetRef(
	ancestor gatewayv1.PolicyAncestorStatus,
	targetRef gatewayv1.LocalPolicyTargetReference,
	namespace string,
) error {
	if ancestor.ControllerName != NgfControllerName {
		err := fmt.Errorf("expected ancestor controller name to be %s, got %s", NgfControllerName, ancestor.ControllerName)
		GinkgoWriter.Printf("ERROR: %v\n", err)
		return err
	}
	if ancestor.AncestorRef.Namespace == nil {
		err := fmt.Errorf("expected ancestor namespace to be %s, got nil", namespace)
		GinkgoWriter.Printf("ERROR: %v\n", err)
		return err
	}
	if string(*ancestor.AncestorRef.Namespace) != namespace {
		err := fmt.Errorf("expected ancestor namespace to be %s, got %s", namespace, string(*ancestor.AncestorRef.Namespace))
		GinkgoWriter.Printf("ERROR: %v\n", err)
		return err
	}
	if ancestor.AncestorRef.Name != targetRef.Name {
		err := fmt.Errorf("expected ancestorRef to have name %s, got %s", targetRef.Name, ancestor.AncestorRef.Name)
		GinkgoWriter.Printf("ERROR: %v\n", err)
		return err
	}
	if ancestor.AncestorRef.Group == nil {
		err := fmt.Errorf("expected ancestorRef to have group %s, got nil", targetRef.Group)
		GinkgoWriter.Printf("ERROR: %v\n", err)
		return err
	}
	if *ancestor.AncestorRef.Group != targetRef.Group {
		err := fmt.Errorf("expected ancestorRef to have group %s, got %s",
			targetRef.Group, string(*ancestor.AncestorRef.Group))
		GinkgoWriter.Printf("ERROR: %v\n", err)
		return err
	}
	if ancestor.AncestorRef.Kind == nil {
		err := fmt.Errorf("expected ancestorRef to have kind %s, got nil", targetRef.Kind)
		GinkgoWriter.Printf("ERROR: %v\n", err)
		return err
	}
	if *ancestor.AncestorRef.Kind != targetRef.Kind {
		err := fmt.Errorf("expected ancestorRef to have kind %s, got %s", targetRef.Kind, string(*ancestor.AncestorRef.Kind))
		GinkgoWriter.Printf("ERROR: %v\n", err)
		return err
	}
	GinkgoWriter.Printf("Success: AncestorRef %q matches TargetRef\n", ancestor.AncestorRef.Name)
	return nil
}

// AncestorMustEqualGatewayRef verifies that the ancestor's ref matches the given Gateway name.
func AncestorMustEqualGatewayRef(
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
		return fmt.Errorf("expected ancestorRef to have kind Gateway, got nil")
	}
	if *ancestor.AncestorRef.Kind != gatewayv1.Kind("Gateway") {
		return fmt.Errorf("expected ancestorRef to have kind %s, got %s", "Gateway", string(*ancestor.AncestorRef.Kind))
	}
	return nil
}

// FindTargetRefForAncestor finds the LocalPolicyTargetReference in the list that matches the given ancestor.
func FindTargetRefForAncestor(
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
