// This package needs to be named main to get build info
// because of https://github.com/golang/go/issues/33976
package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var _ = Describe("WAFPolicy", Ordered, Label("waf"), func() {
	// WAF requires amd64 and NGINX Plus with NAP WAF images (--waf-enabled=true).
	BeforeAll(func() {
		if runtime.GOARCH == "arm64" {
			Skip("NAP WAF does not support ARM architecture")
		}
		if !*wafEnabled {
			Skip("Skipping WAF tests: --waf-enabled is not set")
		}
	})

	var (
		files = []string{
			"waf-policy/cafe.yaml",
			"waf-policy/bundle-server.yaml",
			"waf-policy/gateway.yaml",
			"waf-policy/cafe-routes.yaml",
		}

		proxyFile = []string{
			"waf-policy/nginx-proxy.yaml",
		}

		namespace    = "waf-policy"
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

		// bundleFiles maps pre-compiled .tgz paths (relative to the repo root, output by
		// make compile-waf-bundles) to the filename each is served as by the bundle server.
		bundleFiles := map[string]string{
			"manifests/waf-policy/dataguard-blocking.tgz":         "dataguard-blocking.tgz",
			"manifests/waf-policy/attack-signatures-blocking.tgz": "attack-signatures-blocking.tgz",
			"manifests/waf-policy/logconf.tgz":                    "logconf.tgz",
		}

		// Copy pre-compiled WAF policy bundles into the bundle-server pod so that the
		// WAFPolicy HTTP source can fetch them during the tests.
		bundleServerPodNames, err := resourceManager.GetPodNames(
			namespace, client.MatchingLabels{"app": "bundle-server"},
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(bundleServerPodNames).To(HaveLen(1), "expected exactly one bundle-server pod")

		for localPath, remoteName := range bundleFiles {
			cpCmd := exec.CommandContext(
				context.Background(),
				"kubectl", "cp",
				localPath,
				fmt.Sprintf("%s/%s:/usr/share/nginx/html/%s", namespace, bundleServerPodNames[0], remoteName),
			)
			cpOut, cpErr := cpCmd.CombinedOutput()
			Expect(cpErr).ToNot(HaveOccurred(), "kubectl cp %s failed: %s", localPath, string(cpOut))
		}

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

	Context("when NginxProxy has WAF enabled", func() {
		It("injects WAF sidecar containers into the NGINX pod", func() {
			ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout)
			defer cancel()

			var pod core.Pod
			Expect(resourceManager.Get(ctx, types.NamespacedName{Name: nginxPodName, Namespace: namespace}, &pod)).
				To(Succeed())

			containerNames := make([]string, 0, len(pod.Spec.Containers))
			for _, c := range pod.Spec.Containers {
				containerNames = append(containerNames, c.Name)
			}

			Expect(containerNames).To(ContainElements("waf-enforcer", "waf-config-mgr"),
				"expected WAF sidecar containers to be present in the NGINX pod")
		})
	})

	Context("when a valid WAFPolicy targeting an existing Gateway is created", func() {
		policyFiles := []string{"waf-policy/wafpolicy.yaml"}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
		})

		It("is accepted by the Gateway", func() {
			nsname := types.NamespacedName{Name: "gateway-waf", Namespace: namespace}
			Expect(waitForWAFPolicyAccepted(nsname)).To(Succeed())
		})

		It("produces the correct NGINX directives", func() {
			conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, nginxCrossplanePath)
			Expect(err).ToNot(HaveOccurred())

			// app_protect directives are set at the server level for gateway-targeted policies.
			wafFile := fmt.Sprintf("WAFPolicy_%s_gateway-waf.conf", namespace)
			expectedFields := []framework.ExpectedNginxField{
				{
					Directive: "app_protect_enable",
					Value:     "on",
					File:      wafFile,
				},
				{
					Directive: "app_protect_policy_file",
					Value:     fmt.Sprintf("/etc/app_protect/bundles/%s_gateway-waf.tgz", namespace),
					File:      wafFile,
				},
				{
					Directive: "app_protect_security_log_enable",
					Value:     "on",
					File:      wafFile,
				},
				{
					// Use substring match: the log bundle filename contains a content-derived hash
					// that may change across compiler versions.
					Directive:             "app_protect_security_log",
					Value:                 fmt.Sprintf("/etc/app_protect/bundles/%s_gateway-waf_log_", namespace),
					File:                  wafFile,
					ValueSubstringAllowed: true,
				},
			}
			for _, field := range expectedFields {
				Expect(framework.ValidateNginxFieldExists(conf, field)).To(Succeed())
			}
		})

		It("blocks requests containing attack signatures", func() {
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			// </script> is a classic XSS payload that the attack-signatures policy blocks.
			attackURL := fmt.Sprintf("http://cafe.example.com:%d/coffee?x=%%3C%%2Fscript%%3E", port)

			Eventually(func() (bool, error) {
				resp, err := framework.Get(framework.Request{
					URL:     attackURL,
					Address: address,
					Timeout: timeoutConfig.RequestTimeout,
				})
				if err != nil {
					return false, err
				}
				return strings.Contains(resp.Body, "Request Rejected"), nil
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500*time.Millisecond).
				Should(BeTrue(), "expected WAF to block XSS attack signature")
		})

		It("allows responses containing sensitive data without a dataguard policy", func() {
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			coffeeURL := fmt.Sprintf("http://cafe.example.com:%d/coffee", port)

			// The attack-signatures policy does not mask response data — SSN passes through.
			Eventually(func() (bool, error) {
				resp, err := framework.Get(framework.Request{
					URL:     coffeeURL,
					Address: address,
					Timeout: timeoutConfig.RequestTimeout,
				})
				if err != nil {
					return false, err
				}
				return strings.Contains(resp.Body, "123-45-6789"), nil
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500*time.Millisecond).
				Should(BeTrue(), "expected SSN to pass through without a dataguard policy")
		})
	})

	Context("when a WAFPolicy targets an HTTPRoute", func() {
		policyFiles := []string{"waf-policy/wafpolicy-route.yaml"}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
		})

		It("is accepted", func() {
			nsname := types.NamespacedName{Name: "coffee-route-waf", Namespace: namespace}
			Expect(waitForWAFPolicyAccepted(nsname)).To(Succeed())
		})

		It("produces WAF directives in the location block for the targeted route", func() {
			conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, nginxCrossplanePath)
			Expect(err).ToNot(HaveOccurred())

			// For an HTTPRoute-targeted policy, directives are in the location block.
			wafFile := fmt.Sprintf("WAFPolicy_%s_coffee-route-waf.conf", namespace)
			expectedFields := []framework.ExpectedNginxField{
				{
					Directive: "app_protect_enable",
					Value:     "on",
					File:      wafFile,
					Location:  "/coffee",
				},
				{
					Directive: "app_protect_policy_file",
					Value:     fmt.Sprintf("/etc/app_protect/bundles/%s_coffee-route-waf.tgz", namespace),
					File:      wafFile,
					Location:  "/coffee",
				},
			}
			for _, field := range expectedFields {
				Expect(framework.ValidateNginxFieldExists(conf, field)).To(Succeed())
			}
		})

		It("masks sensitive data in responses on the protected route", func() {
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			coffeeURL := fmt.Sprintf("http://cafe.example.com:%d/coffee", port)

			// The dataguard policy on the coffee route masks SSN and credit card numbers.
			Eventually(func() (bool, error) {
				resp, err := framework.Get(framework.Request{
					URL:     coffeeURL,
					Address: address,
					Timeout: timeoutConfig.RequestTimeout,
				})
				if err != nil {
					return false, err
				}
				return !strings.Contains(resp.Body, "4111-1111-1111-1111") &&
					!strings.Contains(resp.Body, "123-45-6789"), nil
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500*time.Millisecond).
				Should(BeTrue(), "expected WAF dataguard to mask sensitive data on the coffee route")
		})

		It("allows requests to the unprotected tea route", func() {
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			teaURL := fmt.Sprintf("http://cafe.example.com:%d/tea", port)

			Eventually(func() error {
				return framework.ExpectRequestToSucceed(
					timeoutConfig.RequestTimeout, teaURL, address, "URI: /tea",
				)
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})
	})

	Context("when a WAFPolicy references a nonexistent bundle", func() {
		policyFiles := []string{"waf-policy/wafpolicy-missing-bundle.yaml"}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
		})

		It("has a Programmed=False/Pending condition", func() {
			nsname := types.NamespacedName{Name: "gateway-waf-missing-bundle", Namespace: namespace}
			Expect(waitForWAFPolicyCondition(nsname, "Programmed", metav1.ConditionFalse, "Pending")).To(Succeed())
		})

		It("does not add app_protect directives to NGINX config", func() {
			conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, nginxCrossplanePath)
			Expect(err).ToNot(HaveOccurred())

			err = framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
				Directive: "app_protect_policy_file",
				Value:     fmt.Sprintf("/etc/app_protect/bundles/%s_gateway-waf-missing-bundle.tgz", namespace),
				File:      fmt.Sprintf("WAFPolicy_%s_gateway-waf-missing-bundle.conf", namespace),
			})
			Expect(err).To(HaveOccurred(), "expected no WAF policy directive for missing bundle")
		})
	})

	Context("when a WAFPolicy targets a nonexistent Gateway", func() {
		policyFiles := []string{"waf-policy/invalid-wafpolicy.yaml"}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
		})

		It("is created without error and has no ancestor status", func() {
			// When the target Gateway does not exist, NGF does not process the policy and sets no
			// ancestor status — the policy is silently ignored until its target appears.
			ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout)
			defer cancel()

			var pol ngfAPI.WAFPolicy
			Expect(resourceManager.Get(
				ctx,
				types.NamespacedName{Name: "gateway-waf-invalid", Namespace: namespace},
				&pol,
			)).To(Succeed())

			Expect(pol.Status.Ancestors).To(BeEmpty(),
				"expected no ancestor status for a policy targeting a nonexistent Gateway")
		})
	})

	Context("when a WAFPolicy with polling is applied and the bundle becomes unavailable", Ordered, func() {
		// This context exercises the stale-bundle (fail-open) path:
		// NGF keeps the last successfully fetched bundle active and sets
		// Programmed=True/StaleBundleWarning instead of removing WAF protection.
		policyFiles := []string{"waf-policy/wafpolicy-polling.yaml"}

		var bundleServerPodName string

		BeforeAll(func() {
			bundleServerPodNames, err := resourceManager.GetPodNames(
				namespace, client.MatchingLabels{"app": "bundle-server"},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(bundleServerPodNames).To(HaveLen(1))
			bundleServerPodName = bundleServerPodNames[0]

			Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
		})

		AfterAll(func() {
			// Restore the bundle so other tests are not affected.
			cpCmd := exec.CommandContext( //nolint:gosec // not a subprocess launched with tainted input
				context.Background(),
				"kubectl", "cp",
				"manifests/waf-policy/attack-signatures-blocking.tgz",
				fmt.Sprintf("%s/%s:/usr/share/nginx/html/attack-signatures-blocking.tgz", namespace, bundleServerPodName),
			)
			cpOut, cpErr := cpCmd.CombinedOutput()
			Expect(cpErr).ToNot(HaveOccurred(), "kubectl cp to restore bundle failed: %s", string(cpOut))

			Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
		})

		It("is accepted and enforces WAF while the bundle is available", func() {
			nsname := types.NamespacedName{Name: "gateway-waf-polling", Namespace: namespace}
			Expect(waitForWAFPolicyAccepted(nsname)).To(Succeed())

			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			attackURL := fmt.Sprintf("http://cafe.example.com:%d/coffee?x=%%3C%%2Fscript%%3E", port)

			Eventually(func() (bool, error) {
				resp, err := framework.Get(framework.Request{
					URL:     attackURL,
					Address: address,
					Timeout: timeoutConfig.RequestTimeout,
				})
				if err != nil {
					return false, err
				}
				return strings.Contains(resp.Body, "Request Rejected"), nil
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500*time.Millisecond).
				Should(BeTrue(), "expected WAF to be active before bundle removal")
		})

		It("transitions to Programmed=True/StaleBundleWarning after the bundle is removed and keeps WAF active", func() {
			// Remove the bundle from the server so the next poll fetch returns 404.
			ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.RequestTimeout)
			defer cancel()
			_, err := resourceManager.ExecInPod(
				ctx,
				namespace,
				bundleServerPodName,
				"",
				[]string{"rm", "-f", "/usr/share/nginx/html/attack-signatures-blocking.tgz"},
			)
			Expect(err).ToNot(HaveOccurred(), "failed to remove bundle from server")

			// Wait for the poller to attempt re-fetch (interval is 15s) and set the stale warning.
			// Allow up to 45s: one full interval plus generous processing time.
			nsname := types.NamespacedName{Name: "gateway-waf-polling", Namespace: namespace}
			Expect(waitForWAFPolicyCondition(
				nsname, "Programmed", metav1.ConditionTrue, "StaleBundleWarning",
				45*time.Second,
			)).To(Succeed())

			// Confirm WAF is still enforcing with the stale bundle — XSS should still be blocked.
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			attackURL := fmt.Sprintf("http://cafe.example.com:%d/coffee?x=%%3C%%2Fscript%%3E", port)

			Eventually(func() (bool, error) {
				resp, err := framework.Get(framework.Request{
					URL:     attackURL,
					Address: address,
					Timeout: timeoutConfig.RequestTimeout,
				})
				if err != nil {
					return false, err
				}
				return strings.Contains(resp.Body, "Request Rejected"), nil
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500*time.Millisecond).
				Should(BeTrue(), "expected WAF to remain active using stale bundle after fetch failure")
		})
	})

	Context("when a WAFPolicy is deleted", Ordered, func() {
		policyFiles := []string{"waf-policy/wafpolicy.yaml"}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
			nsname := types.NamespacedName{Name: "gateway-waf", Namespace: namespace}
			Expect(waitForWAFPolicyAccepted(nsname)).To(Succeed())
		})

		It("removes WAF directives from the NGINX config after deletion", func() {
			Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())

			Eventually(func() error {
				conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, nginxCrossplanePath)
				if err != nil {
					return err
				}
				err = framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "app_protect_policy_file",
					Value:     fmt.Sprintf("/etc/app_protect/bundles/%s_gateway-waf.tgz", namespace),
					File:      fmt.Sprintf("WAFPolicy_%s_gateway-waf.conf", namespace),
				})
				if err == nil {
					return fmt.Errorf("app_protect_policy_file directive still present after policy deletion")
				}
				return nil
			}).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500*time.Millisecond).
				Should(Succeed(), "expected WAF directives to be removed after policy deletion")
		})

		It("continues to serve traffic after WAF policy removal", func() {
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			coffeeURL := fmt.Sprintf("http://cafe.example.com:%d/coffee", port)

			Eventually(func() error {
				return framework.ExpectRequestToSucceed(
					timeoutConfig.RequestTimeout, coffeeURL, address, "Customer List",
				)
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500*time.Millisecond).
				Should(Succeed(), "expected traffic to continue after WAF policy removal")
		})
	})
})

