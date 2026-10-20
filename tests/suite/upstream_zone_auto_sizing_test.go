package main

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngxConfig "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

// This test verifies that NginxProxy.ZoneSize: "auto" flows through to the generated NGINX
// configuration as the flat 64k auto-sizing cold start.
var _ = Describe("NginxProxy ZoneSize auto-sizing", Ordered, Label("functional", "nginxproxy"), func() {
	var (
		files = []string{
			"upstream-zone-auto-sizing/nginx-proxy.yaml",
			"upstream-zone-auto-sizing/cafe.yaml",
			"upstream-zone-auto-sizing/gateway.yaml",
			"upstream-zone-auto-sizing/routes.yaml",
		}

		namespace    = "zone-auto-sizing"
		nginxPodName string
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

		nginxPodNames, err := resourceManager.GetReadyNginxPodNames(
			namespace,
			timeoutConfig.GetStatusTimeout,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(nginxPodNames).To(HaveLen(1))

		nginxPodName = nginxPodNames[0]

		setUpPortForward(nginxPodName, namespace)
	})

	AfterAll(func() {
		framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
		cleanUpPortForward()

		Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
	})

	When("ZoneSize is set to \"auto\" on the NginxProxy", func() {
		It("should use the flat 64k auto-sizing cold start for all upstreams", func() {
			// Verify traffic works first
			port := helpers.BuildPortFwdPort(80, portFwdPort)
			baseCoffeeURL := helpers.BuildPortFwdURL("cafe.example.com/coffee", port)
			baseTeaURL := helpers.BuildPortFwdURL("cafe.example.com/tea", port)

			Eventually(
				func() error {
					return framework.ExpectRequestToSucceed(timeoutConfig.RequestTimeout, baseCoffeeURL, address, "URI: /coffee")
				}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())

			Eventually(
				func() error {
					return framework.ExpectRequestToSucceed(timeoutConfig.RequestTimeout, baseTeaURL, address, "URI: /tea")
				}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())

			expectedZoneSize := ngxConfig.NewZoneSizeCalculator(nil, 0).
				Resolve("", helpers.GetPointer(ngxConfig.ZoneSizeAuto), ngxConfig.HTTPProfile(*plusEnabled))
			GinkgoWriter.Printf("Expected auto-sizing cold-start zone size: %s\n", expectedZoneSize)

			Eventually(func() error {
				conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, "")
				if err != nil {
					return err
				}

				coffeeUpstreamName := fmt.Sprintf("%s_coffee_80", namespace)
				if err := framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "zone",
					Value:     fmt.Sprintf("%s %s", coffeeUpstreamName, expectedZoneSize),
					Upstream:  coffeeUpstreamName,
					File:      "http.conf",
				}); err != nil {
					return fmt.Errorf("coffee upstream zone mismatch: %w", err)
				}

				teaUpstreamName := fmt.Sprintf("%s_tea_80", namespace)
				if err := framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "zone",
					Value:     fmt.Sprintf("%s %s", teaUpstreamName, expectedZoneSize),
					Upstream:  teaUpstreamName,
					File:      "http.conf",
				}); err != nil {
					return fmt.Errorf("tea upstream zone mismatch: %w", err)
				}

				return nil
			}).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})
	})

	When("a Service has an explicit UpstreamSettingsPolicy.ZoneSize override", func() {
		uspFiles := []string{"upstream-zone-auto-sizing/upstream-settings-policy.yaml"}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(uspFiles, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(uspFiles, namespace)).To(Succeed())
		})

		It("should override the \"auto\" default only for the targeted Service", func() {
			uspNsName := types.NamespacedName{Name: "tea-zone-override", Namespace: namespace}
			Expect(waitForUSPolicyStatus(
				uspNsName,
				"gateway",
				metav1.ConditionTrue,
				gatewayv1.PolicyReasonAccepted,
			)).To(Succeed())

			// coffee has no override, so it remains at the flat "auto" cold start, unaffected by
			// tea's explicit override below.
			expectedCoffeeZoneSize := ngxConfig.NewZoneSizeCalculator(nil, 0).
				Resolve("", helpers.GetPointer(ngxConfig.ZoneSizeAuto), ngxConfig.HTTPProfile(*plusEnabled))

			Eventually(func() error {
				conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, "")
				if err != nil {
					return err
				}

				teaUpstreamName := fmt.Sprintf("%s_tea_80", namespace)
				if err := framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "zone",
					Value:     fmt.Sprintf("%s 2m", teaUpstreamName),
					Upstream:  teaUpstreamName,
					File:      "http.conf",
				}); err != nil {
					return fmt.Errorf("tea upstream zone override not applied: %w", err)
				}

				coffeeUpstreamName := fmt.Sprintf("%s_coffee_80", namespace)
				if err := framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "zone",
					Value:     fmt.Sprintf("%s %s", coffeeUpstreamName, expectedCoffeeZoneSize),
					Upstream:  coffeeUpstreamName,
					File:      "http.conf",
				}); err != nil {
					return fmt.Errorf("coffee upstream zone should remain at the auto default: %w", err)
				}

				return nil
			}).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})
	})

	When("the coffee Deployment's endpoint count increases", Ordered, func() {
		BeforeAll(func() {
			Expect(resourceManager.ScaleDeployment(namespace, "coffee", 100)).To(Succeed())

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			Expect(resourceManager.WaitForPodsToBeReady(ctx, namespace)).To(Succeed())
		})

		It("should automatically grow the coffee upstream zone size to 128k", func() {
			// tea is untouched by the coffee scale-up, so it remains at the flat "auto" cold
			// start.
			expectedTeaZoneSize := ngxConfig.NewZoneSizeCalculator(nil, 0).
				Resolve("", helpers.GetPointer(ngxConfig.ZoneSizeAuto), ngxConfig.HTTPProfile(*plusEnabled))

			Eventually(func() error {
				conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, "")
				if err != nil {
					return err
				}

				coffeeUpstreamName := fmt.Sprintf("%s_coffee_80", namespace)
				if err := framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "zone",
					Value:     fmt.Sprintf("%s 128k", coffeeUpstreamName),
					Upstream:  coffeeUpstreamName,
					File:      "http.conf",
				}); err != nil {
					return fmt.Errorf("coffee upstream zone did not grow to 128k: %w", err)
				}

				teaUpstreamName := fmt.Sprintf("%s_tea_80", namespace)
				if err := framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "zone",
					Value:     fmt.Sprintf("%s %s", teaUpstreamName, expectedTeaZoneSize),
					Upstream:  teaUpstreamName,
					File:      "http.conf",
				}); err != nil {
					return fmt.Errorf("tea upstream zone should remain unaffected: %w", err)
				}

				return nil
			}).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})
	})
})
