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

// CrossNamespaceRouting tests the gateway-admin→app-developer trust boundary:
//   - Gateway (gw-ns) uses allowedRoutes.namespaces.from=Selector, restricted to app-ns.
//   - HTTPRoute (app-ns) has a backend Service in backend-ns, requiring a ReferenceGrant.
//   - A route from gw-ns (which lacks the selector label) is rejected with NotAllowedByListeners.
//   - A route with a mismatched hostname is rejected with NoMatchingListenerHostname.
var _ = Describe("CrossNamespaceRouting", Ordered, Label("functional", "cross-namespace"), func() {
	var (
		gwNamespace      string
		appNamespace     string
		backendNamespace string

		gatewayFiles = []string{"cross-namespace-routing/gateway.yaml"}
		appFiles     = []string{"cross-namespace-routing/apps.yaml"}
		routeFile    = []string{"cross-namespace-routing/route.yaml"}
		grantFile    = []string{"cross-namespace-routing/reference-grant.yaml"}
	)

	BeforeAll(func() {
		gwNamespace = fmt.Sprintf("xns-gw-%d", GinkgoParallelProcess())
		appNamespace = fmt.Sprintf("xns-app-%d", GinkgoParallelProcess())
		backendNamespace = fmt.Sprintf("xns-backend-%d", GinkgoParallelProcess())

		for _, ns := range []struct {
			labels map[string]string
			name   string
		}{
			// gwNamespace has no gateway-access label, so routes from it are rejected by the selector.
			{name: gwNamespace},
			// appNamespace is labeled so the Gateway's selector allows routes from here.
			{name: appNamespace, labels: map[string]string{"gateway-access": "allowed"}},
			{name: backendNamespace},
		} {
			nsObj := &core.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   ns.name,
					Labels: ns.labels,
				},
			}
			Expect(resourceManager.Apply([]client.Object{nsObj})).To(Succeed())
		}

		resourceManager.AddTextReplacement("GATEWAY_NAMESPACE", gwNamespace)
		resourceManager.AddTextReplacement("BACKEND_NAMESPACE", backendNamespace)
		resourceManager.AddTextReplacement("ROUTE_NAMESPACE", appNamespace)

		Expect(resourceManager.ApplyFromFiles(gatewayFiles, gwNamespace)).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(appFiles, backendNamespace)).To(Succeed())

		// backend-ns holds only the backend pods; WaitForAppsToBeReady also waits for a Programmed
		// Gateway, which only exists in gw-ns.
		podsCtx, cancel := context.WithTimeout(context.Background(), timeoutConfig.CreateTimeout)
		defer cancel()
		Expect(resourceManager.WaitForPodsToBeReady(podsCtx, backendNamespace)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(gwNamespace)).To(Succeed())

		nginxPodNames, err := resourceManager.GetReadyNginxPodNames(gwNamespace, timeoutConfig.GetStatusTimeout)
		Expect(err).ToNot(HaveOccurred())
		Expect(nginxPodNames).To(HaveLen(1))
		setUpPortForward(nginxPodNames[0], gwNamespace)
	})

	AfterAll(func() {
		framework.AddNginxLogsAndEventsToReport(resourceManager, gwNamespace)
		cleanUpPortForward()

		resourceManager.RemoveTextReplacement("GATEWAY_NAMESPACE")
		resourceManager.RemoveTextReplacement("BACKEND_NAMESPACE")
		resourceManager.RemoveTextReplacement("ROUTE_NAMESPACE")

		Expect(resourceManager.DeleteFromFiles(appFiles, backendNamespace)).To(Succeed())
		Expect(resourceManager.DeleteFromFiles(gatewayFiles, gwNamespace)).To(Succeed())
		for _, ns := range []string{gwNamespace, appNamespace, backendNamespace} {
			Expect(resourceManager.DeleteNamespace(ns)).To(Succeed())
		}
	})

	// --- ReferenceGrant boundary ---

	Context("when an HTTPRoute in app-ns has a backend Service in backend-ns", Ordered, func() {
		// grantApplied tracks whether the second It applied the grant, so AfterAll
		// only attempts to delete it when it was actually created.
		var grantApplied bool

		BeforeAll(func() {
			grantApplied = false
			Expect(resourceManager.ApplyFromFiles(routeFile, appNamespace)).To(Succeed())
		})

		AfterAll(func() {
			// Delete in reverse-application order: grant first (if applied), then route.
			if grantApplied {
				Expect(resourceManager.DeleteFromFiles(grantFile, backendNamespace)).To(Succeed())
			}
			Expect(resourceManager.DeleteFromFiles(routeFile, appNamespace)).To(Succeed())
		})

		It("reports ResolvedRefs=False/RefNotPermitted without a ReferenceGrant", func() {
			expectHTTPRouteCondition(
				types.NamespacedName{Name: "coffee", Namespace: appNamespace},
				gwNamespace,
				string(gatewayv1.RouteConditionResolvedRefs),
				metav1.ConditionFalse,
				string(gatewayv1.RouteReasonRefNotPermitted),
			)
		})

		It("resolves and traffic reaches the backend once a ReferenceGrant is added", func() {
			Expect(resourceManager.ApplyFromFiles(grantFile, backendNamespace)).To(Succeed())
			grantApplied = true

			expectHTTPRouteCondition(
				types.NamespacedName{Name: "coffee", Namespace: appNamespace},
				gwNamespace,
				string(gatewayv1.RouteConditionResolvedRefs),
				metav1.ConditionTrue,
				string(gatewayv1.RouteReasonResolvedRefs),
			)

			// Traffic assertion: verify requests actually reach the backend after the grant resolves.
			port := helpers.BuildPortFwdPort(80, portFwdPort)
			coffeeURL := helpers.BuildPortFwdURL("cafe.example.com/coffee", port)
			Eventually(func() error {
				return framework.ExpectRequestToSucceed(timeoutConfig.RequestTimeout, coffeeURL, address, "URI: /coffee")
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500*time.Millisecond).
				Should(Succeed(), "traffic should reach the backend once the ReferenceGrant is in place")
		})
	})

	// --- allowedRoutes namespace boundary ---

	// gwNamespace has no gateway-access label, so routes applied there are rejected by the selector.
	// This avoids spinning up a fourth namespace solely for this negative case.
	Context("when an HTTPRoute is applied from a namespace not permitted by the Gateway selector", func() {
		badRouteFiles := []string{"cross-namespace-routing/bad-namespace-route.yaml"}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(badRouteFiles, gwNamespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(badRouteFiles, gwNamespace)).To(Succeed())
		})

		It("reports NotAllowedByListeners on the parent status", func() {
			expectHTTPRouteCondition(
				types.NamespacedName{Name: "bad-ns-coffee", Namespace: gwNamespace},
				gwNamespace,
				string(gatewayv1.RouteConditionAccepted),
				metav1.ConditionFalse,
				string(gatewayv1.RouteReasonNotAllowedByListeners),
			)
		})
	})

	// --- Hostname mismatch ---

	Context("when an HTTPRoute specifies a hostname that does not match any listener", func() {
		wrongHostnameFiles := []string{"cross-namespace-routing/wrong-hostname-route.yaml"}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(wrongHostnameFiles, appNamespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(wrongHostnameFiles, appNamespace)).To(Succeed())
		})

		It("reports NoMatchingListenerHostname on the parent status", func() {
			expectHTTPRouteCondition(
				types.NamespacedName{Name: "wrong-hostname", Namespace: appNamespace},
				gwNamespace,
				string(gatewayv1.RouteConditionAccepted),
				metav1.ConditionFalse,
				string(gatewayv1.RouteReasonNoMatchingListenerHostname),
			)
		})
	})
})

