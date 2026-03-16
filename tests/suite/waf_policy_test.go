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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/plm"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

// certManagerVersion is the version of cert-manager to install for SeaweedFS TLS.
// renovate: datasource=github-releases depName=cert-manager/cert-manager
const certManagerVersion = "v1.19.4"

var _ = Describe("WAFGatewayBindingPolicy", Ordered, Label("waf"), func() {
	// WAF requires amd64 and must be run with --waf-enabled.
	BeforeAll(func() {
		if runtime.GOARCH == "arm64" {
			Skip("NAP WAF does not support ARM architecture")
		}
		if !*wafEnabled {
			Skip("WAF tests require --waf-enabled (run with make test-waf)")
		}
	})

	var (
		baseFiles = []string{
			"waf-policy/cafe.yaml",
			"waf-policy/ap-logconf.yaml",
			"waf-policy/appolicy.yaml",
			"waf-policy/appolicy-strict.yaml",
		}
		gatewayFiles = []string{
			"waf-policy/nginx-proxy.yaml",
			"waf-policy/gateway.yaml",
			"waf-policy/cafe-routes.yaml",
		}

		namespace    = "waf-policy"
		nginxPodName string
	)

	BeforeAll(func() {
		// Install cert-manager and create SeaweedFS TLS secrets BEFORE installing NGF,
		// so the secrets exist when the PLM chart's SeaweedFS StatefulSets start up.
		// The NGF namespace must be created first since Helm's --create-namespace races
		// with the cert-manager Certificate resources that need to write into it.
		installCertManager()
		nsCmd := exec.CommandContext(context.Background(), "kubectl", "apply", "-f", "-")
		nsCmd.Stdin = strings.NewReader(fmt.Sprintf(
			"apiVersion: v1\nkind: Namespace\nmetadata:\n  name: %s\n", ngfNamespace,
		))
		nsOut, err := nsCmd.CombinedOutput()
		Expect(err).ToNot(HaveOccurred(), string(nsOut))
		applyCertManagerSeaweedFSCerts(ngfNamespace, releaseName)

		// Install NGF with the f5-waf-plm subchart enabled.
		installNGFWithPLM()

		// Create the test namespace and apply base resources.
		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(baseFiles, namespace)).To(Succeed())

		// Wait for PLM to compile the AP* resources before deploying the Gateway.
		Expect(waitForAPPolicyReady(types.NamespacedName{Name: "dataguard-blocking", Namespace: namespace})).
			To(Succeed())
		Expect(waitForAPPolicyReady(types.NamespacedName{Name: "attack-signatures-blocking", Namespace: namespace})).
			To(Succeed())
		Expect(waitForAPLogConfReady(types.NamespacedName{Name: "logconf", Namespace: namespace})).
			To(Succeed())

		// Apply Gateway resources — WAF sidecars are injected because NginxProxy has waf: "enabled".
		Expect(resourceManager.ApplyFromFiles(gatewayFiles, namespace)).To(Succeed())
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

		uninstallNGFWithPLM()
		uninstallCertManager()
	})

	Context("WAF sidecar containers", func() {
		It("are injected into the NGINX pod when NginxProxy has waf enabled", func() {
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

	Context("when a valid WAFGatewayBindingPolicy targeting an existing Gateway is created", func() {
		policyFiles := []string{"waf-policy/wgbpolicy.yaml"}

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

			// app_protect_enable is set at the server level for gateway-targeted policies
			Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
				Directive: "app_protect_enable",
				Value:     "on",
				File:      fmt.Sprintf("WAFGatewayBindingPolicy_%s_gateway-waf.conf", namespace),
			})).To(Succeed())

			Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
				Directive: "app_protect_policy_file",
				Value:     fmt.Sprintf("/etc/app_protect/bundles/%s_dataguard-blocking.tgz", namespace),
				File:      fmt.Sprintf("WAFGatewayBindingPolicy_%s_gateway-waf.conf", namespace),
			})).To(Succeed())

			Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
				Directive: "app_protect_security_log_enable",
				Value:     "on",
				File:      fmt.Sprintf("WAFGatewayBindingPolicy_%s_gateway-waf.conf", namespace),
			})).To(Succeed())

			Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
				Directive: "app_protect_security_log",
				Value: fmt.Sprintf(
					"/etc/app_protect/bundles/%s_logconf.tgz stderr",
					namespace,
				),
				File: fmt.Sprintf("WAFGatewayBindingPolicy_%s_gateway-waf.conf", namespace),
			})).To(Succeed())
		})

		It("blocks responses containing sensitive data", func() {
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			coffeeURL := fmt.Sprintf("http://cafe.example.com:%d/coffee", port)

			// The coffee backend returns a response containing a credit card and SSN.
			// The dataguard-blocking policy should intercept and replace this response.
			Eventually(func() (bool, error) {
				resp, err := framework.Get(framework.Request{
					URL:     coffeeURL,
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
				Should(BeTrue(), "expected WAF to block response containing sensitive data")
		})

		It("does not disrupt routing to unprotected routes", func() {
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			teaURL := fmt.Sprintf("http://cafe.example.com:%d/tea", port)

			Eventually(func() error {
				return framework.ExpectRequestToSucceed(timeoutConfig.RequestTimeout, teaURL, address, "tea-response")
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})
	})

	Context("when the WAFGatewayBindingPolicy is removed", func() {
		policyFiles := []string{"waf-policy/wgbpolicy.yaml"}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
			// Wait until policy is active (directives present) before testing removal.
			nsname := types.NamespacedName{Name: "gateway-waf", Namespace: namespace}
			Expect(waitForWAFPolicyAccepted(nsname)).To(Succeed())
		})

		AfterAll(func() {
			// Re-apply for any subsequent contexts that expect it.
			Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
		})

		It("removes app_protect directives from NGINX config", func() {
			Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())

			// app_protect directives must be gone after the policy is deleted.
			Eventually(func() error {
				conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, nginxCrossplanePath)
				if err != nil {
					return err
				}
				err = framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "app_protect_enable",
					Value:     "on",
					File:      fmt.Sprintf("WAFGatewayBindingPolicy_%s_gateway-waf.conf", namespace),
				})
				if err == nil {
					return fmt.Errorf("app_protect_enable still present after policy deletion")
				}
				return nil
			}).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})

		It("allows sensitive data through once policy is removed", func() {
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			coffeeURL := fmt.Sprintf("http://cafe.example.com:%d/coffee", port)

			Eventually(func() error {
				return framework.ExpectRequestToSucceed(
					timeoutConfig.RequestTimeout, coffeeURL, address, "Welcome to Coffee",
				)
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})
	})

	Context("when a WAFGatewayBindingPolicy targets an HTTPRoute", func() {
		policyFiles := []string{"waf-policy/wgbpolicy-route.yaml"}

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
			Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
				Directive: "app_protect_enable",
				Value:     "on",
				File:      fmt.Sprintf("WAFGatewayBindingPolicy_%s_coffee-route-waf.conf", namespace),
				Location:  "/coffee",
			})).To(Succeed())

			Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
				Directive: "app_protect_policy_file",
				Value:     fmt.Sprintf("/etc/app_protect/bundles/%s_attack-signatures-blocking.tgz", namespace),
				File:      fmt.Sprintf("WAFGatewayBindingPolicy_%s_coffee-route-waf.conf", namespace),
				Location:  "/coffee",
			})).To(Succeed())
		})

		It("blocks requests containing attack signatures on the protected route", func() {
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			// Send a request with a SQL injection attack in the query string.
			// The attack-signatures-blocking policy should detect and block this.
			attackURL := fmt.Sprintf("http://cafe.example.com:%d/coffee?id=1%%27+OR+1%%3D1--", port)

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
				Should(BeTrue(), "expected WAF to block SQL injection attack on the coffee route")
		})

		It("does not affect the unprotected tea route", func() {
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			teaURL := fmt.Sprintf("http://cafe.example.com:%d/tea", port)

			Eventually(func() error {
				return framework.ExpectRequestToSucceed(
					timeoutConfig.RequestTimeout, teaURL, address, "tea-response",
				)
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})
	})

	Context("when a WAFGatewayBindingPolicy references a nonexistent APPolicy", func() {
		policyFiles := []string{"waf-policy/wgbpolicy-missing-appolicy.yaml"}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
		})

		It("has a ResolvedRefs=False/InvalidRef condition", func() {
			nsname := types.NamespacedName{Name: "gateway-waf-missing-appolicy", Namespace: namespace}
			Expect(waitForWAFPolicyResolvedRefs(nsname, metav1.ConditionFalse, "InvalidRef")).To(Succeed())
		})

		It("does not add app_protect directives to NGINX config", func() {
			conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, nginxCrossplanePath)
			Expect(err).ToNot(HaveOccurred())

			err = framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
				Directive: "app_protect_policy_file",
				Value:     fmt.Sprintf("/etc/app_protect/bundles/%s_nonexistent-appolicy.tgz", namespace),
				File:      fmt.Sprintf("WAFGatewayBindingPolicy_%s_gateway-waf-missing-appolicy.conf", namespace),
			})
			Expect(err).To(HaveOccurred(), "expected no WAF policy directive for missing APPolicy")
		})
	})

	Context("when an APPolicy is updated to a new valid policy", func() {
		// Start with the strict attack-signatures policy, then switch to dataguard-blocking
		// and verify the new bundle is compiled and applied.
		policyFiles := []string{"waf-policy/wgbpolicy-route.yaml"}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
			Expect(waitForWAFPolicyAccepted(types.NamespacedName{Name: "coffee-route-waf", Namespace: namespace})).
				To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
		})

		It("updates the bundle in NGINX config after the APPolicy change", func() {
			// Switch attack-signatures-blocking to use VIOL_DATA_GUARD instead, making it
			// functionally equivalent to dataguard-blocking. This gives us a new compiled bundle.
			updatedPolicy := `apiVersion: appprotect.f5.com/v1
kind: APPolicy
metadata:
  name: attack-signatures-blocking
spec:
  policy:
    name: attack_signatures_blocking_updated
    template:
      name: POLICY_TEMPLATE_NGINX_BASE
    applicationLanguage: utf-8
    enforcementMode: blocking
    blocking-settings:
      violations:
      - name: VIOL_DATA_GUARD
        alarm: true
        block: true
    data-guard:
      enabled: true
      maskData: true
      creditCardNumbers: true
      usSocialSecurityNumbers: true
      enforcementMode: ignore-urls-in-list
      enforcementUrls: []
`
			cmd := exec.CommandContext(context.Background(), "kubectl", "apply", "-f", "-", "-n", namespace)
			cmd.Stdin = strings.NewReader(updatedPolicy)
			out, err := cmd.CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(out))

			// Wait for PLM to recompile the updated policy.
			Expect(waitForAPPolicyReady(
				types.NamespacedName{Name: "attack-signatures-blocking", Namespace: namespace},
			)).To(Succeed())

			// NGINX config should still reference the same bundle path (name unchanged).
			Eventually(func() error {
				conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, nginxCrossplanePath)
				if err != nil {
					return err
				}
				return framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "app_protect_policy_file",
					Value:     fmt.Sprintf("/etc/app_protect/bundles/%s_attack-signatures-blocking.tgz", namespace),
					File:      fmt.Sprintf("WAFGatewayBindingPolicy_%s_coffee-route-waf.conf", namespace),
					Location:  "/coffee",
				})
			}).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})

		It("applies the updated policy behavior to traffic", func() {
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			coffeeURL := fmt.Sprintf("http://cafe.example.com:%d/coffee", port)

			// After switching to data-guard, the coffee response with CC/SSN should now be blocked.
			Eventually(func() (bool, error) {
				resp, err := framework.Get(framework.Request{
					URL:     coffeeURL,
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
				Should(BeTrue(), "expected updated WAF policy to block sensitive data in response")
		})

		AfterAll(func() {
			// Restore attack-signatures-blocking to its original form.
			Expect(resourceManager.ApplyFromFiles([]string{"waf-policy/appolicy-strict.yaml"}, namespace)).To(Succeed())
			Expect(waitForAPPolicyReady(
				types.NamespacedName{Name: "attack-signatures-blocking", Namespace: namespace},
			)).To(Succeed())
		})
	})

	Context("when a WAFGatewayBindingPolicy targets a nonexistent Gateway", func() {
		policyFiles := []string{"waf-policy/invalid-wgbpolicy.yaml"}

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

			var pol ngfAPI.WAFGatewayBindingPolicy
			Expect(resourceManager.Get(
				ctx,
				types.NamespacedName{Name: "gateway-waf-invalid", Namespace: namespace},
				&pol,
			)).To(Succeed())

			Expect(pol.Status.Ancestors).To(BeEmpty(),
				"expected no ancestor status for a policy targeting a nonexistent Gateway")
		})
	})
})

