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

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/tests/framework"
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

		Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
	})

	AfterAll(func() {
		Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
	})

	When("UpstreamSettingsPolicies target distinct Services", func() {
		usps := []string{
			"upstream-settings-policy/valid-usps.yaml",
		}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(usps, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(usps, namespace)).To(Succeed())
		})

		Specify("they are accepted", func() {
			usPolicies := []string{
				"multiple-http-svc-usp",
				"grpc-svc-usp",
			}

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
				podNames, err := framework.GetReadyNGFPodNames(k8sClient, ngfNamespace, releaseName, timeoutConfig.GetTimeout)
				Expect(err).ToNot(HaveOccurred())
				Expect(podNames).To(HaveLen(1))

				ngfPodName := podNames[0]

				conf, err = resourceManager.GetNginxConfig(ngfPodName, ngfNamespace)
				Expect(err).ToNot(HaveOccurred())
			})

			DescribeTable("are set properly for",
				func(expCfgs []framework.ExpectedNginxField) {
					for _, expCfg := range expCfgs {
						Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())
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
			Expect(resourceManager.ApplyFromFiles(usps, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(usps, namespace)).To(Succeed())
		})

		DescribeTable("upstreamSettingsPolicy status is set as expected",
			func(name string, status metav1.ConditionStatus, condReason v1alpha2.PolicyConditionReason) {
				uspolicyNsName := types.NamespacedName{Name: name, Namespace: namespace}
				Expect(waitForUSPolicyStatus(uspolicyNsName, gatewayName, status, condReason)).To(Succeed())
			},
			Entry("uspolicy merge-usp-1", "merge-usp-1", metav1.ConditionTrue, v1alpha2.PolicyReasonAccepted),
			Entry("uspolicy merge-usp-2", "merge-usp-2", metav1.ConditionTrue, v1alpha2.PolicyReasonAccepted),
			Entry("uspolicy merge-usp-3", "z-merge-usp-3", metav1.ConditionFalse, v1alpha2.PolicyReasonConflicted),
			Entry("uspolicy a-usp-wins", "a-usp-wins", metav1.ConditionTrue, v1alpha2.PolicyReasonAccepted),
			Entry("uspolicy z-usp", "z-usp", metav1.ConditionFalse, v1alpha2.PolicyReasonConflicted),
		)

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
				podNames, err := framework.GetReadyNGFPodNames(k8sClient, ngfNamespace, releaseName, timeoutConfig.GetTimeout)
				Expect(err).ToNot(HaveOccurred())
				Expect(podNames).To(HaveLen(1))

				ngfPodName := podNames[0]

				conf, err = resourceManager.GetNginxConfig(ngfPodName, ngfNamespace)
				Expect(err).ToNot(HaveOccurred())
			})

			DescribeTable("are set properly for",
				func(expCfgs []framework.ExpectedNginxField) {
					for _, expCfg := range expCfgs {
						Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())
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

			Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())

			uspolicyNsName := types.NamespacedName{Name: "usps-target-not-found", Namespace: namespace}

			Consistently(
				func() bool {
					return usPolicyHasNoAncestors(uspolicyNsName)
				}).WithTimeout(timeoutConfig.GetTimeout).
				WithPolling(500 * time.Millisecond).
				Should(BeTrue())

			Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
		})
	})

	When("UpstreamSettingsPolicy targets a Service that is owned by an invalid Gateway", func() {
		Specify("upstreamSettingsPolicy is not Accepted with the reason TargetNotFound", func() {
			// delete existing gateway
			gatewayFileName := "upstream-settings-policy/gateway.yaml"
			Expect(resourceManager.DeleteFromFiles([]string{gatewayFileName}, namespace)).To(Succeed())

			files := []string{"upstream-settings-policy/invalid-target-usps.yaml"}
			Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())

			uspolicyNsName := types.NamespacedName{Name: "soda-svc-usp", Namespace: namespace}
			gatewayName = "gateway-not-valid"
			Expect(waitForUSPolicyStatus(
				uspolicyNsName,
				gatewayName,
				metav1.ConditionFalse,
				v1alpha2.PolicyReasonTargetNotFound,
			)).To(Succeed())

			Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
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

			if err := k8sClient.Get(ctx, usPolicyNsName, &usPolicy); err != nil {
				return false, err
			}

			if len(usPolicy.Status.Ancestors) == 0 {
				GinkgoWriter.Printf("UpstreamSettingsPolicy %q does not have an ancestor status yet\n", usPolicy)

				return false, nil
			}

			if len(usPolicy.Status.Ancestors) != 1 {
				return false, fmt.Errorf("policy has %d ancestors, expected 1", len(usPolicy.Status.Ancestors))
			}

			ancestors := usPolicy.Status.Ancestors

			for _, ancestor := range ancestors {
				if err := ancestorMustEqualGatewayRef(ancestor, gatewayName, usPolicy.Namespace); err != nil {
					return false, err
				}

				err = ancestorStatusMustHaveAcceptedCondition(ancestor, condStatus, condReason)
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