// waitForWAFPolicyAccepted polls until the WAFPolicy has Accepted/True/Accepted.
func waitForWAFPolicyAccepted(nsname types.NamespacedName) error {
	return waitForWAFPolicyAncestorStatus(nsname, metav1.ConditionTrue, v1.PolicyReasonAccepted)
}

// waitForWAFPolicyCondition polls until the WAFPolicy ancestor has a condition of the given type,
// status, and reason. Pass 0 for timeout to use the default GetStatusTimeout.
func waitForWAFPolicyCondition(
	nsname types.NamespacedName,
	condType string,
	condStatus metav1.ConditionStatus,
	reason string,
	timeout ...time.Duration,
) error {
	d := timeoutConfig.GetStatusTimeout
	if len(timeout) > 0 && timeout[0] > 0 {
		d = timeout[0]
	}
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	GinkgoWriter.Printf(
		"Waiting for WAFPolicy %q to have condition %s/%s/%s\n",
		nsname, condType, condStatus, reason,
	)

	return wait.PollUntilContextCancel(ctx, 500*time.Millisecond, true,
		func(ctx context.Context) (bool, error) {
			var pol ngfAPI.WAFPolicy
			if err := resourceManager.Get(ctx, nsname, &pol); err != nil {
				return false, err
			}

			if len(pol.Status.Ancestors) == 0 {
				GinkgoWriter.Printf("WAFPolicy %q has no ancestor status yet\n", nsname)
				return false, nil
			}

			for _, cond := range pol.Status.Ancestors[0].Conditions {
				if cond.Type != condType {
					continue
				}
				if string(cond.Status) == string(condStatus) && cond.Reason == reason {
					return true, nil
				}
				GinkgoWriter.Printf(
					"WAFPolicy %q condition %s is %s/%s, waiting for %s/%s\n",
					nsname, condType, cond.Status, cond.Reason, condStatus, reason,
				)
				return false, nil
			}

			GinkgoWriter.Printf("WAFPolicy %q has no %s condition yet\n", nsname, condType)
			return false, nil
		},
	)
}