// installCertManager installs cert-manager for SeaweedFS TLS certificate generation.
func installCertManager() {
	GinkgoWriter.Printf("Installing cert-manager %s\n", certManagerVersion)

	url := fmt.Sprintf(
		"https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.yaml",
		certManagerVersion,
	)
	out, err := exec.CommandContext(
		context.Background(),
		"kubectl", "apply", "--server-side", "--force-conflicts", "-f", url,
	).CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), string(out))

	// Wait for cert-manager webhook to be ready before creating Certificate resources.
	out, err = exec.CommandContext(
		context.Background(),
		"kubectl", "rollout", "status", "deployment/cert-manager-webhook",
		"-n", "cert-manager", "--timeout=3m",
	).CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), string(out))
}

// uninstallCertManager uninstalls cert-manager.
func uninstallCertManager() {
	GinkgoWriter.Printf("Uninstalling cert-manager\n")

	url := fmt.Sprintf(
		"https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.yaml",
		certManagerVersion,
	)
	out, err := exec.CommandContext(
		context.Background(),
		"kubectl", "delete", "-f", url, "--ignore-not-found",
	).CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), string(out))
}

// applyCertManagerSeaweedFSCerts creates the cert-manager Issuer and Certificate resources that
// generate TLS secrets for SeaweedFS. Secret names are derived from the Helm release name to match
// what the PLM chart expects: <release>-f5-waf-seaweedfs-<component>-cert.
func applyCertManagerSeaweedFSCerts(namespace, release string) {
	prefix := release + "-f5-waf-seaweedfs"

	manifests := fmt.Sprintf(`apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: %[1]s-issuer
  namespace: %[2]s
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: %[1]s-ca-cert
  namespace: %[2]s
spec:
  secretName: %[1]s-ca-cert
  commonName: "seaweedfs-root-ca"
  isCA: true
  issuerRef:
    name: %[1]s-issuer
    kind: Issuer
  duration: 87600h
  renewBefore: 720h
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: %[1]s-ca-issuer
  namespace: %[2]s
spec:
  ca:
    secretName: %[1]s-ca-cert
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: %[1]s-master-cert
  namespace: %[2]s
spec:
  secretName: %[1]s-master-cert
  issuerRef:
    name: %[1]s-ca-issuer
    kind: Issuer
  commonName: "SeaweedFS CA"
  dnsNames:
    - '*.%[2]s'
    - '*.%[2]s.svc'
    - '*.%[2]s.svc.cluster.local'
  privateKey:
    algorithm: RSA
    size: 2048
  duration: 2160h
  renewBefore: 360h
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: %[1]s-volume-cert
  namespace: %[2]s
spec:
  secretName: %[1]s-volume-cert
  issuerRef:
    name: %[1]s-ca-issuer
    kind: Issuer
  commonName: "SeaweedFS CA"
  dnsNames:
    - '*.%[2]s'
    - '*.%[2]s.svc'
    - '*.%[2]s.svc.cluster.local'
  privateKey:
    algorithm: RSA
    size: 2048
  duration: 2160h
  renewBefore: 360h
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: %[1]s-filer-cert
  namespace: %[2]s
spec:
  secretName: %[1]s-filer-cert
  issuerRef:
    name: %[1]s-ca-issuer
    kind: Issuer
  commonName: "SeaweedFS CA"
  dnsNames:
    - '*.%[2]s'
    - '*.%[2]s.svc'
    - '*.%[2]s.svc.cluster.local'
  privateKey:
    algorithm: RSA
    size: 2048
  duration: 2160h
  renewBefore: 360h
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: %[1]s-client-cert
  namespace: %[2]s
spec:
  secretName: %[1]s-client-cert
  issuerRef:
    name: %[1]s-ca-issuer
    kind: Issuer
  commonName: "SeaweedFS CA"
  dnsNames:
    - '*.%[2]s'
    - '*.%[2]s.svc'
    - '*.%[2]s.svc.cluster.local'
    - client
  privateKey:
    algorithm: RSA
    size: 2048
  duration: 2160h
  renewBefore: 360h
`, prefix, namespace)

	// The cert-manager webhook may not be fully ready (TLS cert provisioned) immediately after
	// the deployment rollout completes. Retry until it accepts requests.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	Eventually(func() error {
		cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-", "--namespace", namespace)
		cmd.Stdin = strings.NewReader(manifests)
		out, err := cmd.CombinedOutput()
		GinkgoWriter.Printf("applyCertManagerSeaweedFSCerts output: %s\n", string(out))
		return err
	}).WithContext(ctx).WithPolling(5 * time.Second).Should(Succeed())
}

