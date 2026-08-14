package main

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

// This suite verifies the NginxProxy-level zoneSize setting by checking the value present in the UpstreamSettingsPolicy
// and in the NginxProxy. If the zoneSize is set in the UpstreamSettingsPolicy, it should override the NginxProxy-level
// setting. Otherwise, the NginxProxy-level setting should be used. If neither are set, the default NGINX values should
// be used.
var _ = Describe("NginxProxy zoneSize", Ordered, Label("functional", "nginxproxy-zonesize"), func() {
	var (
		// The NginxProxy is applied before the Gateway so the Gateway's infrastructure.parametersRef resolves.
		proxyFile = []string{"nginxproxy-zonesize/nginx-proxy.yaml"}
		files     = []string{
			"nginxproxy-zonesize/cafe.yaml",
			"nginxproxy-zonesize/gateway.yaml",
			"nginxproxy-zonesize/routes.yaml",
		}
		policies = []string{"nginxproxy-zonesize/upstream.yaml"}

		namespace = "zonesize"

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
		Expect(resourceManager.ApplyFromFiles(policies, namespace)).To(Succeed())
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
		Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
		Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
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

	Context("uspZoneSize is set", func() {
		It("uses the upstreamSettingsPolicy zone size when set", func() {
			Eventually(
				func() error {
					conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, "")
					if err != nil {
						return err
					}

					return framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "zone",
						Value:     "zonesize_coffee_80 2m",
						Upstream:  "zonesize_coffee_80",
						File:      "http.conf",
					})
				}).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})
	})
})
