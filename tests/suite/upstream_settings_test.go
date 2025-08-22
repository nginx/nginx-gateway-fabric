package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var _ = Describe("UpstreamSettingsPolicy", Ordered, Label("functional", "uspolicy"), func() {
	var (
		files = []string{
			"upstream-settings-policy/cafe.yaml",
			"upstream-settings-policy/gateway.yaml",
			"upstream-settings-policy/grpc-backend.yaml",
			"upstream-settings-policy/routes.yaml",
		}

		namespace   = "uspolicy"
		gatewayName = "gateway"

		nginxPodName string
	)

	zoneSize := "512k"
	if *plusEnabled {
		zoneSize = "1m"
	}

	BeforeAll(func() {
		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		GinkgoWriter.Printf("\nCreating namespace %q\n", ns)
		cnErr := resourceManager.Apply([]client.Object{ns})
		if cnErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during applying resource to namespace %q, error: %v\n",
				ns,
				cnErr,
			)
		}
		Expect(cnErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Namespace %q applied successfully,\n", ns)

		// Log applying resources
		GinkgoWriter.Printf("Applying resources from files %v to namespace %q\n", files, namespace)
		applyErr := resourceManager.ApplyFromFiles(files, namespace)
		if applyErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during applying resource from files %v, in namespace %q, error: %v\n",
				files,
				namespace,
				applyErr,
			)
		}
		Expect(applyErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Resources from files applied successfully to namespace %q,\n", namespace)

		// Log waiting for readiness
		GinkgoWriter.Printf("Waiting for apps to be ready in namespace %q\n", namespace)
		waitingErr := resourceManager.WaitForAppsToBeReady(namespace)
		if waitingErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during waiting for apps to be ready in namespace %q, error: %v\n",
				namespace,
				waitingErr,
			)
		}
		Expect(waitingErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Apps are ready in namespace %q,\n", namespace)

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
		GinkgoWriter.Printf("Nginx pods in namespace %q: %v\n", namespace, nginxPodNames)
		Expect(nginxPodNames).To(HaveLen(1))

		nginxPodName = nginxPodNames[0]
		GinkgoWriter.Printf(
			"Setting up port-forward to nginx pod %s in namespace %q\n",
			nginxPodName,
			namespace,
		)
		setUpPortForward(nginxPodName, namespace)
	})

	AfterAll(func() {
		GinkgoWriter.Printf("Adding NGINX logs and events to report in namespace %q\n", namespace)
		framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
		GinkgoWriter.Printf("Cleaning up port-forward\n")
		cleanUpPortForward()

		GinkgoWriter.Printf("Deleting namespace %q\n", namespace)
		dnErr := resourceManager.DeleteNamespace(namespace)
		if dnErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during deleting namespace %q, error: %v\n",
				namespace,
				dnErr,
			)
		}
		Expect(dnErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Namespace %q deleted successfully,\n", namespace)
	})

	When("UpstreamSettingsPolicies target distinct Services", func() {
		usps := []string{
			"upstream-settings-policy/valid-usps.yaml",
		}

		BeforeAll(func() {
			GinkgoWriter.Printf("Applying resources from files %v to namespace %q\n", usps, namespace)
			applyErr := resourceManager.ApplyFromFiles(usps, namespace)
			if applyErr != nil {
				GinkgoWriter.Printf(
					"ERROR occurred during applying resource from files %v, in namespace %q, error: %v\n",
					usps,
					namespace,
					applyErr,
				)
			}
			Expect(applyErr).ToNot(HaveOccurred())
			GinkgoWriter.Printf("Resources from files applied successfully to namespace %q,\n", namespace)
		})

		AfterAll(func() {
			GinkgoWriter.Printf("Deleting resources from files %v in namespace %q\n", usps, namespace)
			drErr := resourceManager.DeleteFromFiles(usps, namespace)
			if drErr != nil {
				GinkgoWriter.Printf(
					"ERROR occurred during deleting resource from files %v, in namespace %q, error: %v\n",
					usps,
					namespace,
					drErr,
				)
			}
			Expect(drErr).ToNot(HaveOccurred())
		})

		Specify("they are accepted", func() {
			usPolicies := []string{
				"multiple-http-svc-usp",
				"grpc-svc-usp",
			}
			GinkgoWriter.Printf("Verifying acceptance for policies %v\n", usPolicies)
			for _, name := range usPolicies {
				uspolicyNsName := types.NamespacedName{Name: name, Namespace: namespace}

				err := waitForUSPolicyStatus(
					uspolicyNsName,
					gatewayName,
					metav1.ConditionTrue,
					v1alpha2.PolicyReasonAccepted,
				)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", name))
			}
		})

		Context("verify working traffic", func() {
			It("should return a 200 response for HTTPRoutes", func() {
				port := 80
				if portFwdPort != 0 {
					port = portFwdPort
				}
				baseCoffeeURL := fmt.Sprintf("http://cafe.example.com:%d%s", port, "/coffee")
				baseTeaURL := fmt.Sprintf("http://cafe.example.com:%d%s", port, "/tea")

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

		Context("nginx directives", func() {
			var conf *framework.Payload

			BeforeAll(func() {
				GinkgoWriter.Printf("Retrieving NGINX configuration for pod %s in namespace %q\n", nginxPodName, namespace)
				var err error
				conf, err = resourceManager.GetNginxConfig(nginxPodName, namespace, "")
				if err != nil {
					GinkgoWriter.Printf(
						"Failed to retrieve NGINX configuration for pod %s in namespace %q: %v\n",
						nginxPodName,
						namespace,
						err,
					)
				}
				Expect(err).ToNot(HaveOccurred())
				GinkgoWriter.Printf("NGINX configuration retrieved successfully\n")
			})

			DescribeTable("are set properly for",
				func(expCfgs []framework.ExpectedNginxField) {
					for _, expCfg := range expCfgs {
						GinkgoWriter.Printf("Validating NGINX field: %v\n", expCfg)
						validationErr := framework.ValidateNginxFieldExists(conf, expCfg)
						if validationErr != nil {
							GinkgoWriter.Printf("NGINX field validation failed: %v\n", validationErr)
						}
						Expect(validationErr).ToNot(HaveOccurred())
					}
				},
				Entry("HTTP upstreams", []framework.ExpectedNginxField{
					{
						Directive: "upstream",
						Value:     "uspolicy_coffee_80",
						File:      "http.conf",
					},
					{
						Directive: "upstream",
						Value:     "uspolicy_tea_80",
						File:      "http.conf",
					},
					{
						Directive: "zone",
						Value:     fmt.Sprintf("uspolicy_coffee_80 %s", zoneSize),
						Upstream:  "uspolicy_coffee_80",
						File:      "http.conf",
					},
					{
						Directive: "zone",
						Value:     fmt.Sprintf("uspolicy_tea_80 %s", zoneSize),
						Upstream:  "uspolicy_tea_80",
						File:      "http.conf",
					},
					{
						Directive: "keepalive",
						Value:     "10",
						Upstream:  "uspolicy_coffee_80",
						File:      "http.conf",
					},
					{
						Directive: "keepalive",
						Value:     "10",
						Upstream:  "uspolicy_tea_80",
						File:      "http.conf",
					},
					{
						Directive: "keepalive_requests",
						Value:     "3",
						Upstream:  "uspolicy_coffee_80",
						File:      "http.conf",
					},
					{
						Directive: "keepalive_requests",
						Value:     "3",
						Upstream:  "uspolicy_tea_80",
						File:      "http.conf",
					},
					{
						Directive: "keepalive_time",
						Value:     "10s",
						Upstream:  "uspolicy_coffee_80",
						File:      "http.conf",
					},
					{
						Directive: "keepalive_time",
						Value:     "10s",
						Upstream:  "uspolicy_tea_80",
						File:      "http.conf",
					},
					{
						Directive: "keepalive_timeout",
						Value:     "50s",
						Upstream:  "uspolicy_coffee_80",
						File:      "http.conf",
					},
					{
						Directive: "keepalive_timeout",
						Value:     "50s",
						Upstream:  "uspolicy_tea_80",
						File:      "http.conf",
					},
				}),
				Entry("GRPC upstreams", []framework.ExpectedNginxField{
					{
						Directive: "upstream",
						Value:     "uspolicy_grpc-backend_8080",
						File:      "http.conf",
					},
					{
						Directive: "zone",
						Value:     "uspolicy_grpc-backend_8080 64k",
						Upstream:  "uspolicy_grpc-backend_8080",
						File:      "http.conf",
					},
					{
						Directive: "keepalive",
						Value:     "100",
						Upstream:  "uspolicy_grpc-backend_8080",
						File:      "http.conf",
					},
					{
						Directive: "keepalive_requests",
						Value:     "45",
						Upstream:  "uspolicy_grpc-backend_8080",
						File:      "http.conf",
					},
					{
						Directive: "keepalive_time",
						Value:     "1m",
						Upstream:  "uspolicy_grpc-backend_8080",
						File:      "http.conf",
					},
					{
						Directive: "keepalive_timeout",
						Value:     "5h",
						Upstream:  "uspolicy_grpc-backend_8080",
						File:      "http.conf",
					},
				}),
			)
		})
	})

	When("multiple UpstreamSettingsPolicies with overlapping settings target the same Service", func() {
		usps := []string{
			"upstream-settings-policy/valid-merge-usps.yaml",
		}

		BeforeAll(func() {
			GinkgoWriter.Printf("\nApplying resources from files %v to namespace %q\n", usps, namespace)
			applyErr := resourceManager.ApplyFromFiles(usps, namespace)
			if applyErr != nil {
				GinkgoWriter.Printf(
					"Failed to apply resources from files %v to namespace %q: %v\n",
					usps,
					namespace,
					applyErr,
				)
			}
			Expect(applyErr).ToNot(HaveOccurred())
		})

		AfterAll(func() {
			GinkgoWriter.Printf("Deleting resources from files %v in namespace %q\n", usps, namespace)
			deleteErr := resourceManager.DeleteFromFiles(usps, namespace)
			if deleteErr != nil {
				GinkgoWriter.Printf(
					"Failed to delete resources from files %v in namespace %q: %v\n",
					usps,
					namespace,
					deleteErr,
				)
			}
			Expect(deleteErr).ToNot(HaveOccurred())
		})

		DescribeTable("upstreamSettingsPolicy status is set as expected",
			func(name string, status metav1.ConditionStatus, condReason v1alpha2.PolicyConditionReason) {
				uspolicyNsName := types.NamespacedName{Name: name, Namespace: namespace}
				GinkgoWriter.Printf(
					"Waiting for upstreamSettingsPolicy %q on gateway %q status to be %q with reason %q\n",
					name,
					gatewayName,
					status,
					condReason,
				)
				waitingStatusErr := waitForUSPolicyStatus(uspolicyNsName, gatewayName, status, condReason)
				if waitingStatusErr != nil {
					GinkgoWriter.Printf("Failed to wait for upstreamSettingsPolicy status: %v\n", waitingStatusErr)
				}
				Expect(waitingStatusErr).ToNot(HaveOccurred())
				GinkgoWriter.Printf(
					"UpstreamSettingsPolicy %q status is %q with reason %q Succeeded\n",
					name,
					status,
					condReason,
				)
			},
			Entry("uspolicy merge-usp-1", "merge-usp-1", metav1.ConditionTrue, v1alpha2.PolicyReasonAccepted),
			Entry("uspolicy merge-usp-2", "merge-usp-2", metav1.ConditionTrue, v1alpha2.PolicyReasonAccepted),
			Entry("uspolicy merge-usp-3", "z-merge-usp-3", metav1.ConditionFalse, v1alpha2.PolicyReasonConflicted),
			Entry("uspolicy a-usp-wins", "a-usp-wins", metav1.ConditionTrue, v1alpha2.PolicyReasonAccepted),
			Entry("uspolicy z-usp", "z-usp", metav1.ConditionFalse, v1alpha2.PolicyReasonConflicted),
		)

		Context("verify working traffic", func() {
			It("should return a 200 response for HTTPRoutes", func() {
				GinkgoWriter.Printf("Testing HTTPRoutes reachability in namespace %q\n", namespace)
				port := 80
				if portFwdPort != 0 {
					port = portFwdPort
				}
				baseCoffeeURL := fmt.Sprintf("http://cafe.example.com:%d%s", port, "/coffee")
				baseTeaURL := fmt.Sprintf("http://cafe.example.com:%d%s", port, "/tea")

				Eventually(
					func() error {
						return expectRequestToSucceed(baseCoffeeURL, address, "URI: /coffee")
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(1000 * time.Millisecond).
					Should(Succeed())

				Eventually(
					func() error {
						return expectRequestToSucceed(baseTeaURL, address, "URI: /tea")
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(1000 * time.Millisecond).
					Should(Succeed())
			})
		})

		Context("nginx directives", func() {
			var conf *framework.Payload

			BeforeAll(func() {
				GinkgoWriter.Printf(
					"\nRetrieving NGINX configuration for pod %s in namespace %q\n",
					nginxPodName,
					namespace,
				)
				var err error
				conf, err = resourceManager.GetNginxConfig(nginxPodName, namespace, "")
				if err != nil {
					GinkgoWriter.Printf("Failed to retrieve NGINX configuration for pod %s: %v\n", nginxPodName, err)
				}
				Expect(err).ToNot(HaveOccurred())
				GinkgoWriter.Printf("NGINX configuration retrieved successfully for pod %s\n", nginxPodName)
			})

			DescribeTable("are set properly for",
				func(expCfgs []framework.ExpectedNginxField) {
					for _, expCfg := range expCfgs {
						GinkgoWriter.Printf("Validating NGINX field: %q\n", expCfg)
						validationErr := framework.ValidateNginxFieldExists(conf, expCfg)
						if validationErr != nil {
							GinkgoWriter.Printf("Failed to validate NGINX field %q: %v\n", expCfg, validationErr)
						}
						Expect(validationErr).ToNot(HaveOccurred())
						GinkgoWriter.Printf("NGINX field %q validated successfully\n", expCfg)
					}
				},
				Entry("Coffee upstream", []framework.ExpectedNginxField{
					{
						Directive: "upstream",
						Value:     "uspolicy_coffee_80",
						File:      "http.conf",
					},
					{
						Directive: "zone",
						Value:     fmt.Sprintf("uspolicy_coffee_80 %s", zoneSize),
						Upstream:  "uspolicy_coffee_80",
						File:      "http.conf",
					},
					{
						Directive: "keepalive",
						Value:     "100",
						Upstream:  "uspolicy_coffee_80",
						File:      "http.conf",
					},
					{
						Directive: "keepalive_requests",
						Value:     "55",
						Upstream:  "uspolicy_coffee_80",
						File:      "http.conf",
					},
					{
						Directive: "keepalive_time",
						Value:     "1m",
						Upstream:  "uspolicy_coffee_80",
						File:      "http.conf",
					},
					{
						Directive: "keepalive_timeout",
						Value:     "5h",
						Upstream:  "uspolicy_coffee_80",
						File:      "http.conf",
					},
				}),
				Entry("Tea upstream", []framework.ExpectedNginxField{
					{
						Directive: "zone",
						Value:     "uspolicy_tea_80 128k",
						Upstream:  "uspolicy_tea_80",
						File:      "http.conf",
					},
					{
						Directive: "upstream",
						Value:     "uspolicy_tea_80",
						File:      "http.conf",
					},
				}),
			)
		})
	})

	When("UpstreamSettingsPolicy targets a Service that does not exist", func() {
		Specify("upstreamSettingsPolicy sets no condition", func() {
			files := []string{"upstream-settings-policy/invalid-svc-usps.yaml"}

			GinkgoWriter.Printf("Applying resources from files %v to namespace %q\n", files, namespace)
			applyErr := resourceManager.ApplyFromFiles(files, namespace)
			if applyErr != nil {
				GinkgoWriter.Printf(
					"Failed to apply resources from files %v to namespace %q: %v\n",
					files,
					namespace, applyErr,
				)
			}
			Expect(applyErr).ToNot(HaveOccurred())

			uspolicyNsName := types.NamespacedName{Name: "usps-target-not-found", Namespace: namespace}

			GinkgoWriter.Printf("Waiting for UpstreamSettingsPolicy %q to have no ancestors\n", uspolicyNsName)
			Consistently(
				func() bool {
					return usPolicyHasNoAncestors(uspolicyNsName)
				}).WithTimeout(timeoutConfig.GetTimeout).
				WithPolling(500 * time.Millisecond).
				Should(BeTrue())

			GinkgoWriter.Printf("Deleting resources from files %v in namespace %q\n", files, namespace)
			deleteErr := resourceManager.DeleteFromFiles(files, namespace)
			if deleteErr != nil {
				GinkgoWriter.Printf(
					"Failed to delete resources from files %v in namespace %q: %v\n",
					files,
					namespace,
					deleteErr,
				)
			}
			Expect(deleteErr).ToNot(HaveOccurred())
		})
	})

	When("UpstreamSettingsPolicy targets a Service that is owned by an invalid Gateway", func() {
		Specify("upstreamSettingsPolicy is not Accepted with the reason TargetNotFound", func() {
			// delete existing gateway
			gatewayFileName := "upstream-settings-policy/gateway.yaml"
			GinkgoWriter.Printf("Deleting resources from file %q in namespace %q\n", gatewayFileName, namespace)
			deleteErr := resourceManager.DeleteFromFiles([]string{gatewayFileName}, namespace)
			if deleteErr != nil {
				GinkgoWriter.Printf(
					"Failed to delete resources from files %v in namespace %q: %v\n",
					[]string{gatewayFileName},
					namespace,
					deleteErr,
				)
			}
			Expect(deleteErr).ToNot(HaveOccurred())

			files := []string{"upstream-settings-policy/invalid-target-usps.yaml"}
			GinkgoWriter.Printf("Applying resources from files %v to namespace %q\n", files, namespace)
			applyErr := resourceManager.ApplyFromFiles(files, namespace)
			if applyErr != nil {
				GinkgoWriter.Printf(
					"Failed to apply resources from files %v to namespace %q: %v\n",
					files,
					namespace,
					applyErr,
				)
			}
			Expect(applyErr).ToNot(HaveOccurred())

			uspolicyNsName := types.NamespacedName{Name: "soda-svc-usp", Namespace: namespace}
			gatewayName = "gateway-not-valid"
			Expect(waitForUSPolicyStatus(
				uspolicyNsName,
				gatewayName,
				metav1.ConditionFalse,
				v1alpha2.PolicyReasonTargetNotFound,
			)).To(Succeed())

			GinkgoWriter.Printf("Deleting resources from files %v in namespace %q\n", files, namespace)
			deleteFromFileErr := resourceManager.DeleteFromFiles(files, namespace)
			if deleteFromFileErr != nil {
				GinkgoWriter.Printf(
					"Failed to delete resources from files %v in namespace %q: %v\n",
					files,
					namespace,
					deleteFromFileErr,
				)
			}
			Expect(deleteFromFileErr).ToNot(HaveOccurred())
		})
	})
})

func usPolicyHasNoAncestors(usPolicyNsName types.NamespacedName) bool {
	GinkgoWriter.Printf("Checking that UpstreamSettingsPolicy %q has no ancestors in status\n", usPolicyNsName)

	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout)
	defer cancel()

	var usPolicy ngfAPI.UpstreamSettingsPolicy
	if err := k8sClient.Get(ctx, usPolicyNsName, &usPolicy); err != nil {
		GinkgoWriter.Printf("Failed to get UpstreamSettingsPolicy %q: %s", usPolicyNsName, err.Error())

		return false
	}

	return len(usPolicy.Status.Ancestors) == 0
}

func waitForUSPolicyStatus(
	usPolicyNsName types.NamespacedName,
	gatewayName string,
	condStatus metav1.ConditionStatus,
	condReason v1alpha2.PolicyConditionReason,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout*2)
	defer cancel()

	GinkgoWriter.Printf(
		"Waiting for UpstreamSettings Policy %q to have the condition %q/%q\n",
		usPolicyNsName,
		condStatus,
		condReason,
	)

	return wait.PollUntilContextCancel(
		ctx,
		2000*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var usPolicy ngfAPI.UpstreamSettingsPolicy
			var err error

			GinkgoWriter.Printf("Retrieving UpstreamSettingsPolicy %q\n", usPolicyNsName)
			if err := k8sClient.Get(ctx, usPolicyNsName, &usPolicy); err != nil {
				GinkgoWriter.Printf("Failed to get UpstreamSettingsPolicy %q: %s", usPolicyNsName, err.Error())

				return false, err
			}

			if len(usPolicy.Status.Ancestors) == 0 {
				GinkgoWriter.Printf("UpstreamSettingsPolicy %q does not have an ancestor status yet\n", usPolicy)

				return false, nil
			}

			if len(usPolicy.Status.Ancestors) != 1 {
				lenErr := fmt.Errorf("policy has %d ancestors, expected 1", len(usPolicy.Status.Ancestors))
				GinkgoWriter.Printf(
					"UpstreamSettingsPolicy %q has %d ancestors, expected 1, returning error: %v\n",
					usPolicyNsName,
					len(usPolicy.Status.Ancestors),
					lenErr,
				)

				return false, lenErr
			}

			ancestors := usPolicy.Status.Ancestors

			for _, ancestor := range ancestors {
				if err := ancestorMustEqualGatewayRef(ancestor, gatewayName, usPolicy.Namespace); err != nil {
					GinkgoWriter.Printf(
						"Failed to validate ancestor %q for UpstreamSettingsPolicy %q: %v\n",
						ancestor,
						usPolicyNsName,
						err,
					)

					return false, err
				}

				err = ancestorStatusMustHaveAcceptedCondition(ancestor, condStatus, condReason)
				if err != nil {
					GinkgoWriter.Printf(
						"Failed to validate ancestor %q for UpstreamSettingsPolicy %q: %v\n",
						ancestor,
						usPolicyNsName,
						err,
					)
				}
			}

			return err == nil, err
		},
	)
}

func ancestorMustEqualGatewayRef(
	ancestor v1alpha2.PolicyAncestorStatus,
	gatewayName string,
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

	if string(ancestorRef.Name) != gatewayName {
		return fmt.Errorf("expected ancestorRef to have name %s, got %s", gatewayName, ancestorRef.Name)
	}

	if ancestorRef.Kind == nil {
		return errors.New("expected ancestorRef to have kind Gateway, got nil")
	}

	if *ancestorRef.Kind != gatewayv1.Kind("Gateway") {
		return fmt.Errorf(
			"expected ancestorRef to have kind %s, got %s",
			"Gateway",
			string(*ancestorRef.Kind),
		)
	}

	return nil
}