// installNGFWithPLM installs NGF with the f5-waf-plm subchart enabled.
// It mirrors createNGFInstallConfig but adds WAF-specific fields.
func installNGFWithPLM() {
	GinkgoWriter.Printf("Installing NGF with PLM subchart\n")

	setupCfg := getDefaultSetupCfg()
	installCfg := framework.InstallationConfig{
		ReleaseName:          setupCfg.releaseName,
		Namespace:            ngfNamespace,
		ChartPath:            setupCfg.chartPath,
		ServiceType:          *serviceType,
		Plus:                 *plusEnabled,
		PlusUsageEndpoint:    *plusUsageEndpoint,
		WAFEnabled:           true,
		NginxImagePullSecret: *nginxImagePullSecret,
		PLMImagePullSecret:   *plmImagePullSecret,
	}

	// Set custom image repos when installing from a local chart.
	if !strings.HasPrefix(setupCfg.chartPath, "oci://") {
		installCfg.NgfImageRepository = *ngfImageRepository
		// WAF requires NGINX Plus with NAP, so always use the Plus image repo.
		installCfg.NginxImageRepository = *nginxPlusImageRepository
		installCfg.ImageTag = *imageTag
		installCfg.ImagePullPolicy = *imagePullPolicy
	}

	output, err := framework.InstallGatewayAPI(*gatewayAPIVersion)
	Expect(err).ToNot(HaveOccurred(), string(output))

	if *plusEnabled {
		Expect(framework.CreateLicenseSecret(resourceManager, ngfNamespace, *plusLicenseFileName)).To(Succeed())
	}

	output, err = framework.InstallNGF(installCfg,
		"--set", "nginxGateway.config.logging.level=debug",
		"--set", "nginx.config.logging.agentLevel=debug",
		"--set", fmt.Sprintf("f5-waf-plm.seaweedfs-operator.image.pullSecrets=%s", *plmImagePullSecret),
	)
	Expect(err).ToNot(HaveOccurred(), string(output))

	_, err = resourceManager.GetReadyNGFPodNames(ngfNamespace, releaseName, timeoutConfig.CreateTimeout)
	Expect(err).ToNot(HaveOccurred())
}

