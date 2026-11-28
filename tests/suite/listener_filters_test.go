package main

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
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

const (
	listenerFilterAuthHeader1    = "X-Auth-1"
	listenerFilterAuthHeader2    = "X-Auth-2"
	listenerFilterListenerName   = "http"
	listenerFilterPublicListener = "http-public"
)

// expectListenerCondition waits for a Listener condition to reach the given status and reason.
func expectListenerCondition(
	nsName types.NamespacedName,
	listenerName string,
	condType string,
	condStatus metav1.ConditionStatus,
	condReason string,
) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var gw gatewayv1.Gateway
		g.Expect(resourceManager.Get(context.Background(), nsName, &gw)).To(Succeed())

		var listener *gatewayv1.ListenerStatus
		for i := range gw.Status.Listeners {
			if gw.Status.Listeners[i].Name == gatewayv1.SectionName(listenerName) {
				listener = &gw.Status.Listeners[i]
				break
			}
		}
		g.Expect(listener).ToNot(BeNil(), "listener %q not found in gateway status", listenerName)

		cond := meta.FindStatusCondition(listener.Conditions, condType)
		g.Expect(cond).ToNot(BeNil(), "condition %q not yet present", condType)
		g.Expect(cond.Status).To(Equal(condStatus), "condition %q status", condType)
		g.Expect(cond.Reason).To(Equal(condReason), "condition %q reason", condType)
	}).
		WithTimeout(timeoutConfig.GetStatusTimeout).
		WithPolling(500*time.Millisecond).
		Should(Succeed(), "gateway %s listener %s conditions", nsName, listenerName)
}

// expectListenerAcceptedAndProgrammed waits for a Listener to be accepted, programmed, and to have
// its refs resolved.
func expectListenerAcceptedAndProgrammed(nsName types.NamespacedName, listenerName string) {
	GinkgoHelper()

	for _, cond := range []struct {
		condType string
		reason   string
	}{
		{string(gatewayv1.ListenerConditionAccepted), string(gatewayv1.ListenerReasonAccepted)},
		{string(gatewayv1.ListenerConditionProgrammed), string(gatewayv1.ListenerReasonProgrammed)},
		{string(gatewayv1.ListenerConditionResolvedRefs), string(gatewayv1.ListenerReasonResolvedRefs)},
	} {
		expectListenerCondition(nsName, listenerName, cond.condType, metav1.ConditionTrue, cond.reason)
	}
}