// waitForWAFPolicyAncestorStatus polls until the WAFPolicy ancestor status has the given condition.
func waitForWAFPolicyAncestorStatus(
	nsname types.NamespacedName,
	condStatus metav1.ConditionStatus,
	condReason v1.PolicyConditionReason,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout)
	defer cancel()

	GinkgoWriter.Printf(
		"Waiting for WAFPolicy %q to have condition Accepted/%s/%s\n",
		nsname, condStatus, condReason,
	)

	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var pol ngfAPI.WAFPolicy
			if err := resourceManager.Get(ctx, nsname, &pol); err != nil {
				return false, err
			}

			if len(pol.Status.Ancestors) == 0 {
				GinkgoWriter.Printf("WAFPolicy %q has no ancestor status yet\n", nsname)
				return false, nil
			}

			ancestor := pol.Status.Ancestors[0]

			if ancestor.ControllerName != framework.NgfControllerName {
				return false, fmt.Errorf(
					"expected controller name %s, got %s",
					framework.NgfControllerName,
					ancestor.ControllerName,
				)
			}

			for _, cond := range ancestor.Conditions {
				if cond.Type != string(v1.PolicyConditionAccepted) {
					continue
				}
				if cond.Status == condStatus && cond.Reason == string(condReason) {
					return true, nil
				}
				return false, fmt.Errorf(
					"Accepted condition is %s/%s, expected %s/%s",
					cond.Status, cond.Reason, condStatus, condReason,
				)
			}

			GinkgoWriter.Printf("WAFPolicy %q has no Accepted condition yet\n", nsname)
			return false, nil
		},
	)
}