// uninstallNGFWithPLM uninstalls NGF and the PLM subchart.
func uninstallNGFWithPLM() {
	GinkgoWriter.Printf("Uninstalling NGF with PLM subchart\n")

	// Remove finalizers from PLM-managed CRDs that block namespace deletion.
	// The PLM controller is stopped by helm uninstall before it can process its own finalizers.
	for _, resource := range []string{"apsignatures", "appolicies", "aplogconfs"} {
		out, err := exec.CommandContext(
			context.Background(),
			"kubectl", "get", resource, "-A",
			"-o", "jsonpath={range .items[*]}{.metadata.namespace}/{.metadata.name}{'\\n'}{end}",
		).Output()
		if err != nil {
			GinkgoWriter.Printf("Could not list %s (may not exist): %v\n", resource, err)
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "/", 2)
			if len(parts) != 2 {
				continue
			}
			ns, name := parts[0], parts[1]
			patchOut, patchErr := exec.CommandContext(
				context.Background(),
				"kubectl", "patch", resource, name, "-n", ns,
				"--type=merge", "-p", `{"metadata":{"finalizers":[]}}`,
			).CombinedOutput()
			GinkgoWriter.Printf("Removed finalizers from %s %s/%s: %s\n", resource, ns, name, string(patchOut))
			if patchErr != nil {
				GinkgoWriter.Printf("Warning: could not remove finalizers from %s %s/%s: %v\n", resource, ns, name, patchErr)
			}
		}
	}

	teardown(releaseName)
}