// expectHTTPRouteCondition polls until the named HTTPRoute's parent status (for the Gateway in
// gwNamespace) has a condition matching the given type, status, and reason.
func expectHTTPRouteCondition(
	nsName types.NamespacedName,
	gwNamespace string,
	condType string,
	condStatus metav1.ConditionStatus,
	condReason string,
) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var route gatewayv1.HTTPRoute
		g.Expect(resourceManager.Get(context.Background(), nsName, &route)).To(Succeed())

		var parent *gatewayv1.RouteParentStatus
		for i := range route.Status.Parents {
			p := &route.Status.Parents[i]
			if p.ControllerName == gatewayv1.GatewayController(framework.NgfControllerName) &&
				(p.ParentRef.Namespace == nil || string(*p.ParentRef.Namespace) == gwNamespace) {
				parent = p
				break
			}
		}
		g.Expect(parent).ToNot(BeNil(),
			"no parent status for controller %s in namespace %s", framework.NgfControllerName, gwNamespace,
		)

		cond := meta.FindStatusCondition(parent.Conditions, condType)
		g.Expect(cond).ToNot(BeNil(), "condition %q not yet present", condType)
		g.Expect(cond.Status).To(Equal(condStatus), "condition %q status", condType)
		g.Expect(cond.Reason).To(Equal(condReason), "condition %q reason", condType)
	}).
		WithTimeout(timeoutConfig.GetStatusTimeout).
		WithPolling(500*time.Millisecond).
		Should(Succeed(), "HTTPRoute %s did not reach expected condition %s=%s/%s", nsName, condType, condStatus, condReason)
}
