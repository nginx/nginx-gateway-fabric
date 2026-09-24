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

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

// This test verifies that NginxProxy.UpstreamZoneAutoSizing configuration correctly
// flows through to the generated NGINX configuration, and that zone sizes are
// dynamically recalculated as endpoint counts change.
var _ = Describe("NginxProxy UpstreamZoneAutoSizing", Ordered, Label("functional", "nginxproxy"), func() {
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

	When("initial auto-sizing from NginxProxy CR is applied", func() {
		It("should calculate zone sizes based on the bufferMultiplier in the CR", func() {
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

			// Calculate the expected zone size using the same calculator used by the controller
			// The NginxProxy spec has bufferMultiplier: "200", minSize/maxSize unset (use defaults)
			calcConfig := config.ZoneSizeCalculatorConfig{
				BufferMultiplier: 200.0,
				MinSize:          128 * 1024,        // 128k default
				MaxSize:          512 * 1024 * 1024, // 512m default
			}
			calc := config.NewZoneSizeCalculator(calcConfig)

			// Both services have 1 replica at this point
			expectedZoneSize := calc.Calculate(1, config.HTTPProfile(*plusEnabled))
			GinkgoWriter.Printf("Expected zone size for 1 endpoint with bufferMultiplier=200: %s\n", expectedZoneSize)

			// Verify both coffee and tea upstreams have the calculated zone size
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

		It("should override auto-sizing only for the targeted Service", func() {
			uspNsName := types.NamespacedName{Name: "tea-zone-override", Namespace: namespace}
			Expect(waitForUSPolicyStatus(
				uspNsName,
				"gateway",
				metav1.ConditionTrue,
				gatewayv1.PolicyReasonAccepted,
			)).To(Succeed())

			// coffee is still at 1 replica at this point (scaling happens in the next When
			// block), so its zone should remain governed by the NginxProxy's auto-sizing
			// config, unaffected by tea's explicit override below.
			calcConfig := config.ZoneSizeCalculatorConfig{
				BufferMultiplier: 200.0,
				MinSize:          128 * 1024,        // 128k default
				MaxSize:          512 * 1024 * 1024, // 512m default
			}
			calc := config.NewZoneSizeCalculator(calcConfig)
			expectedCoffeeZoneSize := calc.Calculate(1, config.HTTPProfile(*plusEnabled))

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
					return fmt.Errorf("coffee upstream zone should remain auto-sized: %w", err)
				}

				return nil
			}).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})
	})

	When("backend endpoints scale up", func() {
		It("should dynamically recalculate zone sizes", func() {
			// Scale the coffee deployment from 1 to 4 replicas
			Expect(resourceManager.ScaleDeployment(namespace, "coffee", 4)).To(Succeed())

			// Wait for the new pods to be ready
			ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.CreateTimeout)
			defer cancel()
			Expect(resourceManager.WaitForPodsToBeReady(ctx, namespace)).To(Succeed())

			// Recalculate expected zone size for 4 endpoints
			calcConfig := config.ZoneSizeCalculatorConfig{
				BufferMultiplier: 200.0,
				MinSize:          128 * 1024,        // 128k default
				MaxSize:          512 * 1024 * 1024, // 512m default
			}
			calc := config.NewZoneSizeCalculator(calcConfig)

			expectedZoneSize := calc.Calculate(4, config.HTTPProfile(*plusEnabled))
			GinkgoWriter.Printf("Expected zone size for 4 endpoints with bufferMultiplier=200: %s\n", expectedZoneSize)

			// Verify the coffee upstream zone size has been recalculated (should be larger)
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
					return fmt.Errorf("coffee upstream zone not recalculated: %w", err)
				}

				return nil
			}).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())

			// Verify that tea's zone size remains the same (it was not scaled)
			conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, "")
			Expect(err).ToNot(HaveOccurred())

			teaUpstreamName := fmt.Sprintf("%s_tea_80", namespace)
			teaCalc := config.NewZoneSizeCalculator(calcConfig)
			teaExpectedZoneSize := teaCalc.Calculate(1, config.HTTPProfile(*plusEnabled))

			Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
				Directive: "zone",
				Value:     fmt.Sprintf("%s %s", teaUpstreamName, teaExpectedZoneSize),
				Upstream:  teaUpstreamName,
				File:      "http.conf",
			})).To(Succeed())
		})
	})
})
