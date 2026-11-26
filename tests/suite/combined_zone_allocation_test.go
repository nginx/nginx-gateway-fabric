package main

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

// This suite verifies that a large rate limit zone (limit_req_zone) and a large proxy cache zone
// (proxy_cache_path keys_zone) can be allocated at the same time without interfering with each
// other, and that NGINX reliably reloads and serves traffic with both in place. The two zones are
// independent NGINX shared memory zones with distinct names, so the risk is not a name collision in
// the generated config but whether NGINX can allocate both large zones and reload successfully.
var _ = Describe("CombinedZoneAllocation", Ordered, Label("functional", "combined-zone-allocation"), func() {
	var (
		files = []string{
			"combined-zone-allocation/app.yaml",
			"combined-zone-allocation/gateway.yaml",
			"combined-zone-allocation/routes.yaml",
		}

		policies = []string{
			"combined-zone-allocation/policies.yaml",
		}

		// rateLimitZoneSize and cacheKeysZoneSize are intentionally large so that the test exercises
		// the combined memory footprint of both zones. They must stay well within the NGINX pod's
		// available memory: NGINX reserves the full size of each zone up front, so the goal is to
		// confirm NGINX can manage both allocations at once, not to deliberately exhaust memory.
		rateLimitZoneSize = "256m"
		cacheKeysZoneSize = "256m"

		namespace    string
		filePrefix   string
		nginxPodName string
	)

	BeforeAll(func() {
		namespace = fmt.Sprintf("combined-zone-allocation-%d", GinkgoParallelProcess())
		filePrefix = fmt.Sprintf("/etc/nginx/includes/RateLimitPolicy_%s", namespace)

		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())

		nginxPodNames, err := resourceManager.GetReadyNginxPodNames(namespace, timeoutConfig.GetStatusTimeout)
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

	When("a large rate limit zone and a large proxy cache zone are configured together", func() {
		var baseCoffeeURL string

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policies, namespace)).To(Succeed())

			port := helpers.BuildPortFwdPort(80, portFwdPort)
			baseCoffeeURL = helpers.BuildPortFwdURL("cafe.example.com/coffee", port)
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
		})

		Specify("both policies are accepted and affect their targets", func() {
			Expect(waitForRateLimitPolicyToBeAccepted(
				types.NamespacedName{Name: "combined-zone-rate-limit", Namespace: namespace},
			)).To(Succeed(), "combined-zone-rate-limit was not accepted")

			Expect(resourceManager.WaitForPolicyStatus(
				types.NamespacedName{Name: "combined-zone-caching", Namespace: namespace},
				&ngfAPI.ProxySettingsPolicy{},
				framework.PolicyAccepted,
				framework.AncestorsMatchTargetRefs,
			)).To(Succeed(), "combined-zone-caching was not accepted")

			Expect(resourceManager.WaitForHTTPRoutePolicyAffected(
				types.NamespacedName{Name: "coffee", Namespace: namespace},
				string(conditions.RateLimitPolicyAffected),
				timeoutConfig.GetStatusTimeout,
			)).To(Succeed())

			Expect(resourceManager.WaitForGatewayPolicyAffected(
				types.NamespacedName{Name: "gateway", Namespace: namespace},
				string(conditions.ProxySettingsPolicyAffected),
				timeoutConfig.GetStatusTimeout,
			)).To(Succeed())
		})

		Specify("NGINX reloads successfully and the pod stays ready with both zones allocated", func() {
			// Acceptance of a policy does not by itself prove NGINX reloaded: the control plane can
			// report Accepted while NGINX is still applying the config. Re-polling for a ready NGINX
			// pod confirms NGINX actually reserved both large zones and reloaded without failing.
			nginxPodNames, err := resourceManager.GetReadyNginxPodNames(namespace, timeoutConfig.GetStatusTimeout)
			Expect(err).ToNot(HaveOccurred())
			Expect(nginxPodNames).To(ContainElement(nginxPodName))
		})

		Context("verify working traffic", func() {
			It("serves traffic on the rate-limited, cache-enabled route", func() {
				Eventually(func() error {
					return framework.ExpectRequestToSucceed(
						timeoutConfig.RequestTimeout, baseCoffeeURL, address, "URI: /coffee",
					)
				}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500*time.Millisecond).
					Should(Succeed(), "route %s did not serve traffic with both zones configured", baseCoffeeURL)
			})
		})

		Context("nginx config", func() {
			var conf *framework.Payload

			BeforeAll(func() {
				conf = framework.MustGetNginxConfig(&resourceManager, nginxPodName, namespace)
			})

			It("allocates both the large rate limit zone and the large cache zone with distinct names", func() {
				// The cache keys_zone lives in the base http.conf; its name is fixed by NGF.
				Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive:             "proxy_cache_path",
					Value:                 fmt.Sprintf("keys_zone=nginx_gateway_cache:%s", cacheKeysZoneSize),
					File:                  "http.conf",
					ValueSubstringAllowed: true,
				})).To(Succeed())

				// The rate limit zone lives in the policy include file; its name is namespaced.
				Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "limit_req_zone",
					Value: fmt.Sprintf(
						"$binary_remote_addr zone=%s_rl_combined-zone-rate-limit_rule0:%s rate=100r/s",
						namespace, rateLimitZoneSize,
					),
					File: fmt.Sprintf("%s_combined-zone-rate-limit_internal_http.conf", filePrefix),
				})).To(Succeed())

				// The cache zone is actually referenced (not just defined), so NGINX validates it.
				cacheFilePrefix := fmt.Sprintf("/etc/nginx/includes/ProxySettingsPolicy_%s", namespace)
				Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "proxy_cache",
					Value:     "nginx_gateway_cache",
					File:      fmt.Sprintf("%s_combined-zone-caching.conf", cacheFilePrefix),
				})).To(Succeed())
			})
		})
	})
})
