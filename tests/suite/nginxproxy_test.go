package main

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
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

		namespace string

		nginxPodName string
	)

	BeforeAll(func() {
		namespace = fmt.Sprintf("clusterip-%d", GinkgoParallelProcess())

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
			port := helpers.BuildPortFwdPort(80, portFwdPort)
			coffeeURL := helpers.BuildPortFwdURL("cafe.example.com/coffee", port)

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
		It("uses the Service ClusterIP as the upstream server", func() {
			expectUpstreamToUseClusterIP(nginxPodName, namespace, "coffee", fmt.Sprintf("%s_coffee_80", namespace))
		})
	})

	Context("when an NginxProxy with an invalid spec is referenced by a Gateway", Ordered, func() {
		// The infrastructure provider applies an NginxProxy with a malformed CIDR (prefix length out of range).
		// The Gateway should surface InvalidParameters; existing traffic on the valid Gateway must be unaffected.
		invalidFiles := []string{"nginxproxy-use-cluster-ip/invalid-nginx-proxy.yaml"}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(invalidFiles, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(invalidFiles, namespace)).To(Succeed())
		})

		It("sets Programmed=False with reason InvalidParameters on the referencing Gateway", func() {
			Eventually(func(g Gomega) {
				var gw gatewayv1.Gateway
				g.Expect(resourceManager.Get(
					context.Background(),
					types.NamespacedName{Name: "invalid-gateway", Namespace: namespace},
					&gw,
				)).To(Succeed())

				cond := meta.FindStatusCondition(gw.Status.Conditions, string(gatewayv1.GatewayConditionProgrammed))
				g.Expect(cond).ToNot(BeNil(), "Programmed condition not yet present")
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(string(gatewayv1.GatewayReasonInvalidParameters)))
			}).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})

		It("does not disrupt traffic through the valid Gateway", func() {
			port := helpers.BuildPortFwdPort(80, portFwdPort)
			coffeeURL := helpers.BuildPortFwdURL("cafe.example.com/coffee", port)

			Eventually(func() error {
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
})
