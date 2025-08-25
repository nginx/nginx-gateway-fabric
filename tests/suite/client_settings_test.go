package main

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

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
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

		nginxPodName string
	)

	BeforeAll(func() {
		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		GinkgoWriter.Printf("\nCreating namespace %q\n", ns)
		applyErr := resourceManager.Apply([]client.Object{ns})
		if applyErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during applying resource to namespace %q, error: %v\n",
				ns,
				applyErr,
			)
		}
		Expect(applyErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Namespace %q applied successfully,\n", ns)

		GinkgoWriter.Printf("Applying resources from files %v to namespace %q\n", files, namespace)
		applyFromFilesErr := resourceManager.ApplyFromFiles(files, namespace)
		if applyFromFilesErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during applying resource from files %v, in namespace %q, error: %v\n",
				files,
				namespace,
				applyFromFilesErr,
			)
		}
		Expect(applyFromFilesErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Resources from files applied successfully to namespace %q,\n", namespace)

		GinkgoWriter.Printf("Waiting for applications to be ready in namespace %q\n", namespace)
		waitErr := resourceManager.WaitForAppsToBeReady(namespace)
		if waitErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during waiting for applications to be ready in namespace %q, error: %v\n",
				namespace,
				waitErr,
			)
		}
		Expect(waitErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Applications ready in namespace %q\n", namespace)

		GinkgoWriter.Printf("Retrieving ready NGINX pods in namespace %q\n", namespace)
		nginxPodNames, err := framework.GetReadyNginxPodNames(k8sClient, namespace, timeoutConfig.GetStatusTimeout)
		if err != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during retrieving ready NGINX pod names in namespace %q, error: %v\n",
				namespace,
				err,
			)
		}
		Expect(err).ToNot(HaveOccurred())
		Expect(nginxPodNames).To(HaveLen(1))
		GinkgoWriter.Printf("NGINX pods in namespace %q: %v\n", namespace, nginxPodNames)

		nginxPodName = nginxPodNames[0]

		GinkgoWriter.Printf("Setting up port-forward for NGINX pod %q in namespace %q\n", nginxPodName, namespace)
		setUpPortForward(nginxPodName, namespace)
	})

	AfterAll(func() {
		GinkgoWriter.Printf("Adding NGINX logs and events report for namespace %q\n", namespace)
		framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
		GinkgoWriter.Printf("Cleaning up port-forward for namespace %q\n", namespace)
		cleanUpPortForward()

		GinkgoWriter.Printf("Deleting namespace %q\n", namespace)
		deleteErr := resourceManager.DeleteNamespace(namespace)
		if deleteErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during deleting namespace %q, error: %v\n",
				namespace,
				deleteErr,
			)
		}
		Expect(deleteErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Namespace %q deleted successfully,\n", namespace)
	})

	// Log the When block for valid policies
	When("valid ClientSettingsPolicies are created", func() {
		var (
			policies = []string{
				"clientsettings/valid-csps.yaml",
			}

			baseURL string
		)

		BeforeAll(func() {
			GinkgoWriter.Printf("Creating valid ClientSettingsPolicies from files %v in namespace %q\n", policies, namespace)
			applyErr := resourceManager.ApplyFromFiles(policies, namespace)
			if applyErr != nil {
				GinkgoWriter.Printf(
					"ERROR occurred during applying resource from files %v, in namespace %q, error: %v\n",
					policies,
					namespace,
					applyErr,
				)
			}
			Expect(applyErr).ToNot(HaveOccurred())
			GinkgoWriter.Printf("Valid ClientSettingsPolicies created successfully in namespace %q\n", namespace)

			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}

			baseURL = fmt.Sprintf("http://cafe.example.com:%d", port)
			GinkgoWriter.Printf("Setting up base URL for tests: %s\n", baseURL)
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

				GinkgoWriter.Printf("Waiting for ClientSettingsPolicy %q to be accepted\n", name)
				err := waitForCSPolicyToBeAccepted(nsname)
				if err != nil {
					GinkgoWriter.Printf(
						"ERROR occurred during waiting for ClientSettingsPolicy %q to be accepted, error: %v\n",
						name,
						err,
					)
				}
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", name))
				GinkgoWriter.Printf("ClientSettingsPolicy %q accepted successfully\n", name)
			}
		})

		Context("verify working traffic", func() {
			It("should return a 200 response for HTTPRoutes", func() {
				baseCoffeeURL := baseURL + "/coffee"
				baseTeaURL := baseURL + "/tea"

				Eventually(
					func() error {
						return expectRequestToSucceed(baseCoffeeURL, address, "URI: /coffee")
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())

				Eventually(
					func() error {
						return expectRequestToSucceed(baseTeaURL, address, "URI: /tea")
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		Context("nginx config", func() {
			var conf *framework.Payload
			filePrefix := fmt.Sprintf("/etc/nginx/includes/ClientSettingsPolicy_%s", namespace)

			BeforeAll(func() {
				var err error
				GinkgoWriter.Printf("Retrieving NGINX configuration for namespace %q, pod %s\n", namespace, nginxPodName)
				conf, err = resourceManager.GetNginxConfig(nginxPodName, namespace, "")
				if err != nil {
					GinkgoWriter.Printf(
						"ERROR occurred during retrieving NGINX configuration for namespace %q, pod %s, error: %v\n",
						namespace,
						nginxPodName,
						err,
					)
				}
				Expect(err).ToNot(HaveOccurred())
				GinkgoWriter.Printf("NGINX configuration retrieved successfully\n")
			})

			DescribeTable("is set properly for",
				func(expCfgs []framework.ExpectedNginxField) {
					for _, expCfg := range expCfgs {
						fieldValidationErr := framework.ValidateNginxFieldExists(conf, expCfg)
						GinkgoWriter.Printf("Validating NGINX field %q\n", expCfg)
						if fieldValidationErr != nil {
							GinkgoWriter.Printf(
								"ERROR occurred during validating NGINX field %q, error: %v\n",
								expCfg,
								fieldValidationErr,
							)
						}
						Expect(fieldValidationErr).ToNot(HaveOccurred())
						GinkgoWriter.Printf("NGINX field %q validated successfully\n", expCfg)
					}
				},
				Entry("gateway policy", []framework.ExpectedNginxField{
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s_gw-csp.conf", filePrefix),
						File:      "http.conf",
						Server:    "*.example.com",
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s_gw-csp.conf", filePrefix),
						File:      "http.conf",
						Server:    "cafe.example.com",
					},
					{
						Directive: "client_max_body_size",
						Value:     "1000",
						File:      fmt.Sprintf("%s_gw-csp.conf", filePrefix),
					},
					{
						Directive: "client_body_timeout",
						Value:     "30s",
						File:      fmt.Sprintf("%s_gw-csp.conf", filePrefix),
					},
					{
						Directive: "keepalive_requests",
						Value:     "100",
						File:      fmt.Sprintf("%s_gw-csp.conf", filePrefix),
					},
					{
						Directive: "keepalive_time",
						Value:     "5s",
						File:      fmt.Sprintf("%s_gw-csp.conf", filePrefix),
					},
					{
						Directive: "keepalive_timeout",
						Value:     "2s 1s",
						File:      fmt.Sprintf("%s_gw-csp.conf", filePrefix),
					},
				}),
				Entry("coffee route policy", []framework.ExpectedNginxField{
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s_coffee-route-csp.conf", filePrefix),
						File:      "http.conf",
						Server:    "cafe.example.com",
						Location:  "/coffee",
					},
					{
						Directive: "client_max_body_size",
						Value:     "2000",
						File:      fmt.Sprintf("%s_coffee-route-csp.conf", filePrefix),
					},
				}),
				Entry("tea route policy", []framework.ExpectedNginxField{
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s_tea-route-csp.conf", filePrefix),
						File:      "http.conf",
						Server:    "cafe.example.com",
						Location:  "/tea",
					},
					{
						Directive: "keepalive_requests",
						Value:     "200",
						File:      fmt.Sprintf("%s_tea-route-csp.conf", filePrefix),
					},
				}),
				Entry("soda route policy", []framework.ExpectedNginxField{
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s_soda-route-csp.conf", filePrefix),
						File:      "http.conf",
						Server:    "cafe.example.com",
						Location:  "/soda",
					},
					{
						Directive: "client_max_body_size",
						Value:     "3000",
						File:      fmt.Sprintf("%s_soda-route-csp.conf", filePrefix),
					},
				}),
				Entry("grpc route policy", []framework.ExpectedNginxField{
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s_grpc-route-csp.conf", filePrefix),
						File:      "http.conf",
						Server:    "*.example.com",
						Location:  "/helloworld.Greeter/SayHello",
					},
					{
						Directive: "client_max_body_size",
						Value:     "0",
						File:      fmt.Sprintf("%s_grpc-route-csp.conf", filePrefix),
					},
				}),
			)
		})

		// We only test that the client_max_body_size directive in this test is propagated correctly.
		// This is because we can easily verify this directive by sending requests with different sized payloads.
		DescribeTable("client_max_body_size requests work as expected",
			func(uri string, byteLengthOfRequestBody, expStatus int) {
				url := baseURL + uri

				payload := make([]byte, byteLengthOfRequestBody)
				GinkgoWriter.Printf("\nGenerating random payload of %d bytes for request to %s\n", byteLengthOfRequestBody, url)
				_, err := rand.Read(payload)
				if err != nil {
					GinkgoWriter.Printf("ERROR occurred during generating random payload for request to %s, error: %v\n", url, err)
				}
				Expect(err).ToNot(HaveOccurred())

				GinkgoWriter.Printf("Sending request to %s with payload of %d bytes\n", url, byteLengthOfRequestBody)
				resp, err := framework.Post(url, address, bytes.NewReader(payload), timeoutConfig.RequestTimeout, nil, nil)
				if err != nil {
					GinkgoWriter.Printf("ERROR occurred during sending request to %s, error: %v\n", url, err)
				}
				Expect(err).ToNot(HaveOccurred())
				GinkgoWriter.Printf("Received response status: %d for request to %s\n", resp.StatusCode, url)
				Expect(resp).To(HaveHTTPStatus(expStatus))

				if expStatus == http.StatusOK {
					GinkgoWriter.Printf("Request was successful, checking response body to include URI: %s\n", uri)
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
				"clientsettings/invalid-route-csp.yaml",
			}

			GinkgoWriter.Printf("\nCreating ClientSettingsPolicy from files %v in namespace %q\n", files, namespace)
			applyErr := resourceManager.ApplyFromFiles(files, namespace)
			if applyErr != nil {
				GinkgoWriter.Printf("ERROR occurred during applying files %v for %s, error: %v\n", files, namespace, applyErr)
			}
			Expect(applyErr).ToNot(HaveOccurred())
			GinkgoWriter.Printf("ClientSettingsPolicy created successfully\n")

			nsname := types.NamespacedName{Name: "invalid-route-csp", Namespace: namespace}
			Expect(waitForCSPolicyToHaveTargetNotFoundAcceptedCond(nsname)).To(Succeed())

			GinkgoWriter.Printf("Deleting ClientSettingsPolicy from files %v in namespace %q\n", files, namespace)
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

				GinkgoWriter.Printf("\nCreating ClientSettingsPolicies from files %v in namespace %q\n", policies, namespace)
				Expect(resourceManager.ApplyFromFiles(policies, namespace)).To(Succeed())
				GinkgoWriter.Printf("ClientSettingsPolicies created successfully in namespace\n")

				for _, name := range conflictedPolicyNames {
					nsname := types.NamespacedName{Name: name, Namespace: namespace}

					GinkgoWriter.Printf("Waiting for ClientSettingsPolicy %q to be marked as conflicted\n", name)
					err := waitForCSPolicyToBeConflicted(nsname)
					Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not marked as conflicted", name))
				}

				for _, name := range mergeablePolicyNames {
					nsname := types.NamespacedName{Name: name, Namespace: namespace}

					GinkgoWriter.Printf("Waiting for ClientSettingsPolicy %q to be accepted\n", name)
					err := waitForCSPolicyToBeAccepted(nsname)
					Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", name))
				}

				GinkgoWriter.Printf("Deleting ClientSettingsPolicies from files %v in namespace %q\n", policies, namespace)
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

			GinkgoWriter.Printf("\nRetrieving ClientSettingsPolicy %q\n", policyNsname)
			if err := k8sClient.Get(ctx, policyNsname, &pol); err != nil {
				GinkgoWriter.Printf("ERROR occurred during retrieving ClientSettingsPolicy %q, error: %v\n", policyNsname, err)

				return false, err
			}

			if len(pol.Status.Ancestors) == 0 {
				GinkgoWriter.Printf("ClientSettingsPolicy %q does not have an ancestor status yet\n", policyNsname)

				return false, nil
			}

			GinkgoWriter.Printf(
				"ClientSettingsPolicy %q has %d ancestors, and we expect 1\n",
				policyNsname,
				len(pol.Status.Ancestors),
			)
			if len(pol.Status.Ancestors) != 1 {
				return false, fmt.Errorf("policy has %d ancestors, expected 1", len(pol.Status.Ancestors))
			}

			ancestor := pol.Status.Ancestors[0]

			GinkgoWriter.Printf("Retrieving ancestor status for ClientSettingsPolicy %q\n", policyNsname)
			if err := ancestorMustEqualTargetRef(ancestor, pol.GetTargetRefs()[0], policyNsname.Namespace); err != nil {
				GinkgoWriter.Printf(
					"ERROR occurred during retrieving ancestor status for ClientSettingsPolicy %q, error: %v\n",
					policyNsname,
					err,
				)

				return false, err
			}

			GinkgoWriter.Printf("Checking ancestor status for ClientSettingsPolicy %q\n", policyNsname)
			err := ancestorStatusMustHaveAcceptedCondition(ancestor, condStatus, condReason)
			if err != nil {
				GinkgoWriter.Printf(
					"ERROR occurred during checking ancestor status for ClientSettingsPolicy %q, error: %v\n",
					policyNsname,
					err,
				)
			}

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

	if status.Conditions[0].Type != string(v1alpha2.RouteConditionAccepted) {
		return fmt.Errorf("expected condition type to be Accepted, got %s", status.Conditions[0].Type)
	}

	if status.Conditions[0].Status != condStatus {
		return fmt.Errorf("expected condition status to be %s, got %s", condStatus, status.Conditions[0].Status)
	}

	if status.Conditions[0].Reason != string(condReason) {
		return fmt.Errorf("expected condition reason to be %s, got %s", condReason, status.Conditions[0].Reason)
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