var _ = Describe(
	"ListenerFilters",
	Ordered,
	Label("functional", "listener-filters"),
	func() {
		var (
			files = []string{
				"listener-filters/headers-echo.yaml",
				"listener-filters/external-auth.yaml",
				"listener-filters/cafe-routes.yaml",
			}

			gatewayFile = "listener-filters/gateway.yaml"

			namespace string
			gwNsName  types.NamespacedName

			port         int
			nginxPodName string

			// credentials are the values both ext-auth backends accept.
			credentials = map[string]string{
				listenerFilterAuthHeader1: "pass",
				listenerFilterAuthHeader2: "pass",
			}
		)

		// urlFor builds a port-forward URL for a host and path served by the Gateway.
		urlFor := func(hostPath string) string {
			return helpers.BuildPortFwdURL(hostPath, port)
		}

		// credsWith returns the accepted credentials plus extra request headers.
		credsWith := func(extra map[string]string) map[string]string {
			headers := maps.Clone(credentials)
			maps.Copy(headers, extra)
			return headers
		}

		// expectRequest polls a request until it produces the wanted outcome.
		expectRequest := func(url string, outcome framework.RequestOutcome, headers map[string]string) {
			GinkgoHelper()
			framework.VerifyRequest(
				timeoutConfig, url, address, outcome,
				framework.WithRequestHeaders(headers),
			)
		}

		// echoGet requests an echo path and returns the lower-cased headers the backend received.
		echoGet := func(g Gomega, url string, headers map[string]string) map[string]string {
			GinkgoHelper()

			resp, err := framework.Get(
				framework.Request{
					URL:     url,
					Address: address,
					Headers: headers,
					Timeout: timeoutConfig.RequestTimeout,
				},
			)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(resp.StatusCode).To(Equal(200))

			var parsed struct {
				Headers map[string]string `json:"headers"`
			}
			g.Expect(json.Unmarshal([]byte(resp.Body), &parsed)).To(Succeed())

			got := make(map[string]string, len(parsed.Headers))
			for k, v := range parsed.Headers {
				got[strings.ToLower(k)] = v
			}
			return got
		}

		BeforeAll(func() {
			namespace = fmt.Sprintf("listener-filters-%d", GinkgoParallelProcess())
			gwNsName = types.NamespacedName{Namespace: namespace, Name: "gateway"}

			ns := &core.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			}
			Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())

			backendObjects, err := framework.GenerateBackendAppObjects([]string{"coffee"})
			Expect(err).ToNot(HaveOccurred())
			for _, obj := range backendObjects {
				obj.SetNamespace(namespace)
			}
			Expect(resourceManager.Apply(backendObjects)).To(Succeed())

			Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())
			Expect(resourceManager.ApplyFromFiles([]string{gatewayFile}, namespace)).To(Succeed())
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())

			nginxPodNames, err := resourceManager.GetReadyNginxPodNames(
				namespace,
				timeoutConfig.GetStatusTimeout,
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(nginxPodNames).To(HaveLen(1))

			nginxPodName = nginxPodNames[0]

			setUpPortForward(nginxPodName, namespace)
			port = helpers.BuildPortFwdPort(80, portFwdPort)
		})

		AfterAll(func() {
			framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
			cleanUpPortForward()

			Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
		})

		It("accepts and programs both listeners", func() {
			expectListenerAcceptedAndProgrammed(gwNsName, listenerFilterListenerName)
			expectListenerAcceptedAndProgrammed(gwNsName, listenerFilterPublicListener)
		})

		It("serves the unfiltered listener without running the wildcard filters", func() {
			// Without credentials the wildcard filters would answer 403, so a 200 here proves the
			// chain does not run on this listener.
			expectRequest(urlFor("public.example.com/coffee"), framework.RequestSucceedsWith("coffee"), nil)
		})

		Context("nginx configuration", func() {
			var conf *framework.Payload

			BeforeAll(func() {
				conf = framework.MustGetNginxConfig(&resourceManager, nginxPodName, namespace)
			})

			It("gates the filtered listener's routes behind the filter chain", func() {
				// The route location must run the chain: a try_files hop from the route location to
				// the first chain location.
				expCfg := framework.ExpectedNginxField{
					Directive:             "try_files",
					Value:                 "/_ngf_pf_continue @ngf_pf_",
					ValueSubstringAllowed: true,
					File:                  "http.conf",
					Server:                "cafe.example.com",
					Location:              "/coffee",
				}
				Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())

				// The subrequest must target the internal auth location that proxies to the ext-auth-1
				// backend. The location path carries a server-specific ID, so capture it from the
				// auth_request directive instead of hardcoding it.
				var authPath string
				capture := framework.ExpectedNginxField{
					Directive:    "auth_request",
					CaptureValue: &authPath,
					File:         "http.conf",
					Server:       "cafe.example.com",
					Location:     "/coffee",
				}
				Expect(framework.ValidateNginxFieldExists(conf, capture)).To(Succeed())
				Expect(authPath).To(HavePrefix("/_ngf_pf_"))

				authLoc := framework.ExpectedNginxField{
					Directive:             "proxy_pass",
					Value:                 "ext-auth-1",
					ValueSubstringAllowed: true,
					File:                  "http.conf",
					Server:                "cafe.example.com",
					Location:              authPath,
				}
				Expect(framework.ValidateNginxFieldExists(conf, authLoc)).To(Succeed())
			})
		})

		It("gates every request on the filtered listener", func() {
			cafe := urlFor("cafe.example.com/coffee")
			onlyFirst := map[string]string{listenerFilterAuthHeader1: "pass"}
			onlySecond := map[string]string{listenerFilterAuthHeader2: "pass"}
			spoofed := credsWith(map[string]string{"X-User-Id": "mallory"})

			// With no credentials at all, whichever filter runs first decides the status: auth-1
			// rejects with 403 and auth-2 with 401. Carrying only the second filter's credential still
			// gets a 403, proving the chain runs in the exact configured order.
			expectRequest(cafe, framework.RequestReturnsStatus(403), nil)
			expectRequest(cafe, framework.RequestReturnsStatus(403), onlySecond)
			expectRequest(cafe, framework.RequestReturnsStatus(401), onlyFirst)

			// A spoofed X-User-Id must not reach the second filter, which only accepts the value the
			// first filter returned.
			expectRequest(cafe, framework.RequestSucceedsWith("coffee"), credentials)
			expectRequest(cafe, framework.RequestReturnsStatus(200), spoofed)

			// Requests that match no route are gated the same way: the chain answers 403, and once it
			// passes the request falls through to the 404.
			expectRequest(urlFor("cafe.example.com/none"), framework.RequestReturnsStatus(403), nil)
			expectRequest(urlFor("nobody.example.com/anything"), framework.RequestReturnsStatus(403), nil)
			expectRequest(urlFor("nobody.example.com/anything"), framework.RequestReturnsStatus(404), credentials)
		})

		It("merges the auth response headers into the proxied request", func() {
			// X-User-Id comes only from auth-1 and X-Shared from both filters, where the last non-empty
			// value (auth-2) wins. Client-supplied values must be replaced.
			headers := credsWith(map[string]string{"X-User-Id": "spoofed", "X-Shared": "spoofed"})

			Eventually(func(g Gomega) {
				got := echoGet(g, urlFor("cafe.example.com/headers"), headers)
				g.Expect(got["x-user-id"]).To(Equal("alice"))
				g.Expect(got["x-shared"]).To(Equal("from-auth2"))
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})

		It("keeps route header modifiers from duplicating the auth headers", func() {
			headers := credsWith(map[string]string{"X-User-Id": "spoofed", "X-Shared": "spoofed"})

			Eventually(func(g Gomega) {
				got := echoGet(g, urlFor("cafe.example.com/headers-modified"), headers)
				// The route Set wins outright: one value, not the auth value next to the route value.
				g.Expect(got["x-user-id"]).To(Equal("route-owned"))
				// The route Add appends to the value the filters returned, never to the client value.
				g.Expect(got["x-shared"]).To(Equal("from-auth2,extra"))
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})

		It("matches routes on the headers the filters set", func() {
			// The /vip route only matches requests whose X-User-Id header is alice, a value that only
			// auth-1 sets, so matching proves the filters ran before route selection. A client-supplied
			// value must not be used for the match.
			vip := urlFor("cafe.example.com/vip")
			spoofed := credsWith(map[string]string{"X-User-Id": "mallory"})

			Eventually(func(g Gomega) {
				for _, headers := range []map[string]string{credsWith(nil), spoofed} {
					g.Expect(echoGet(g, vip, headers)["x-user-id"]).To(Equal("alice"))
				}
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})

		Context("when a filter backendRef cannot be resolved", func() {
			setFilterBackend := func(name string) {
				var gw gatewayv1.Gateway
				Expect(resourceManager.Get(context.Background(), gwNsName, &gw)).To(Succeed())
				gw.Spec.Listeners[0].Filters.Requests[0].ExternalAuth.BackendRef.Name = gatewayv1.ObjectName(name)
				Expect(resourceManager.Apply([]client.Object{&gw})).To(Succeed())
			}

			BeforeAll(func() {
				setFilterBackend("ext-auth-missing")
			})

			AfterAll(func() {
				setFilterBackend("ext-auth-1")
			})

			It("keeps the listener accepted but fails closed for all requests", func() {
				expectListenerCondition(
					gwNsName, listenerFilterListenerName,
					string(gatewayv1.ListenerConditionAccepted),
					metav1.ConditionTrue,
					string(gatewayv1.ListenerReasonAccepted),
				)
				expectListenerCondition(
					gwNsName, listenerFilterListenerName,
					string(gatewayv1.ListenerConditionResolvedRefs),
					metav1.ConditionFalse,
					"InvalidListenerFilterRef",
				)

				expectRequest(urlFor("cafe.example.com/coffee"), framework.RequestReturnsStatus(500), credentials)
			})
		})

		Context("when a filter Service is deleted and recreated", func() {
			externalAuthYAML := []string{"listener-filters/external-auth.yaml"}

			AfterAll(func() {
				Expect(resourceManager.ApplyFromFiles(externalAuthYAML, namespace)).To(Succeed())
			})

			It("fails closed while it is gone and recovers once it is back", func() {
				Expect(
					resourceManager.Delete(
						context.Background(),
						&core.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "ext-auth-2"}},
						nil,
					),
				).To(Succeed())

				cafe := urlFor("cafe.example.com/coffee")
				expectRequest(cafe, framework.RequestReturnsStatus(500), credentials)

				Expect(resourceManager.ApplyFromFiles(externalAuthYAML, namespace)).To(Succeed())

				// Recovery needs the delete and add events of a Service that no route uses. They only
				// reach NGF while the listener keeps the Service referenced.
				expectRequest(cafe, framework.RequestSucceedsWith("coffee"), credentials)
			})
		})
	})
