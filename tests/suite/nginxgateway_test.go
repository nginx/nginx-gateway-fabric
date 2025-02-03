package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/tests/framework"
)

var _ = Describe("NginxGateway", Ordered, Label("functional", "nginxGateway"), func() {
	var (
		ngfPodName string

		namespace          = "nginx-gateway"
		nginxGatewayNsname = types.NamespacedName{Name: releaseName + "-config", Namespace: namespace}

		files = []string{
			"nginxgateway/nginx-gateway.yaml",
		}
	)

	getNginxGateway := func(nsname types.NamespacedName) (ngfAPI.NginxGateway, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
		defer cancel()

		var nginxGateway ngfAPI.NginxGateway

		if err := k8sClient.Get(ctx, nsname, &nginxGateway); err != nil {
			return nginxGateway, fmt.Errorf("failed to get nginxGateway: %w", err)
		}

		return nginxGateway, nil
	}

	verifyNginxGatewayConditions := func(ng ngfAPI.NginxGateway) error {
		if ng.Status.Conditions == nil {
			return errors.New("nginxGateway has no conditions")
		}

		if len(ng.Status.Conditions) != 1 {
			return fmt.Errorf(
				"expected nginxGateway to have only one condition, instead has %d conditions",
				len(ng.Status.Conditions),
			)
		}

		return nil
	}

	getNginxGatewayCurrentObservedGeneration := func(ng ngfAPI.NginxGateway) (int64, error) {
		if err := verifyNginxGatewayConditions(ng); err != nil {
			return 0, err
		}

		return ng.Status.Conditions[0].ObservedGeneration, nil
	}

	verifyNginxGatewayStatus := func(ng ngfAPI.NginxGateway, expObservedGen int64) error {
		if err := verifyNginxGatewayConditions(ng); err != nil {
			return err
		}

		condition := ng.Status.Conditions[0]

		if condition.Type != "Valid" {
			return fmt.Errorf(
				"expected nginxGateway condition type to be Valid, instead has type %s",
				condition.Type,
			)
		}

		if condition.Reason != "Valid" {
			return fmt.Errorf("expected nginxGateway reason to be Valid, instead is %s", condition.Reason)
		}

		if condition.ObservedGeneration != expObservedGen {
			return fmt.Errorf(
				"expected nginxGateway observed generation to be %d, instead is %d",
				expObservedGen,
				condition.ObservedGeneration,
			)
		}

		return nil
	}

	getNGFPodName := func() (string, error) {
		podNames, err := framework.GetReadyNGFPodNames(
			k8sClient,
			ngfNamespace,
			releaseName,
			timeoutConfig.GetTimeout,
		)
		if err != nil {
			return "", err
		}

		if len(podNames) != 1 {
			return "", fmt.Errorf("expected 1 pod name, got %d", len(podNames))
		}

		return podNames[0], nil
	}

	AfterAll(func() {
		// re-apply NginxGateway crd to restore NGF instance for following functional tests
		Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())

		Eventually(
			func() bool {
				ng, err := getNginxGateway(nginxGatewayNsname)
				if err != nil {
					return false
				}

				return verifyNginxGatewayStatus(ng, int64(1)) == nil
			}).WithTimeout(timeoutConfig.UpdateTimeout).
			WithPolling(500 * time.Millisecond).
			Should(BeTrue())
	})

	When("testing NGF on startup", func() {
		When("log level is set to debug", func() {
			It("outputs debug logs and the status is valid", func() {
				ngfPodName, err := getNGFPodName()
				Expect(err).ToNot(HaveOccurred())

				ng, err := getNginxGateway(nginxGatewayNsname)
				Expect(err).ToNot(HaveOccurred())

				Expect(verifyNginxGatewayStatus(ng, int64(1))).To(Succeed())

				Eventually(
					func() bool {
						logs, err := resourceManager.GetPodLogs(ngfNamespace, ngfPodName, &core.PodLogOptions{
							Container: "nginx-gateway",
						})
						if err != nil {
							return false
						}

						return strings.Contains(logs, "\"level\":\"debug\"")
					}).WithTimeout(timeoutConfig.GetTimeout).
					WithPolling(500 * time.Millisecond).
					Should(BeTrue())
			})
		})

		When("default log level is used", func() {
			It("only outputs info logs and the status is valid", func() {
				teardown(releaseName)

				cfg := getDefaultSetupCfg()
				cfg.debugLogLevel = false
				setup(cfg)

				ngfPodName, err := getNGFPodName()
				Expect(err).ToNot(HaveOccurred())

				Eventually(
					func() bool {
						ng, err := getNginxGateway(nginxGatewayNsname)
						if err != nil {
							return false
						}

						return verifyNginxGatewayStatus(ng, int64(1)) == nil
					}).WithTimeout(timeoutConfig.UpdateTimeout).
					WithPolling(500 * time.Millisecond).
					Should(BeTrue())

				Consistently(
					func() bool {
						logs, err := resourceManager.GetPodLogs(ngfNamespace, ngfPodName, &core.PodLogOptions{
							Container: "nginx-gateway",
						})
						if err != nil {
							return false
						}

						return !strings.Contains(logs, "\"level\":\"debug\"")
					}).WithTimeout(timeoutConfig.GetTimeout).
					WithPolling(500 * time.Millisecond).
					Should(BeTrue())
			})
		})
	})

	When("testing on an existing NGF instance", Ordered, func() {
		BeforeAll(func() {
			var err error
			ngfPodName, err = getNGFPodName()
			Expect(err).ToNot(HaveOccurred())
		})

		When("NginxGateway is updated", func() {
			It("captures the change, the status is valid, and the observed generation is incremented", func() {
				// previous test has left the log level at info, this test will change the log level to debug
				ng, err := getNginxGateway(nginxGatewayNsname)
				Expect(err).ToNot(HaveOccurred())

				gen, err := getNginxGatewayCurrentObservedGeneration(ng)
				Expect(err).ToNot(HaveOccurred())

				Expect(verifyNginxGatewayStatus(ng, gen)).To(Succeed())

				logs, err := resourceManager.GetPodLogs(ngfNamespace, ngfPodName, &core.PodLogOptions{
					Container: "nginx-gateway",
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(logs).ToNot(ContainSubstring("\"level\":\"debug\""))

				Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())

				Eventually(
					func() bool {
						ng, err := getNginxGateway(nginxGatewayNsname)
						if err != nil {
							return false
						}

						return verifyNginxGatewayStatus(ng, gen+1) == nil
					}).WithTimeout(timeoutConfig.UpdateTimeout).
					WithPolling(500 * time.Millisecond).
					Should(BeTrue())

				Eventually(
					func() bool {
						logs, err := resourceManager.GetPodLogs(ngfNamespace, ngfPodName, &core.PodLogOptions{
							Container: "nginx-gateway",
						})
						if err != nil {
							return false
						}

						return strings.Contains(
							logs,
							"\"current\":\"debug\",\"msg\":\"Log level changed\",\"prev\":\"info\"",
						)
					}).WithTimeout(timeoutConfig.GetTimeout).
					WithPolling(500 * time.Millisecond).
					Should(BeTrue())
			})
		})

		When("NginxGateway is deleted", func() {
			It("captures the deletion and default values are used", func() {
				Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())

				Eventually(
					func() error {
						_, err := getNginxGateway(nginxGatewayNsname)
						return err
					}).WithTimeout(timeoutConfig.DeleteTimeout).
					WithPolling(500 * time.Millisecond).
					Should(MatchError(ContainSubstring("failed to get nginxGateway")))

				Eventually(
					func() bool {
						logs, err := resourceManager.GetPodLogs(ngfNamespace, ngfPodName, &core.PodLogOptions{
							Container: "nginx-gateway",
						})
						if err != nil {
							return false
						}

						return strings.Contains(logs, "NginxGateway configuration was deleted; using defaults")
					}).WithTimeout(timeoutConfig.GetTimeout).
					WithPolling(500 * time.Millisecond).
					Should(BeTrue())

				events, err := resourceManager.GetEvents(namespace)
				Expect(err).ToNot(HaveOccurred())

				var foundNginxGatewayDeletionEvent bool
				for _, item := range events.Items {
					if item.Message == "NginxGateway configuration was deleted; using defaults" &&
						item.Type == "Warning" &&
						item.Reason == "ResourceDeleted" {
						foundNginxGatewayDeletionEvent = true
						break
					}
				}
				Expect(foundNginxGatewayDeletionEvent).To(BeTrue())
			})
		})
	})
})
