package main

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var _ = Describe("Secret label selector", Ordered, Label("secret-label-selector"), func() {
	const (
		namespace      = "secret-label-selector"
		watchLabelKey  = "gateway.nginx.org/watch"
		watchLabelVal  = "true"
		watchSelector  = watchLabelKey + "=" + watchLabelVal
		tlsObjName     = "cafe-tls-secret"
		httpsBaseURL   = "https://cafe.example.com"
		upgradeTimeout = 5 * time.Minute
	)

	var (
		gwNsName    types.NamespacedName
		httpsURL    string
		ngfUpgraded bool
	)

	upgradeNGFSelector := func(value string) {
		// --reuse-values keeps the existing release values and only flips the selector.
		// An empty value unsets it, so the flag is dropped from the NGF Deployment.
		args := []string{
			"upgrade", releaseName, localChartPath,
			"--namespace", ngfNamespace,
			"--reuse-values", "--wait",
			"--set", fmt.Sprintf("nginxGateway.watchSecretLabelSelector=%s", value),
		}

		ctx, cancel := context.WithTimeout(context.Background(), upgradeTimeout)
		defer cancel()

		out, err := exec.CommandContext(ctx, "helm", args...).CombinedOutput()
		Expect(err).ToNot(HaveOccurred(), string(out))
	}

	listenerConditions := func() []metav1.Condition {
		var gw gatewayv1.Gateway
		Expect(resourceManager.Get(context.Background(), gwNsName, &gw)).To(Succeed())

		Expect(gw.Status.Listeners).ToNot(BeEmpty())

		return gw.Status.Listeners[0].Conditions
	}

	waitForResolvedRefs := func(resolved bool) {
		Eventually(func() bool {
			conds := listenerConditions()
			if resolved {
				return meta.IsStatusConditionTrue(conds, string(gatewayv1.ListenerConditionResolvedRefs))
			}
			return meta.IsStatusConditionFalse(conds, string(gatewayv1.ListenerConditionResolvedRefs))
		}).
			WithTimeout(timeoutConfig.GetStatusTimeout).
			WithPolling(time.Second).
			Should(BeTrue(), "listener ResolvedRefs condition never reached the expected state")
	}

	setTLSWatchLabel := func(labeled bool) {
		var secret core.Secret
		nsName := types.NamespacedName{Name: tlsObjName, Namespace: namespace}
		Expect(resourceManager.Get(context.Background(), nsName, &secret)).To(Succeed())

		if labeled {
			secret.Labels = map[string]string{watchLabelKey: watchLabelVal}
		} else {
			secret.Labels = nil
		}
		Expect(resourceManager.Update(context.Background(), &secret, nil)).To(Succeed())
	}

	BeforeAll(func() {
		if !*secretLabelSelectorTests {
			Skip("re-upgrades the shared NGF release with a cluster-wide --watch-secret-label-selector")
		}
		ngfUpgraded = true
		gwNsName = types.NamespacedName{Name: "sls-gateway", Namespace: namespace}

		upgradeNGFSelector(watchSelector)

		ns := &core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())

		// The TLS Secret is deliberately created without the watch label: NGF must not see it.
		cert, err := framework.GenerateSelfSignedCACert("cafe.example.com")
		Expect(err).ToNot(HaveOccurred())

		tlsSecret := &core.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: tlsObjName, Namespace: namespace},
			Type:       core.SecretTypeTLS,
			Data: map[string][]byte{
				core.TLSCertKey:       cert.CertPEM,
				core.TLSPrivateKeyKey: cert.KeyPEM,
			},
		}
		Expect(resourceManager.Apply([]client.Object{tlsSecret})).To(Succeed())

		Expect(resourceManager.ApplyFromFiles(
			[]string{"secret-label-selector/cafe.yaml", "secret-label-selector/gateway.yaml"},
			namespace,
		)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())

		nginxPodNames, err := resourceManager.GetReadyNginxPodNames(namespace, timeoutConfig.GetStatusTimeout)
		Expect(err).ToNot(HaveOccurred())
		Expect(nginxPodNames).To(HaveLen(1))

		setUpPortForward(nginxPodNames[0], namespace)
		httpsURL = fmt.Sprintf("%s:%d/", httpsBaseURL, portFwdHTTPSPort)
		if portFwdHTTPSPort == 0 {
			httpsURL = httpsBaseURL + "/"
		}
	})

	AfterAll(func() {
		if !ngfUpgraded {
			return
		}
		framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
		cleanUpPortForward()
		Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
		upgradeNGFSelector("")
	})

	It("does not watch an unlabelled TLS Secret", func() {
		waitForResolvedRefs(false)
		Eventually(func() string {
			conds := listenerConditions()
			cond := meta.FindStatusCondition(conds, string(gatewayv1.ListenerConditionResolvedRefs))
			if cond == nil {
				return ""
			}
			return cond.Reason
		}).
			WithTimeout(timeoutConfig.GetStatusTimeout).
			Should(Equal(string(gatewayv1.ListenerReasonInvalidCertificateRef)))
	})

	It("watches the Secret and serves TLS once the label is added", func() {
		setTLSWatchLabel(true)
		waitForResolvedRefs(true)

		Eventually(func() int {
			resp, err := framework.Get(
				framework.Request{URL: httpsURL, Address: address, Timeout: timeoutConfig.RequestTimeout},
			)
			if err != nil {
				return 0
			}
			return resp.StatusCode
		}).
			WithTimeout(timeoutConfig.RequestTimeout).
			WithPolling(time.Second).
			Should(Equal(http.StatusOK))
	})

	It("stops serving the certificate once the label is removed", func() {
		setTLSWatchLabel(false)
		waitForResolvedRefs(false)
	})
})