// waitForAPPolicyReady polls until the APPolicy has bundle state "ready".
func waitForAPPolicyReady(nsname types.NamespacedName) error {
	return waitForAPResourceReady(nsname, plm.NewAPPolicyUnstructured())
}

// waitForAPLogConfReady polls until the APLogConf has bundle state "ready".
func waitForAPLogConfReady(nsname types.NamespacedName) error {
	return waitForAPResourceReady(nsname, plm.NewAPLogConfUnstructured())
}

// waitForAPResourceReady polls until an AP* resource has bundle state "ready".
func waitForAPResourceReady(nsname types.NamespacedName, obj *unstructured.Unstructured) error {
	// PLM compilation can take several minutes on first run.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	kind := obj.GetKind()
	GinkgoWriter.Printf("Waiting for %s %q to have bundle state %q\n", kind, nsname, plm.StateReady)

	return wait.PollUntilContextCancel(
		ctx,
		5*time.Second,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			fetched := obj.DeepCopy()
			if err := resourceManager.Get(ctx, nsname, fetched); err != nil {
				return false, err
			}

			status, err := plm.ExtractAPResourceStatus(fetched)
			if err != nil {
				return false, err
			}

			GinkgoWriter.Printf("%s %q bundle state: %q\n", kind, nsname, status.Bundle.State)

			switch status.Bundle.State {
			case plm.StateReady:
				return true, nil
			case plm.StateInvalid:
				return false, fmt.Errorf(
					"%s %q bundle is invalid: %v", kind, nsname, status.Processing.Errors,
				)
			default:
				return false, nil
			}
		},
	)
}

