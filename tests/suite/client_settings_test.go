package suite

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPI "github.com/nginxinc/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginxinc/nginx-gateway-fabric/tests/framework"
)

var _ = Describe("ClientSettingsPolicy", Ordered, Label("functional", "cspolicy"), func() {
	var (
		files = []string{
			"clientsettings/cafe.yaml",
			"clientsettings/gateway.yaml",
			"clientsettings/cafe-routes.yaml",
			"clientsettings/grpc-route.yaml",
			"clientsettings/grpc-backend.yaml",
		}

		namespace = "clientsettings"
	)

	BeforeAll(func() {
		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
	})

	AfterAll(func() {
		Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
		Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
	})

	When("valid ClientSettingsPolicies are created", func() {
		var (
			policies = []string{
				"clientsettings/valid-csps.yaml",
			}

			baseURL string
		)

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policies, namespace)).To(Succeed())

			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}

			baseURL = fmt.Sprintf("http://cafe.example.com:%d", port)
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
		})

		Specify("they are accepted by the target resource", func() {
			policyNames := []string{
				"gw-csp",
				"coffee-route-csp",
				"tea-route-csp",
				"soda-route-csp",
				"grpc-route-csp",
			}

			for _, name := range policyNames {
				nsname := types.NamespacedName{Name: name, Namespace: namespace}

				err := waitForCSPolicyToBeAccepted(nsname)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", name))
			}
		})

		// We only test that the client_max_body_size directive in this test is propagated correctly.
		// This is because we can easily verify this directive by sending requests with different sized payloads.
		DescribeTable("the settings are propagated to the nginx config",
			func(uri string, byteLengthOfRequestBody, expStatus int) {
				url := baseURL + uri

				payload := make([]byte, byteLengthOfRequestBody)
				_, err := rand.Read(payload)
				Expect(err).ToNot(HaveOccurred())

				resp, err := framework.Post(url, address, bytes.NewReader(payload), timeoutConfig.RequestTimeout)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp).To(HaveHTTPStatus(expStatus))

				if expStatus == http.StatusOK {
					Expect(resp).To(HaveHTTPBody(ContainSubstring(fmt.Sprintf("URI: %s", uri))))
				}
			},
			func(uri string, byteLengthOfRequestBody, expStatus int) string {
				return fmt.Sprintf(
					"request body of %d should return %d for %s",
					byteLengthOfRequestBody,
					expStatus,
					uri,
				)
			},
			Entry(nil, "/tea", 900, http.StatusOK),
			Entry(nil, "/tea", 1200, http.StatusRequestEntityTooLarge),
			Entry(nil, "/coffee", 1200, http.StatusOK),
			Entry(nil, "/coffee", 2500, http.StatusRequestEntityTooLarge),
			Entry(nil, "/soda", 2500, http.StatusOK),
			Entry(nil, "/soda", 3300, http.StatusRequestEntityTooLarge),
		)
	})

	When("a ClientSettingsPolicy targets an invalid resources", func() {
		Specify("their accepted condition is set to TargetNotFound", func() {
			files := []string{
				"clientsettings/ignored-gateway.yaml",
				"clientsettings/invalid-csp.yaml",
			}

			Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())

			nsname := types.NamespacedName{Name: "invalid-csp", Namespace: namespace}
			Expect(waitForCSPolicyToHaveTargetNotFoundAcceptedCond(nsname)).To(Succeed())

			Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
		})
	})

	Context("Merging behavior", func() {
		When("multiple policies target the same resource", func() {
			Specify("policies that cannot be merged are marked as conflicted", func() {
				policies := []string{
					"clientsettings/merging-csps.yaml",
				}

				mergeablePolicyNames := []string{
					"hr-merge-1",
					"hr-merge-2",
					"hr-merge-3",
					"grpc-merge-1",
					"grpc-merge-2",
				}

				conflictedPolicyNames := []string{
					"z-hr-conflict-1",
					"z-hr-conflict-2",
					"z-grpc-conflict",
				}

				Expect(resourceManager.ApplyFromFiles(policies, namespace)).To(Succeed())

				for _, name := range conflictedPolicyNames {
					nsname := types.NamespacedName{Name: name, Namespace: namespace}

					err := waitForCSPolicyToBeConflicted(nsname)
					Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not marked as conflicted", name))
				}

				for _, name := range mergeablePolicyNames {
					nsname := types.NamespacedName{Name: name, Namespace: namespace}

					err := waitForCSPolicyToBeAccepted(nsname)
					Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not marked as conflicted", name))
				}

				Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
			})
		})
	})
})

func waitForCSPolicyToBeAccepted(policyNsname types.NamespacedName) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout)
	defer cancel()

	GinkgoWriter.Printf(
		"Waiting for ClientSettingsPolicy %q to have the condition Accepted/True/Accepted\n",
		policyNsname,
	)

	return waitForClientSettingsAncestorStatus(ctx, policyNsname, metav1.ConditionTrue, v1alpha2.PolicyReasonAccepted)
}

