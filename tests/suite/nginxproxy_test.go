package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

// This suite verifies the NginxProxy-level useClusterIP setting. It mirrors the UpstreamSettingsPolicy
// useClusterIP functional test (see upstream_settings_test.go), but configures the setting globally on the
// NginxProxy referenced by the Gateway instead of per-Service via a policy.
var _ = Describe("NginxProxy UseClusterIP", Ordered, Label("functional", "nginxproxy"), func() {
	var (
		// The NginxProxy is applied before the Gateway so the Gateway's infrastructure.parametersRef resolves.
		proxyFile = []string{"nginxproxy-use-cluster-ip/nginx-proxy.yaml"}
		files     = []string{
			"nginxproxy-use-cluster-ip/cafe.yaml",
			"nginxproxy-use-cluster-ip/gateway.yaml",
			"nginxproxy-use-cluster-ip/routes.yaml",
		}

		namespace = "clusterip"

		nginxPodName string
	)

	BeforeAll(func() {
		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(proxyFile, namespace)).To(Succeed())
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

		Expect(resourceManager.DeleteFromFiles(proxyFile, namespace)).To(Succeed())
		Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
	})

	Context("verify working traffic", func() {
		It("should return a 200 response", func() {
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			coffeeURL := fmt.Sprintf("http://cafe.example.com:%d/coffee", port)

			Eventually(
				func() error {
					return framework.ExpectRequestToSucceed(
						timeoutConfig.RequestTimeout,
						coffeeURL,
						address,
						"URI: /coffee",
					)
				}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})
	})

	Context("nginx config", func() {
		var clusterIP string

		BeforeAll(func() {
			ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout)
			defer cancel()

			var svc core.Service
			err := resourceManager.Get(
				ctx,
				types.NamespacedName{Name: "coffee", Namespace: namespace},
				&svc,
			)
			Expect(err).ToNot(HaveOccurred())
			clusterIP = svc.Spec.ClusterIP
		})

		It("uses the Service ClusterIP as the upstream server", func() {
			var serverAddr string
			if strings.Contains(clusterIP, ":") {
				serverAddr = fmt.Sprintf("[%s]:80", clusterIP)
			} else {
				serverAddr = fmt.Sprintf("%s:80", clusterIP)
			}

			upstreamName := fmt.Sprintf("%s_coffee_80", namespace)

			if *plusEnabled {
				// In NGINX Plus, upstream servers are managed via the Plus API and
				// persisted in a state file rather than as server directives in the
				// config. Read the state file to verify the ClusterIP is used.
				Eventually(func() error {
					ctx, cancel := context.WithTimeout(
						context.Background(),
						timeoutConfig.RequestTimeout,
					)
					defer cancel()

					stateFileContent, err := resourceManager.GetNginxStateFile(
						ctx, nginxPodName, namespace, upstreamName,
					)
					if err != nil {
						return err
					}

					if !strings.Contains(stateFileContent, serverAddr) {
						return fmt.Errorf(
							"expected state file for upstream %s to contain server %s, got: %s",
							upstreamName,
							serverAddr,
							stateFileContent,
						)
					}

					return nil
				}).
					WithTimeout(timeoutConfig.GetStatusTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			} else {
				Eventually(func() error {
					conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, "")
					if err != nil {
						return err
					}

					return framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "server",
						Value:     serverAddr,
						Upstream:  upstreamName,
						File:      "http.conf",
					})
				}).
					WithTimeout(timeoutConfig.GetStatusTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			}
		})
	})
})