// waitForWAFPolicyAccepted polls until the WAFGatewayBindingPolicy has Accepted/True/Accepted.
func waitForWAFPolicyAccepted(nsname types.NamespacedName) error {
	return waitForWAFPolicyAncestorStatus(nsname, metav1.ConditionTrue, v1.PolicyReasonAccepted)
}

// waitForWAFPolicyResolvedRefs polls until the WAFGatewayBindingPolicy ancestor has a ResolvedRefs
// condition with the given status and reason. NGF uses a separate "ResolvedRefs" condition type
// (distinct from "Accepted") to report APPolicy/APLogConf reference resolution errors.
func waitForWAFPolicyResolvedRefs(nsname types.NamespacedName, condStatus metav1.ConditionStatus, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout)
	defer cancel()

	GinkgoWriter.Printf(
		"Waiting for WAFGatewayBindingPolicy %q to have condition ResolvedRefs/%s/%s\n",
		nsname, condStatus, reason,
	)

	return wait.PollUntilContextCancel(ctx, 500*time.Millisecond, true,
		func(ctx context.Context) (bool, error) {
			var pol ngfAPI.WAFGatewayBindingPolicy
			if err := resourceManager.Get(ctx, nsname, &pol); err != nil {
				return false, err
			}

			if len(pol.Status.Ancestors) == 0 {
				GinkgoWriter.Printf("WAFGatewayBindingPolicy %q has no ancestor status yet\n", nsname)
				return false, nil
			}

			for _, cond := range pol.Status.Ancestors[0].Conditions {
				if cond.Type != "ResolvedRefs" {
					continue
				}
				if string(cond.Status) == string(condStatus) && cond.Reason == reason {
					return true, nil
				}
				return false, fmt.Errorf(
					"ResolvedRefs condition is %s/%s, expected %s/%s",
					cond.Status, cond.Reason, condStatus, reason,
				)
			}

			GinkgoWriter.Printf("WAFGatewayBindingPolicy %q has no ResolvedRefs condition yet\n", nsname)
			return false, nil
		},
	)
}

// waitForWAFPolicyAncestorStatus polls until the WAFGatewayBindingPolicy ancestor status has the given condition.
func waitForWAFPolicyAncestorStatus(
	nsname types.NamespacedName,
	condStatus metav1.ConditionStatus,
	condReason v1.PolicyConditionReason,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout)
	defer cancel()

	GinkgoWriter.Printf(
		"Waiting for WAFGatewayBindingPolicy %q to have condition Accepted/%s/%s\n",
		nsname, condStatus, condReason,
	)

	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var pol ngfAPI.WAFGatewayBindingPolicy
			if err := resourceManager.Get(ctx, nsname, &pol); err != nil {
				return false, err
			}

			if len(pol.Status.Ancestors) == 0 {
				GinkgoWriter.Printf("WAFGatewayBindingPolicy %q has no ancestor status yet\n", nsname)
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

			GinkgoWriter.Printf("WAFGatewayBindingPolicy %q has no Accepted condition yet\n", nsname)
			return false, nil
		},
	)
}