func waitForCSPolicyToBeConflicted(policyNsname types.NamespacedName) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout)
	defer cancel()

	GinkgoWriter.Printf(
		"Waiting for ClientSettingsPolicy %q to have the condition Accepted/False/Conflicted\n",
		policyNsname,
	)

	return waitForClientSettingsAncestorStatus(
		ctx,
		policyNsname,
		metav1.ConditionFalse,
		v1alpha2.PolicyReasonConflicted,
	)
}

func waitForCSPolicyToHaveTargetNotFoundAcceptedCond(policyNsname types.NamespacedName) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout)
	defer cancel()

	GinkgoWriter.Printf(
		"Waiting for ClientSettingsPolicy %q to have the condition Accepted/False/TargetNotFound\n",
		policyNsname,
	)

	return waitForClientSettingsAncestorStatus(
		ctx,
		policyNsname,
		metav1.ConditionFalse,
		v1alpha2.PolicyReasonTargetNotFound,
	)
}

func waitForClientSettingsAncestorStatus(
	ctx context.Context,
	policyNsname types.NamespacedName,
	condStatus metav1.ConditionStatus,
	condReason v1alpha2.PolicyConditionReason,
) error {
	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {

			var pol ngfAPI.ClientSettingsPolicy

			if err := k8sClient.Get(ctx, policyNsname, &pol); err != nil {
				return false, err
			}

			if len(pol.Status.Ancestors) == 0 {
				GinkgoWriter.Printf("ClientSettingsPolicy %q does not have an ancestor status yet\n", policyNsname)

				return false, nil
			}

			if len(pol.Status.Ancestors) != 1 {
				return false, fmt.Errorf("policy has %d ancestors, expected 1", len(pol.Status.Ancestors))
			}

			ancestor := pol.Status.Ancestors[0]

			if err := ancestorMustEqualTargetRef(ancestor, pol.GetTargetRef(), policyNsname.Namespace); err != nil {
				return false, err
			}

			err := ancestorStatusMustHaveAcceptedCondition(ancestor, condStatus, condReason)

			return err == nil, err
		},
	)
}

func ancestorStatusMustHaveAcceptedCondition(
	status v1alpha2.PolicyAncestorStatus,
	condStatus metav1.ConditionStatus,
	condReason v1alpha2.PolicyConditionReason,
) error {
	if len(status.Conditions) != 1 {
		return fmt.Errorf("expected 1 condition in status, got %d", len(status.Conditions))
	}

	if status.Conditions[0].Type != "Accepted" {
		return fmt.Errorf("expected condition type to be Accepted, got %s", status.Conditions[0].Type)
	}

	if status.Conditions[0].Status != condStatus {
		return fmt.Errorf("expected condition status to be %s, got %s", condStatus, status.Conditions[0].Status)
	}

	if status.Conditions[0].Reason != string(condReason) {
		return fmt.Errorf("expected condition status to be %s, got %s", condReason, status.Conditions[0].Reason)
	}

	return nil
}

func ancestorMustEqualTargetRef(
	ancestor v1alpha2.PolicyAncestorStatus,
	targetRef v1alpha2.LocalPolicyTargetReference,
	namespace string,
) error {
	if ancestor.ControllerName != ngfControllerName {
		return fmt.Errorf(
			"expected ancestor controller name to be %s, got %s",
			ngfControllerName,
			ancestor.ControllerName,
		)
	}

	if ancestor.AncestorRef.Namespace == nil {
		return fmt.Errorf("expected ancestor namespace to be %s, got nil", namespace)
	}

	if string(*ancestor.AncestorRef.Namespace) != namespace {
		return fmt.Errorf(
			"expected ancestor namespace to be %s, got %s",
			namespace,
			string(*ancestor.AncestorRef.Namespace),
		)
	}

	ancestorRef := ancestor.AncestorRef

	if ancestorRef.Name != targetRef.Name {
		return fmt.Errorf("expected ancestorRef to have name %s, got %s", targetRef.Name, ancestorRef.Name)
	}

	if ancestorRef.Group == nil {
		return fmt.Errorf("expected ancestorRef to have group %s, got nil", targetRef.Group)
	}

	if *ancestorRef.Group != targetRef.Group {
		return fmt.Errorf("expected ancestorRef to have group %s, got %s", targetRef.Group, string(*ancestorRef.Group))
	}

	if ancestorRef.Kind == nil {
		return fmt.Errorf("expected ancestorRef to have kind %s, got nil", targetRef.Kind)
	}

	if *ancestorRef.Kind != targetRef.Kind {
		return fmt.Errorf("expected ancestorRef to have kind %s, got %s", targetRef.Kind, string(*ancestorRef.Kind))
	}

	return nil
}
