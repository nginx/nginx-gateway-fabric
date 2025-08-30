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

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
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

		GinkgoWriter.Printf("Getting NginxGateway %q\n", nsname)
		if err := k8sClient.Get(ctx, nsname, &nginxGateway); err != nil {
			err := fmt.Errorf("failed to get nginxGateway: %w", err)
			GinkgoWriter.Printf("ERROR: %v\n", err)

			return nginxGateway, err
		}

		return nginxGateway, nil
	}

	verifyNginxGatewayConditions := func(ng ngfAPI.NginxGateway) error {
		GinkgoWriter.Printf("Verifying NginxGateway conditions to exist\n")
		if ng.Status.Conditions == nil {
			GinkgoWriter.Printf("ERROR: NginxGateway has no conditions\n")

			return errors.New("nginxGateway has no conditions")
		}

		GinkgoWriter.Printf("NginxGateway has %d conditions\n", len(ng.Status.Conditions))
		if len(ng.Status.Conditions) != 1 {
			condAmountErr := fmt.Errorf(
				"expected nginxGateway to have only one condition, instead has %d conditions",
				len(ng.Status.Conditions),
			)
			GinkgoWriter.Printf("ERROR: %v\n", condAmountErr)

			return condAmountErr
		}

		return nil
	}

	getNginxGatewayCurrentObservedGeneration := func(ng ngfAPI.NginxGateway) (int64, error) {
		GinkgoWriter.Printf("Getting NginxGateway current observed generation\n")
		if err := verifyNginxGatewayConditions(ng); err != nil {
			return 0, err
		}
		GinkgoWriter.Printf("NginxGateway current observed generation is %d\n", ng.Status.Conditions[0].ObservedGeneration)

		return ng.Status.Conditions[0].ObservedGeneration, nil
	}

	isContainingLogLine := func(logs, expectedLogLine string) bool {
		GinkgoWriter.Printf("Checking if logs contain expected line: %q\n", expectedLogLine)
		isContaining := strings.Contains(logs, expectedLogLine)
		GinkgoWriter.Printf("Line %q present: %v!\n", expectedLogLine, isContaining)

		return isContaining
	}

	verifyNginxGatewayStatus := func(ng ngfAPI.NginxGateway, expObservedGen int64) error {
		GinkgoWriter.Printf("\nVerifying NginxGateway status\n")
		if err := verifyNginxGatewayConditions(ng); err != nil {
			GinkgoWriter.Printf("ERROR in conditions verification: %v\n", err)

			return err
		}

		condition := ng.Status.Conditions[0]

		GinkgoWriter.Printf("Current NginxGateway condition: Type=%q, Status=%q, Reason=%q, ObservedGeneration=%d\n",
			condition.Type, condition.Status, condition.Reason, condition.ObservedGeneration)

		if condition.Type != "Valid" {
			condTypeErr := fmt.Errorf(
				"expected nginxGateway condition type to be Valid, instead has type %s",
				condition.Type,
			)
			GinkgoWriter.Printf("ERROR: %v\n", condTypeErr)

			return condTypeErr
		}

		if condition.Reason != "Valid" {
			condReasonErr := fmt.Errorf("expected nginxGateway reason to be Valid, instead is %s", condition.Reason)
			GinkgoWriter.Printf("ERROR: %v\n", condReasonErr)

			return condReasonErr
		}

		if condition.ObservedGeneration != expObservedGen {
			condOGErr := fmt.Errorf(
				"expected nginxGateway observed generation to be %d, instead is %d",
				expObservedGen,
				condition.ObservedGeneration,
			)
			GinkgoWriter.Printf("ERROR: %v\n", condOGErr)

			return condOGErr
		}

		return nil
	}

	GinkgoWriter.Printf("Getting ready NGINX Gateway pod names\n")
	getNGFPodName := func() (string, error) {
		podNames, err := framework.GetReadyNGFPodNames(
			k8sClient,
			ngfNamespace,
			releaseName,
			timeoutConfig.GetStatusTimeout,
		)
		if err != nil {
			GinkgoWriter.Printf("ERROR: %v\n", err)

			return "", err
		}

		GinkgoWriter.Printf("Ready NGINX Gateway pod names: %v\n", podNames)
		if len(podNames) != 1 {
			podsAmountErr := fmt.Errorf("expected 1 pod name, got %d", len(podNames))
			GinkgoWriter.Printf("ERROR: %v\n", podsAmountErr)

			return "", podsAmountErr
		}

		return podNames[0], nil
	}

	AfterAll(func() {
		// re-apply NginxGateway crd to restore NGF instance for following functional tests
		GinkgoWriter.Printf("Re-applying NginxGateway from files %v\n", files)
		applyErr := resourceManager.ApplyFromFiles(files, namespace)
		if applyErr != nil {
			GinkgoWriter.Printf("ERROR on applying from files %v: %v\n", files, applyErr)
		}
		Expect(applyErr).ToNot(HaveOccurred())

		Eventually(
			func() bool {
				GinkgoWriter.Printf("Waiting for NginxGateway %q to be ready\n", nginxGatewayNsname)
				ng, err := getNginxGateway(nginxGatewayNsname)
				if err != nil {
					GinkgoWriter.Printf("ERROR for gateway %q: %v\n", nginxGatewayNsname, err)

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
				GinkgoWriter.Printf("Getting NGINX Gateway pod name\n")
				ngfPodName, err := getNGFPodName()
				if err != nil {
					GinkgoWriter.Printf("ERROR: %v\n", err)
				}
				Expect(err).ToNot(HaveOccurred())

				GinkgoWriter.Printf("Getting NGINX Gateway resource by name %q\n", nginxGatewayNsname)
				ng, err := getNginxGateway(nginxGatewayNsname)
				Expect(err).ToNot(HaveOccurred())

				Expect(verifyNginxGatewayStatus(ng, int64(1))).To(Succeed())

				Eventually(
					func() bool {
						GinkgoWriter.Printf("Getting pod %q logs in namespace %q\n", ngfPodName, ngfNamespace)
						logs, err := resourceManager.GetPodLogs(ngfNamespace, ngfPodName, &core.PodLogOptions{
							Container: "nginx-gateway",
						})
						if err != nil {
							GinkgoWriter.Printf("ERROR while getting logs for pod %q: %v\n", ngfPodName, err)

							return false
						}

						return isContainingLogLine(logs, "\"level\":\"debug\"")
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
				GinkgoWriter.Printf("\nSetting up NGF with config: %+v\n", cfg)
				setup(cfg)

				GinkgoWriter.Printf("Getting NGINX Gateway pod name\n")
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
						GinkgoWriter.Printf("\nGetting pod %q logs in namespace %q\n", ngfPodName, ngfNamespace)
						logs, err := resourceManager.GetPodLogs(ngfNamespace, ngfPodName, &core.PodLogOptions{
							Container: "nginx-gateway",
						})
						if err != nil {
							GinkgoWriter.Printf("ERROR while getting logs for pod %q: %v\n", ngfPodName, err)

							return false
						}

						GinkgoWriter.Printf("Expecting logs to not contain debug level\n")
						return !isContainingLogLine(logs, "\"level\":\"debug\"")
					}).WithTimeout(timeoutConfig.GetTimeout).
					WithPolling(500 * time.Millisecond).
					Should(BeTrue())
			})
		})
	})

	When("testing on an existing NGF instance", Ordered, func() {
		BeforeAll(func() {
			var err error
			GinkgoWriter.Printf("Getting NGINX Gateway pod name\n")
			ngfPodName, err = getNGFPodName()
			if err != nil {
				GinkgoWriter.Printf("ERROR: %v\n", err)
			}
			Expect(err).ToNot(HaveOccurred())
		})

		When("NginxGateway is updated", func() {
			It("captures the change, the status is valid, and the observed generation is incremented", func() {
				// previous test has left the log level at info, this test will change the log level to debug
				GinkgoWriter.Printf("Getting NGINX Gateway resource by name %q\n", nginxGatewayNsname)
				ng, err := getNginxGateway(nginxGatewayNsname)
				if err != nil {
					GinkgoWriter.Printf("ERROR: %v\n", err)
				}
				Expect(err).ToNot(HaveOccurred())

				GinkgoWriter.Printf("Getting NGINX Gateway current observed generation\n")
				gen, err := getNginxGatewayCurrentObservedGeneration(ng)
				if err != nil {
					GinkgoWriter.Printf("ERROR: %v\n", err)
				}
				Expect(err).ToNot(HaveOccurred())

				Expect(verifyNginxGatewayStatus(ng, gen)).To(Succeed())

				GinkgoWriter.Printf("Getting NGINX Gateway pod %q logs for namespace %q\n", ngfPodName, ngfNamespace)
				logs, err := resourceManager.GetPodLogs(ngfNamespace, ngfPodName, &core.PodLogOptions{
					Container: "nginx-gateway",
				})
				if err != nil {
					GinkgoWriter.Printf("ERROR: %v\n", err)
				}
				Expect(err).ToNot(HaveOccurred())

				GinkgoWriter.Printf("Verifying NGINX Gateway logs do not contain %q\n", "\"level\":\"debug\"")
				Expect(logs).ToNot(ContainSubstring("\"level\":\"debug\""))

				GinkgoWriter.Printf("Verifying that apply from files %v succeeds\n", files)
				Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())

				Eventually(
					func() bool {
						GinkgoWriter.Printf("Getting NGINX Gateway resource by name %q\n", nginxGatewayNsname)
						ng, err := getNginxGateway(nginxGatewayNsname)
						if err != nil {
							GinkgoWriter.Printf("ERROR: %v\n", err)

							return false
						}

						return verifyNginxGatewayStatus(ng, gen+1) == nil
					}).WithTimeout(timeoutConfig.UpdateTimeout).
					WithPolling(500 * time.Millisecond).
					Should(BeTrue())

				Eventually(
					func() bool {
						GinkgoWriter.Printf("Getting pod %q logs in namespace %q\n", ngfPodName, ngfNamespace)
						logs, err := resourceManager.GetPodLogs(ngfNamespace, ngfPodName, &core.PodLogOptions{
							Container: "nginx-gateway",
						})
						if err != nil {
							GinkgoWriter.Printf("ERROR: %v\n", err)

							return false
						}

						return isContainingLogLine(logs, "\"level\":\"debug\"")
					}).WithTimeout(timeoutConfig.GetTimeout).
					WithPolling(500 * time.Millisecond).
					Should(BeTrue())
			})
		})

		When("NginxGateway is deleted", func() {
			It("captures the deletion and default values are used", func() {
				GinkgoWriter.Printf("Deleting K8S resource from files %v\n", files)
				deleteErr := resourceManager.DeleteFromFiles(files, namespace)
				if deleteErr != nil {
					GinkgoWriter.Printf("ERROR: %v\n", deleteErr)
				}
				Expect(deleteErr).ToNot(HaveOccurred())

				Eventually(
					func() error {
						GinkgoWriter.Printf("Getting NGINX Gateway resource by name %q\n", nginxGatewayNsname)
						_, err := getNginxGateway(nginxGatewayNsname)
						if err != nil {
							GinkgoWriter.Printf("ERROR: %v\n", err)
						}

						return err
					}).WithTimeout(timeoutConfig.DeleteTimeout).
					WithPolling(500 * time.Millisecond).
					Should(MatchError(ContainSubstring("failed to get nginxGateway")))

				Eventually(
					func() bool {
						GinkgoWriter.Printf("Getting pod %q logs in namespace %q\n", ngfPodName, ngfNamespace)
						logs, err := resourceManager.GetPodLogs(ngfNamespace, ngfPodName, &core.PodLogOptions{
							Container: "nginx-gateway",
						})
						if err != nil {
							GinkgoWriter.Printf("ERROR: %v\n", err)
							return false
						}

						return isContainingLogLine(logs, "NginxGateway configuration was deleted; using defaults")
					}).WithTimeout(timeoutConfig.GetTimeout).
					WithPolling(500 * time.Millisecond).
					Should(BeTrue())

				GinkgoWriter.Printf("Getting events in namespace %q\n", namespace)
				events, err := resourceManager.GetEvents(namespace)
				if err != nil {
					GinkgoWriter.Printf("ERROR: %v\n", err)
				}
				Expect(err).ToNot(HaveOccurred())
				GinkgoWriter.Printf("Got %d events in namespace %q\n", len(events.Items), namespace)

				var foundNginxGatewayDeletionEvent bool
				GinkgoWriter.Printf(
					"Looking for NginxGateway deletion event with next fields: item.Message = %q item.Type = %q item.Reason == %q\n",
					"NginxGateway configuration was deleted; using defaults",
					"Warning",
					"ResourceDeleted",
				)
				for _, item := range events.Items {
					GinkgoWriter.Printf("Found event with message %q, type %q and reason %q\n", item.Message, item.Type, item.Reason)
					if item.Message == "NginxGateway configuration was deleted; using defaults" &&
						item.Type == "Warning" &&
						item.Reason == "ResourceDeleted" {
						foundNginxGatewayDeletionEvent = true
						break
					}
				}
				GinkgoWriter.Printf("Is NginxGateway deletion event found: %v\n", foundNginxGatewayDeletionEvent)
				Expect(foundNginxGatewayDeletionEvent).To(BeTrue())
			})
		})
	})
})
